package ingress

import (
	"net"
	"net/http"

	"github.com/gravitational/teleport/lib/observability/metrics"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
)

// Constants for each ingress service.
const (
	Web         = "web"
	SSH         = "ssh"
	Kube        = "kube"
	Tunnel      = "tunnel"
	MySQL       = "mysql"
	Postgres    = "postgres"
	SQLServer   = "sqlserver"
	DatabaseTLS = "database_tls"
)

// Constants for each ingress path.
const (
	pathDirect  = "direct"
	pathALPN    = "alpn"
	pathUnknown = "unknown"
)

var commonLabels = []string{"ingress_path", "ingress_service"}

// acceptedConnections measures connections accepeted by each listener type and ingress path.
// This allows us to differentiate between connectoins going through alpn routing or directly
// to the listener.
var acceptedConnections = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "teleport",
	Name:      "accepted_connections",
}, commonLabels)

// activeConnections measures the current number of active connections.
var activeConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "teleport",
	Name:      "active_connections",
}, commonLabels)

// authenticatedConnectionsAccepted measures the number of connections that successfully authenticated.
var authenticatedConnectionsAccepted = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "teleport",
	Name:      "authenticated_accepted_connections",
}, commonLabels)

// authenticatedConnectionsActive measure the current number of active connectoins that
// successfully authenticated.
var authenticatedConnectionsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "teleport",
	Name:      "authenticated_active_connections",
}, commonLabels)

// HTTPConnStateReporter returns a http connection event handler function to track
// connection metrics for an http server.
func HTTPConnStateReporter(service string, r *Reporter) func(net.Conn, http.ConnState) {
	return func(conn net.Conn, state http.ConnState) {
		if r == nil {
			return
		}

		switch state {
		case http.StateNew:
			r.ConnectionAccepeted(service, conn)
			r.ConnectionAuthenticated(service, conn)
		case http.StateClosed, http.StateHijacked:
			r.ConnectionClosed(service, conn)
			r.AuthenticatedConnectionClosed(service, conn)
		}
	}
}

// NewReporter constructs a new ingress reporter.
func NewReporter(alpnAddr string) (*Reporter, error) {
	err := metrics.RegisterPrometheusCollectors(
		acceptedConnections,
		activeConnections,
		authenticatedConnectionsAccepted,
		authenticatedConnectionsActive,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var unspecifiedIP bool
	host, port, err := net.SplitHostPort(alpnAddr)
	if err == nil {
		ip := net.ParseIP(host)
		if ip != nil {
			unspecifiedIP = ip.IsUnspecified()
		}
	}

	return &Reporter{
		alpnAddr:      alpnAddr,
		alpnPort:      port,
		unspecifiedIP: unspecifiedIP,
	}, nil
}

// Reporter provides a simple interface for tracking connection ingress metrics.
type Reporter struct {
	// alpnAddr is the host string expected for a connection ingressing through ALPN routing.
	alpnAddr string
	// alpnPort is the port string expected for a connection ingressing through ALPN routing.
	alpnPort string
	// unspecifiedIP is true if the alpnAddr is an unspecified addr (0.0.0.0, [::]).
	unspecifiedIP bool
}

// ConnectionAccepeted reports a new connection, ConnectionClosed must be called when the connection closes.
func (r *Reporter) ConnectionAccepeted(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	acceptedConnections.WithLabelValues(path, service).Inc()
	activeConnections.WithLabelValues(path, service).Inc()
}

// ConnectionClosed reports a closed connection. This should only be called after ConnectionAccepeted.
func (r *Reporter) ConnectionClosed(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	activeConnections.WithLabelValues(path, service).Dec()
}

// ConnectionAuthenticated reports a new authenticated connection, AuthenticatedConnectionClosed must
// be called when the connection is closed.
func (r *Reporter) ConnectionAuthenticated(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	authenticatedConnectionsAccepted.WithLabelValues(path, service).Inc()
	authenticatedConnectionsActive.WithLabelValues(path, service).Inc()
}

// AuthenticatedConnectionClosed reports a closed authenticated connection, this should only be called
// after ConnectionAuthenticated.
func (r *Reporter) AuthenticatedConnectionClosed(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	authenticatedConnectionsActive.WithLabelValues(path, service).Dec()
}

// getIngressPath determines the ingress path of a given connection.
func (r *Reporter) getIngressPath(conn net.Conn) string {
	// An empty address indicates alpn routing is disabled.
	if r.alpnAddr == "" {
		return pathDirect
	}

	// If the IP is unspecified we only check if the ports match.
	if r.unspecifiedIP {
		_, port, err := net.SplitHostPort(conn.LocalAddr().String())
		if err != nil {
			return pathUnknown
		}
		if port == r.alpnPort {
			return pathALPN
		}
		return pathDirect
	}
	if r.alpnAddr == conn.LocalAddr().String() {
		return pathALPN
	}
	return pathDirect
}
