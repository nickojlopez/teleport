/*
Copyright 2020 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package benchmark package provides tools to run progressive or independent benchmarks against teleport services.
package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// minValue is the min millisecond recorded for histogram
	minValue = 1
	// maxValue is the max millisecond recorded for histogram
	maxValue = 60000
	// significantFigures is the precision of the values
	significantFigures = 3
	// pauseTimeBetweenBenchmarks is the time to pause between each benchmark
	pauseTimeBetweenBenchmarks = time.Second * 5
)

// Service is a the Teleport service to benchmark.
type Service string

const (
	// SSHService is the SSH service
	SSHService Service = "ssh"
	// KubernetesService is the Kubernetes service
	KubernetesService Service = "kube"
)

// Config specifies benchmark requests to run
type Config struct {
	// Rate is requests per second origination rate
	Rate int
	// Command is a command to run
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
	// MinimumWindow is the min duration
	MinimumWindow time.Duration
	// MinimumMeasurments is the min amount of requests
	MinimumMeasurements int
	// Service is the service to benchmark
	Service Service
	// PodExecBenchmark is the pod exec benchmark config.
	// When not nil, it will be used to run the benchmark using the pod exec
	// method. If nil, the benchmark will list pods in all namespaces.
	PodExecBenchmark *PodExecBenchmark
	// PodNamespace is the namespace of the pod to run the benchmark against.
	PodNamespace string
}

type PodExecBenchmark struct {
	// ContainerName is the name of the container to run the benchmark against.
	ContainerName string
	// PodName is the name of the pod to run the benchmark against.
	PodName string
}

// CheckAndSetDefaults checks and sets default values for the benchmark config.
func (c *Config) CheckAndSetDefaults() error {
	switch c.Service {
	case SSHService:
	case KubernetesService:
	default:
		return trace.BadParameter("unsupported service %q", c.Service)
	}
	return nil
}

// Result is a result of the benchmark
type Result struct {
	// RequestsOriginated is amount of requests originated
	RequestsOriginated int
	// RequestsFailed is amount of requests failed
	RequestsFailed int
	// Histogram holds the response duration values
	Histogram *hdrhistogram.Histogram
	// LastError contains last recorded error
	LastError error
	// Duration it takes for the whole benchmark to run
	Duration time.Duration
}

// Run is used to run the benchmarks, it is given a generator, command to run,
// a host, host login, and proxy. If host login or proxy is an empty string, it will
// use the default login
func Run(ctx context.Context, lg *Linear, cmd, host, login, proxy string) ([]Result, error) {
	c := strings.Split(cmd, " ")
	lg.config = &Config{Command: c}
	if err := validateConfig(lg); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := makeTeleportClient(host, login, proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		exitSignals := make(chan os.Signal, 1)
		signal.Notify(exitSignals, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(exitSignals)
		sig := <-exitSignals
		logrus.Debugf("signal: %v", sig)
		cancel()
	}()
	var results []Result
	sleep := false
	for {
		if sleep {
			select {
			case <-time.After(pauseTimeBetweenBenchmarks):
			case <-ctx.Done():
				return results, trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
			}
		}
		benchmarkC := lg.GetBenchmark()
		if benchmarkC == nil {
			break
		}
		result, err := benchmarkC.Benchmark(ctx, tc)
		if err != nil {
			return results, trace.Wrap(err)
		}
		results = append(results, result)
		fmt.Printf("current generation requests: %v, duration: %v\n", result.RequestsOriginated, result.Duration)
		sleep = true
	}
	return results, nil
}

// ExportLatencyProfile exports the latency profile and returns the path as a string if no errors
func ExportLatencyProfile(path string, h *hdrhistogram.Histogram, ticks int32, valueScale float64) (string, error) {
	timeStamp := time.Now().Format("2006-01-02_15:04:05")
	suffix := fmt.Sprintf("latency_profile_%s.txt", timeStamp)
	if path != "." {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return "", trace.Wrap(err)
		}
	}
	fullPath := filepath.Join(path, suffix)
	fo, err := os.Create(fullPath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if _, err := h.PercentilesPrint(fo, ticks, valueScale); err != nil {
		if err := fo.Close(); err != nil {
			logrus.WithError(err).Warningf("failed to close file")
		}
		return "", trace.Wrap(err)
	}

	if err := fo.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	return fo.Name(), nil
}

// workloadFunc is a function that executes a single benchmark call.
type workloadFunc func(benchMeasure) error

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns benchmark result when completed.
// This is a blocking function that can be canceled via context argument.
func (c *Config) Benchmark(ctx context.Context, tc *client.TeleportClient) (Result, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return Result{}, trace.Wrap(err)
	}

	var (
		workload workloadFunc
		err      error
	)
	switch c.Service {
	case SSHService:
		workload = executeSSHBenchmark
	case KubernetesService:
		workload, err = c.kubernetesBenchmarkCreator(ctx, tc)
		if err != nil {
			return Result{}, trace.Wrap(err)
		}
	default:
		return Result{}, trace.BadParameter("unsupported service %q", c.Service)
	}

	tc.Stdout = io.Discard
	tc.Stderr = io.Discard
	tc.Stdin = &bytes.Buffer{}
	var delay time.Duration
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	requestsC := make(chan benchMeasure)
	resultC := make(chan benchMeasure)

	go func() {
		interval := time.Duration(1 / float64(c.Rate) * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				// ticker makes its first tick after the given duration, not immediately
				// this sets the send measure ResponseStart time accurately
				delay = delay + interval
				t := start.Add(delay)
				measure := benchMeasure{
					ResponseStart: t,
					command:       c.Command,
					client:        tc,
					interactive:   c.Interactive,
				}
				go work(ctx, measure, resultC, workload)
			case <-ctx.Done():
				close(requestsC)
				return
			}
		}
	}()

	var result Result
	result.Histogram = hdrhistogram.New(minValue, maxValue, significantFigures)
	statusTicker := time.NewTicker(1 * time.Second)
	timeElapsed := false
	start := time.Now()
	for {
		if c.MinimumWindow <= time.Since(start) {
			timeElapsed = true
		}
		select {
		case measure := <-resultC:
			result.Histogram.RecordValue(int64(measure.End.Sub(measure.ResponseStart) / time.Millisecond))
			result.RequestsOriginated++
			if timeElapsed && result.RequestsOriginated >= c.MinimumMeasurements {
				cancel()
			}
			if measure.Error != nil {
				result.RequestsFailed++
				result.LastError = measure.Error
			}
		case <-ctx.Done():
			result.Duration = time.Since(start)
			return result, nil
		case <-statusTicker.C:
			logrus.Infof("working... current observation count: %d", result.RequestsOriginated)
		}

	}
}

type benchMeasure struct {
	ResponseStart time.Time
	End           time.Time
	Error         error
	client        *client.TeleportClient
	command       []string
	interactive   bool
}

func work(ctx context.Context, m benchMeasure, send chan<- benchMeasure, workload workloadFunc) {
	m.Error = workload(m)
	m.End = time.Now()
	select {
	case send <- m:
	case <-ctx.Done():
		return
	}
}

func executeSSHBenchmark(m benchMeasure) error {
	if !m.interactive {
		// do not use parent context that will cancel in flight requests
		// because we give test some time to gracefully wrap up
		// the in-flight connections to avoid extra errors
		return m.client.SSH(context.TODO(), m.command, false)
	}
	config := m.client.Config
	client, err := client.NewClient(&config)
	if err != nil {
		return err
	}
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	client.Stdin = reader
	out := &utils.SyncBuffer{}
	client.Stdout = out
	client.Stderr = out
	err = m.client.SSH(context.TODO(), nil, false)
	if err != nil {
		return err
	}
	writer.Write([]byte(strings.Join(m.command, " ") + "\r\nexit\r\n"))
	return nil
}

func (c *Config) kubernetesBenchmarkCreator(ctx context.Context, tc *client.TeleportClient) (workloadFunc, error) {
	tlsClientConfig, err := getKubeTLSClientConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	restConfig := &rest.Config{
		Host:            tc.KubeClusterAddr(),
		TLSClientConfig: tlsClientConfig,
		APIPath:         "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: scheme.Codecs,
		},
	}

	// if the user has specified a pod exec benchmark, use that instead of the
	// default kubernetes client.
	if c.PodExecBenchmark != nil {
		exec, err := c.kubeExecOnPod(ctx, tc, restConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return func(bm benchMeasure) error {
			err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdin:  tc.Stdin,
				Stdout: tc.Stdout,
				Stderr: tc.Stderr,
				Tty:    c.Interactive,
			})
			return trace.Wrap(err)
		}, nil
	}

	// create a kubernetes client that will be used to execute the benchmark.
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(bm benchMeasure) error {
		// List all pods in all namespaces.
		_, err := kubeClient.CoreV1().Pods(c.PodNamespace).List(ctx, metav1.ListOptions{})
		return trace.Wrap(err)
	}, nil
}

func (c *Config) kubeExecOnPod(ctx context.Context, tc *client.TeleportClient, restConfig *rest.Config) (remotecommand.Executor, error) {
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := restClient.Post().
		Resource("pods").
		Name(c.PodExecBenchmark.PodName).
		Namespace(c.PodNamespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: c.PodExecBenchmark.ContainerName,
		Command:   c.Command,
		Stdin:     tc.Stdin != nil,
		Stdout:    tc.Stdout != nil,
		Stderr:    tc.Stderr != nil,
		TTY:       c.Interactive,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	return exec, trace.Wrap(err)
}

// getKubeTLSClientConfig returns a TLS client config for the kubernetes cluster
// that the client wants to connected to.
func getKubeTLSClientConfig(ctx context.Context, tc *client.TeleportClient) (rest.TLSClientConfig, error) {
	var k *client.Key
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		k, err = tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		}, nil /*applyOpts*/)
		return err
	})
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	certPem := k.KubeTLSCerts[tc.KubernetesCluster]

	rsaKeyPEM, err := k.PrivateKey.RSAPrivateKeyPEM()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	credentials, err := tc.LocalAgent().GetCoreKey()
	if err != nil {
		return rest.TLSClientConfig{}, trace.Wrap(err)
	}

	var clusterCAs [][]byte
	if tc.LoadAllCAs {
		clusterCAs = credentials.TLSCAs()
	} else {
		clusterCAs, err = credentials.RootClusterCAs()
		if err != nil {
			return rest.TLSClientConfig{}, trace.Wrap(err)
		}
	}
	if len(clusterCAs) == 0 {
		return rest.TLSClientConfig{}, trace.BadParameter("no trusted CAs found")
	}

	tlsServerName := ""
	if tc.TLSRoutingEnabled {
		k8host, _ := tc.KubeProxyHostPort()
		tlsServerName = client.GetKubeTLSServerName(k8host)
	}

	return rest.TLSClientConfig{
		CAData:     bytes.Join(clusterCAs, []byte("\n")),
		CertData:   certPem,
		KeyData:    rsaKeyPEM,
		ServerName: tlsServerName,
	}, nil
}

// makeTeleportClient creates an instance of a teleport client
func makeTeleportClient(host, login, proxy string) (*client.TeleportClient, error) {
	c := client.Config{
		Host:   host,
		Tracer: tracing.NoopProvider().Tracer("test"),
	}

	if login != "" {
		c.HostLogin = login
		c.Username = login
	}
	if proxy != "" {
		c.SSHProxyAddr = proxy
	}

	profileStore := client.NewFSProfileStore("")
	if err := c.LoadProfile(profileStore, proxy); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := client.NewClient(&c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}
