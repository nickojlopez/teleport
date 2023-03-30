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
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

// BenchmarkSuite is an interface that defines a benchmark suite.
type BenchmarkSuite interface {
	IsBenchmark()
	workload(context.Context, *client.TeleportClient) (workloadFunc, error)
}

// SSHBenchmark is a benchmark suite that runs a single SSH command
// against a Teleport node for a given duration and rate.
type SSHBenchmark struct {
	// Command is a command to run
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
}

// IsBenchmark is a no-op function that is used to ensure that SSHBenchmark
// implements the BenchmarkSuite interface.
func (s SSHBenchmark) IsBenchmark() {}

// workload is a helper function that returns a workloadFunc for the given
// benchmark suite.
func (s SSHBenchmark) workload(ctx context.Context, tc *client.TeleportClient) (workloadFunc, error) {
	return func(ctx context.Context, m benchMeasure) error {
		if !s.Interactive {
			// do not use parent context that will cancel in flight requests
			// because we give test some time to gracefully wrap up
			// the in-flight connections to avoid extra errors
			return tc.SSH(ctx, s.Command, false)
		}
		config := tc.Config
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
		err = tc.SSH(ctx, nil, false)
		if err != nil {
			return err
		}
		writer.Write([]byte(strings.Join(s.Command, " ") + "\r\nexit\r\n"))
		return nil
	}, nil
}

// KubeListBenchmark is a benchmark suite that runs successive kubectl get pods
// against a Teleport Kubernetes proxy for a given duration and rate.
type KubeListBenchmark struct {
	// Namespace is the Kubernetes namespace to run the command against.
	// If empty, it will include pods from all namespaces.
	Namespace string
}

// IsBenchmark is a no-op function that is used to ensure that SSHBenchmark
// implements the BenchmarkSuite interface.
func (k KubeListBenchmark) IsBenchmark() {}

// workload is a helper function that returns a workloadFunc for the given
// benchmark suite.
func (k KubeListBenchmark) workload(ctx context.Context, tc *client.TeleportClient) (workloadFunc, error) {
	restCfg, err := newKubernetesRestConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context, m benchMeasure) error {
		// List all pods in all namespaces.
		_, err := clientset.CoreV1().Pods(k.Namespace).List(ctx, metav1.ListOptions{})
		return trace.Wrap(err)
	}, nil
}

// KubeListBenchmark is a benchmark suite that runs successive kubectl exec
// against a Teleport Kubernetes proxy for a given duration and rate.
type KubeExecBenchmark struct {
	// Namespace is the Kubernetes namespace to run the command against.
	Namespace string
	// PodName is the name of the pod to run the command against.
	PodName string
	// ContainerName is the name of the container to run the command against.
	ContainerName string
	// Command is the command to run.
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
}

// IsBenchmark is a no-op function that is used to ensure that SSHBenchmark
// implements the BenchmarkSuite interface.
func (k KubeExecBenchmark) IsBenchmark() {}

// workload is a helper function that returns a workloadFunc for the given
// benchmark suite.
func (k KubeExecBenchmark) workload(ctx context.Context, tc *client.TeleportClient) (workloadFunc, error) {
	restCfg, err := newKubernetesRestConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	exec, err := k.kubeExecOnPod(ctx, tc, restCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context, m benchMeasure) error {
		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  tc.Stdin,
			Stdout: tc.Stdout,
			Stderr: tc.Stderr,
			Tty:    k.Interactive,
		})
		return trace.Wrap(err)
	}, nil
}

func (k KubeExecBenchmark) kubeExecOnPod(ctx context.Context, tc *client.TeleportClient, restConfig *rest.Config) (remotecommand.Executor, error) {
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := restClient.Post().
		Resource("pods").
		Name(k.PodName).
		Namespace(k.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: k.ContainerName,
		Command:   k.Command,
		Stdin:     tc.Stdin != nil,
		Stdout:    tc.Stdout != nil,
		Stderr:    tc.Stderr != nil,
		TTY:       k.Interactive,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	return exec, trace.Wrap(err)
}

func newKubernetesRestConfig(ctx context.Context, tc *client.TeleportClient) (*rest.Config, error) {
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
	return restConfig, nil
}
