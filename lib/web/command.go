/*

 Copyright 2023 Gravitational, Inc.

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

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"
)

func (h *Handler) executeCommand(
	w http.ResponseWriter,
	r *http.Request,
	_ httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnel.RemoteSite,
) (any, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req *CommandRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sessionCtx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := createIdentityContext(req.Login, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), identity, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var sessionData session.Session

	clusterName := site.GetName()

	//if req.SessionID.IsZero() {
	// An existing session ID was not provided, so we need to create a new one.
	sessionData, err = h.generateCommandSession(ctx, clt, req, clusterName, sessionCtx.cfg.User)
	if err != nil {
		h.log.WithError(err).Debug("Unable to generate new ssh session.")
		return nil, trace.Wrap(err)
	}
	//} else {
	//	sessionData, displayLogin, err = h.fetchExistingSession(ctx, clt, req, clusterName)
	//	if err != nil {
	//		return nil, trace.Wrap(err)
	//	}
	//}

	// If the participantMode is not specified, and the user is the one who created the session,
	// they should be in Peer mode. If not, default to Observer mode.
	//if req.ParticipantMode == "" {
	//	if sessionData.Owner == sessionCtx.cfg.User {
	//		req.ParticipantMode = types.SessionPeerMode
	//	} else {
	//		req.ParticipantMode = types.SessionObserverMode
	//	}
	//}

	h.log.Debugf("New terminal request for server=%s, labels=%v, login=%s, sid=%s, websid=%s.",
		req.NodeID, req.Labels, req.Login, sessionData.ID, sessionCtx.GetSessionID())

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.log.WithError(err).Debug("Unable to get auth access point.")
		return nil, trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
		return nil, trace.Wrap(err)
	}

	terminalConfig := CommandHandlerConfig{
		SessionCtx:         sessionCtx,
		AuthProvider:       clt,
		SessionData:        sessionData,
		KeepAliveInterval:  netConfig.GetKeepAliveInterval(),
		ProxyHostPort:      h.ProxyHostPort(),
		InteractiveCommand: strings.Split(req.Command, " "),
		Router:             h.cfg.Router,
		TracerProvider:     h.cfg.TracerProvider,
		PROXYSigner:        h.cfg.PROXYSigner,
	}

	handler, err := newCommandHandler(ctx, terminalConfig)
	if err != nil {
		h.log.WithError(err).Error("Unable to create terminal.")
		return nil, trace.Wrap(err)
	}

	h.userConns.Add(1)
	defer h.userConns.Add(-1)

	// start the websocket session with a web-based terminal:
	h.log.Infof("Getting terminal to %#v.", req)
	httplib.MakeTracingHandler(handler, teleport.ComponentProxy).ServeHTTP(w, r)

	return nil, nil
}

func newCommandHandler(ctx context.Context, cfg CommandHandlerConfig) (*commandHandler, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, span := cfg.tracer.Start(ctx, "NewTerminal")
	defer span.End()

	return &commandHandler{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentWebsocket,
			"session_id":    cfg.SessionData.ID.String(),
		}),
		ctx:                cfg.SessionCtx,
		authProvider:       cfg.AuthProvider,
		sessionData:        cfg.SessionData,
		keepAliveInterval:  cfg.KeepAliveInterval,
		proxyHostPort:      cfg.ProxyHostPort,
		interactiveCommand: cfg.InteractiveCommand,
		router:             cfg.Router,
		proxySigner:        cfg.PROXYSigner,
		tracer:             cfg.tracer,
	}, nil
}

type CommandHandlerConfig struct {
	// sctx is the context for the users web session.
	SessionCtx *SessionContext
	// authProvider is used to fetch nodes and sessions from the backend.
	AuthProvider AuthProvider
	// sessionData is the data to send to the client on the initial session creation.
	SessionData session.Session
	// keepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
	// proxyHostPort is the address of the server to connect to.
	ProxyHostPort string
	// interactiveCommand is a command to execute.
	InteractiveCommand []string
	// Router determines how connections to nodes are created
	Router *proxy.Router
	// TracerProvider is used to create the tracer
	TracerProvider oteltrace.TracerProvider
	// ProxySigner is used to sign PROXY header and securely propagate client IP information
	PROXYSigner multiplexer.PROXYHeaderSigner
	// tracer is used to create spans
	tracer oteltrace.Tracer
}

func (t *CommandHandlerConfig) CheckAndSetDefaults() error {
	// Make sure whatever session is requested is a valid session id.
	_, err := session.ParseID(t.SessionData.ID.String())
	if err != nil {
		return trace.BadParameter("sid: invalid session id")
	}

	if t.SessionData.Login == "" {
		return trace.BadParameter("login: missing login")
	}

	if t.SessionData.ServerID == "" {
		return trace.BadParameter("server: missing server")
	}

	if t.AuthProvider == nil {
		return trace.BadParameter("AuthProvider must be provided")
	}

	if t.SessionCtx == nil {
		return trace.BadParameter("SessionCtx must be provided")
	}

	if t.Router == nil {
		return trace.BadParameter("Router must be provided")
	}

	if t.TracerProvider == nil {
		t.TracerProvider = tracing.DefaultProvider()
	}

	t.tracer = t.TracerProvider.Tracer("webterminal")

	return nil
}

type commandHandler struct {
	// log holds the structured logger.
	log *logrus.Entry
	// ctx is a web session context for the currently logged in user.
	ctx *SessionContext
	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider
	// proxyHostPort is the address of the server to connect to.
	proxyHostPort string

	// keepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	keepAliveInterval time.Duration

	// The server data for the active session.
	sessionData session.Session

	// router is used to dial the host
	router *proxy.Router

	stream *WsStream

	// tracer creates spans
	tracer oteltrace.Tracer

	// sshSession holds the "shell" SSH channel to the node.
	sshSession *tracessh.Session

	// ProxySigner is used to sign PROXY header and securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner

	// interactiveCommand is a command to execute.
	interactiveCommand []string
}

func (t *commandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This allows closing of the websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errMsg := "Error upgrading to websocket"
		t.log.WithError(err).Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	err = ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
	if err != nil {
		t.log.WithError(err).Error("Error setting websocket readline")
		return
	}

	// If the displayLogin is set then use it instead of the login name used in
	// the SSH connection. This is specifically for the use case when joining
	// a session to avoid displaying "-teleport-internal-join" as the username.
	//if t.displayLogin != "" {
	//	t.sessionData.Login = t.displayLogin
	//}

	sendError := func(errMsg string, err error, ws *websocket.Conn) {
		envelope := &Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketError,
			Payload: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		}

		envelopeBytes, _ := proto.Marshal(envelope)
		ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	}

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: t.sessionData})
	if err != nil {
		sendError("unable to marshal session response", err, ws)
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		sendError("unable to marshal session data event for web client", err, ws)
		return
	}

	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		sendError("unable to write message to socket", err, ws)
		return
	}

	t.handler(ws, r)
}

func (t *commandHandler) handler(ws *websocket.Conn, r *http.Request) {
	defer ws.Close()

	// Create a terminal stream that wraps/unwraps the envelope used to
	// communicate over the websocket.
	//resizeC := make(chan *session.TerminalParams, 1)
	stream, err := NewWStream(ws)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a terminal stream for session")
		t.writeError(err)
		return
	}
	t.stream = stream

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	//tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	//t.terminalContext, t.terminalCancel = context.WithCancel(tctx)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(r.Context(), ws)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.writeError(err)
		return
	}

	t.log.Debug("Creating websocket stream")

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
		return nil
	})

	// Start sending ping frames through websocket to client.
	go t.startPingLoop(ws)

	go t.streamEvents(tc)
	// Pump raw terminal in/out and audit events into the websocket.
	t.streamOutput(r.Context(), ws, tc)

	// process window resizing
	//go t.handleWindowResize(resizeC)

	// Block until the terminal session is complete.
	//<-t.terminalContext.Done()
	t.log.Debug("Closing websocket stream")
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *commandHandler) streamOutput(ctx context.Context, ws *websocket.Conn, tc *client.TeleportClient) {
	//ctx, span := t.tracer.Start(t.terminalContext, "terminal/streamTerminal")
	//defer span.End()
	//
	//defer t.terminalCancel()

	accessChecker, err := t.ctx.GetUserAccessChecker()
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to get access checker")
		t.writeError(err)
		return
	}

	getAgent := func() (teleagent.Agent, error) {
		return teleagent.NopCloser(tc.LocalAgent()), nil
	}
	signerCreator := func() (ssh.Signer, error) {
		cert, err := t.ctx.GetSSHCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		validBefore := time.Unix(int64(cert.ValidBefore), 0)
		ttl := time.Until(validBefore)
		return agentless.CreateAuthSigner(ctx, t.ctx.GetUser(), tc.SiteName, ttl, t.router)
	}
	conn, _, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signerCreator)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host.")

		if errors.Is(err, trace.NotFound(teleport.NodeIsAmbiguous)) {
			const message = "error: ambiguous host could match multiple nodes\n\nHint: try addressing the node by unique id (ex: user@node-id)\n"
			t.writeError(trace.NotFound(message))
			return
		}

		t.writeError(err)
		return
	}

	defer func() {
		if conn == nil {
			return
		}

		if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
			t.log.WithError(err).Warn("Failed to close connection to host")
		}
	}()

	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            tc.AuthMethods,
		HostKeyCallback: tc.HostKeyCallback,
	}
	t.log.Debugf("Using SSH config: %+v", sshConfig)
	t.log.Debugf("Using session data: %+v", t.sessionData)

	nc, connectErr := client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)), tc, modules.GetModules().IsBoringBinary())
	switch {
	case connectErr != nil && !trace.IsAccessDenied(connectErr): // catastrophic error, return it
		t.log.WithError(connectErr).Warn("Unable to stream terminal - failed to create node client")
		t.writeError(connectErr)
		return
	case connectErr != nil && trace.IsAccessDenied(connectErr): // see if per session mfa would allow access
		panic(err)
		//	mfaRequiredResp, err := t.authProvider.IsMFARequired(ctx, &authproto.IsMFARequiredRequest{
		//		Target: &authproto.IsMFARequiredRequest_Node{
		//			Node: &authproto.NodeLogin{
		//				Node:  t.sessionData.ServerID,
		//				Login: tc.HostLogin,
		//			},
		//		},
		//	})
		//	if err != nil {
		//		t.log.WithError(err).Warn("Unable to stream terminal - failed to determine if per session mfa is required")
		//		// write the original connect error
		//		t.writeError(connectErr)
		//		return
		//	}
		//
		//	if !mfaRequiredResp.Required {
		//		t.log.WithError(connectErr).Warn("Unable to stream terminal - user does not have access to host")
		//		// write the original connect error
		//		t.writeError(connectErr)
		//		return
		//	}
		//
		//	// perform mfa ceremony and retrieve new certs
		//	if err := t.issueSessionMFACerts(ctx, tc); err != nil {
		//		t.log.WithError(err).Warn("Unable to stream terminal - failed to perform mfa ceremony")
		//		t.writeError(err)
		//		return
		//	}
		//
		//	// update auth methods
		//	sshConfig.Auth = tc.AuthMethods
		//
		//	// connect to the node again with the new certs
		//	conn, _, err = t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signerCreator)
		//	if err != nil {
		//		t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host")
		//		t.writeError(err)
		//		return
		//	}
		//
		//	nc, err = client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)), tc, modules.GetModules().IsBoringBinary())
		//	if err != nil {
		//		t.log.WithError(err).Warn("Unable to stream terminal - failed to create node client")
		//		t.writeError(err)
		//		return
		//	}
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunCommand(ctx, t.interactiveCommand, nil); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure running interactive shell")
		t.writeError(err)
		return
	}

	if err := t.stream.Close(); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// startPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when terminal session is closed.
func (t *commandHandler) startPingLoop(ws *websocket.Conn) {
	t.log.Debugf("Starting websocket ping loop with interval %v.", t.keepAliveInterval)
	tickerCh := time.NewTicker(t.keepAliveInterval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			// A short deadline is used here to detect a broken connection quickly.
			// If this is just a temporary issue, we will retry shortly anyway.
			deadline := time.Now().Add(time.Second)
			if err := ws.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				t.log.WithError(err).Error("Unable to send ping frame to web client")
				t.Close()
				return
			}
			//case <-t.terminalContext.Done():
			//	t.log.Debug("Terminating websocket ping loop.")
			//	return
		}
	}
}

func (t *commandHandler) Close() error {
	return nil
}

type outEnvelope struct {
	NodeID  string `json:"node_id"`
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

type payloadWriter struct {
	nodeID     string
	outputName string
	stream     io.Writer
}

func (p *payloadWriter) Write(b []byte) (n int, err error) {
	out := &outEnvelope{
		NodeID:  p.nodeID,
		Type:    p.outputName,
		Payload: b,
	}
	data, err := json.Marshal(out)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return p.stream.Write(data)
}

func newPayloadWriter(nodeID, outputName string, stream io.Writer) *payloadWriter {
	return &payloadWriter{
		nodeID:     nodeID,
		outputName: outputName,
		stream:     stream,
	}
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *commandHandler) makeClient(ctx context.Context, ws *websocket.Conn) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(ctx, "commandHandler/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig.HostLogin = t.sessionData.Login
	clientConfig.ForwardAgent = client.ForwardAgentLocal
	clientConfig.Namespace = apidefaults.Namespace
	clientConfig.Stdout = newPayloadWriter(t.sessionData.ServerID, "stdout", t.stream)
	clientConfig.Stderr = newPayloadWriter(t.sessionData.ServerID, "stderr", t.stream)
	clientConfig.Stdin = t.stream
	clientConfig.SiteName = t.sessionData.ClusterName
	if err := clientConfig.ParseProxyHost(t.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.sessionData.ServerHostname
	clientConfig.HostPort = t.sessionData.ServerHostPort
	clientConfig.SessionID = t.sessionData.ID.String()
	clientConfig.ClientAddr = ws.RemoteAddr().String()
	clientConfig.Tracer = t.tracer

	//if len(t.interactiveCommand) > 0 {
	//	clientConfig.InteractiveCommand = true
	//}

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	// Save the *ssh.Session after the shell has been created. The session is
	// used to update all other parties window size to that of the web client and
	// to allow future window changes.
	tc.OnShellCreated = func(s *tracessh.Session, c *tracessh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.sshSession = s
		//t.windowChange(ctx, &t.term)

		return false, nil
	}

	return tc, nil
}

func (t *commandHandler) streamEvents(tc *client.TeleportClient) {
	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-tc.EventsChannel():
			logger := t.log.WithField("event", event.GetType())

			data, err := json.Marshal(event)
			if err != nil {
				logger.WithError(err).Error("Unable to marshal audit event")
				continue
			}

			logger.Debug("Sending audit event to web client.")

			_ = data
			//if err := t.stream.writeAuditEvent(data); err != nil {
			//	if err != nil {
			//		if errors.Is(err, websocket.ErrCloseSent) {
			//			logger.WithError(err).Debug("Websocket was closed, no longer streaming events")
			//			return
			//		}
			//		logger.WithError(err).Error("Unable to send audit event to web client")
			//		continue
			//	}
			//}

			// Once the terminal stream is over (and the close envelope has been sent),
			// close stop streaming envelopes.
			//case <-t.terminalContext.Done():
			//	return
		}
	}
}

// writeError displays an error in the terminal window.
func (t *commandHandler) writeError(err error) {
	if writeErr := t.stream.writeError(err); writeErr != nil {
		t.log.WithError(writeErr).Warnf("Unable to send error to terminal: %v", err)
	}
}
