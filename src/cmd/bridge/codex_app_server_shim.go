package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	libcodex "github.com/takezoh/agent-grid/platform/lib/codex"
)

// Daemon-dial retry envelope. The daemon container endpoint is created
// lazily by the runtime (bootstrap.go / cleanup.go call
// startContainerEndpointIfNeeded), and its startup can arrive after this
// shim is spawned as the codex subsystem — a 41s gap has been observed in
// production. Retrying until the endpoint appears keeps the shim from
// hard-exiting into a 15s daemon-side DialUDS timeout that surfaces to
// the browser as `504 daemon_timeout` on POST /api/sessions.
//
// If retry ultimately fails the shim continues with a nil daemon client;
// only frame-messaging tool calls (agent_frames.*) then error at call
// time. Primary codex app-server proxying is unaffected.
var (
	daemonDialTotalTimeout  = 60 * time.Second
	daemonDialRetryInterval = 200 * time.Millisecond
)

type shimConfig struct {
	listenSock      string
	upstreamSock    string
	serverBin       string
	sessionID       string
	serverArgs      []string
	sandboxExternal bool
}

func runCodexAppServerShim(args []string) error {
	cfg, err := parseCodexShimArgs(args)
	if err != nil {
		return err
	}
	daemonSock := strings.TrimSpace(os.Getenv("AG_SOCKET"))
	if daemonSock == "" {
		return errors.New("AG_SOCKET is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	removeShimSockets(cfg.upstreamSock)
	cmd := exec.CommandContext(ctx, cfg.serverBin, libcodex.AppServerListenArgs(cfg.serverBin, cfg.upstreamSock, cfg.serverArgs, cfg.sandboxExternal)[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start upstream app-server: %w", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
		removeShimSockets(cfg.upstreamSock)
	}()

	// Dial the daemon container endpoint in the background so the downstream
	// listener (srv.serve below) can come up immediately. The daemon's
	// DialUDS to our listen socket has a 15s cap; blocking on a synchronous
	// daemon dial here — as prior to this fix — is what caused POST
	// /api/sessions to reach the HTTP 10s deadline and return 504.
	var daemonPtr atomic.Pointer[proto.Client]
	daemonReady := make(chan struct{})
	go func() {
		defer close(daemonReady)
		c := dialDaemonWithRetry(ctx, daemonSock, daemonDialTotalTimeout, daemonDialRetryInterval)
		if c == nil {
			slog.Warn("codex-app-server-shim: daemon dial exhausted retry; frame-messaging tools will error at call time",
				"sock", daemonSock)
			return
		}
		daemonPtr.Store(c)
	}()
	defer func() {
		cancel()
		<-daemonReady
		if c := daemonPtr.Swap(nil); c != nil {
			_ = c.Close()
		}
	}()

	srv := &codexShimServer{
		cfg:       cfg,
		daemonPtr: &daemonPtr,
		httpSrv:   &http.Server{},
	}
	srv.httpSrv.Handler = http.HandlerFunc(srv.serveHTTP)
	return srv.serve(ctx)
}

// dialDaemonWithRetry attempts proto.Dial(sock) at fixed intervals until
// totalTimeout elapses or ctx is done. Returns nil on failure so the shim
// can continue serving the codex app-server proxy without a live daemon
// connection.
func dialDaemonWithRetry(ctx context.Context, sock string, totalTimeout, interval time.Duration) *proto.Client {
	deadline := time.After(totalTimeout)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if c, err := proto.Dial(sock); err == nil {
			return c
		}
		select {
		case <-ctx.Done():
			return nil
		case <-deadline:
			return nil
		case <-tick.C:
		}
	}
}

func removeShimSockets(paths ...string) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		_ = os.Remove(path)
	}
}

func parseCodexShimArgs(args []string) (shimConfig, error) {
	var cfg shimConfig
	fs := flag.NewFlagSet("codex-app-server-shim", flag.ContinueOnError)
	listen := fs.String("listen", "", "shim listen socket (unix://...)")
	upstream := fs.String("upstream", "", "upstream app-server socket (unix://...)")
	serverBin := fs.String("server-bin", "", "real codex app-server binary")
	sessionID := fs.String("session-id", "", "owning agent-grid session id")
	fs.Func("server-arg", "repeatable upstream app-server arg", func(v string) error {
		cfg.serverArgs = append(cfg.serverArgs, v)
		return nil
	})
	fs.BoolVar(&cfg.sandboxExternal, "sandbox-external", false, "append sandbox_mode danger-full-access to upstream app-server")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.listenSock = strings.TrimPrefix(strings.TrimSpace(*listen), "unix://")
	cfg.upstreamSock = strings.TrimPrefix(strings.TrimSpace(*upstream), "unix://")
	cfg.serverBin = strings.TrimSpace(*serverBin)
	cfg.sessionID = strings.TrimSpace(*sessionID)
	if cfg.listenSock == "" || cfg.upstreamSock == "" || cfg.serverBin == "" || cfg.sessionID == "" {
		return cfg, errors.New("usage: codex-app-server-shim --listen unix://<sock> --upstream unix://<sock> --server-bin <bin> --session-id <id> [--server-arg <arg>...]")
	}
	return cfg, nil
}

type codexShimServer struct {
	cfg       shimConfig
	daemonPtr *atomic.Pointer[proto.Client]
	httpSrv   *http.Server
}

func (s *codexShimServer) serve(ctx context.Context) error {
	removeShimSockets(s.cfg.listenSock)
	ln, err := net.Listen("unix", s.cfg.listenSock)
	if err != nil {
		return err
	}
	defer func() {
		_ = ln.Close()
		removeShimSockets(s.cfg.listenSock)
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutCtx)
	}()
	return s.httpSrv.Serve(ln)
}

func (s *codexShimServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = ws.CloseNow() }()

	upstreamTransport, err := codexclient.DialUDS(s.cfg.upstreamSock, 15*time.Second)
	if err != nil {
		_ = ws.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer upstreamTransport.Close()

	downstream := codexclient.NewConn(&shimWSTransport{c: ws}, 30*time.Second)
	upstream := codexclient.NewConn(upstreamTransport, 30*time.Second)
	// Snapshot the daemon pointer at connection time. Retry may still be in
	// flight (nil result here), or may complete later; the session honors a
	// nil daemon by erroring individual agent_frames.* tool calls instead of
	// tearing down the whole proxy.
	session := &codexShimSession{
		downstream: downstream,
		upstream:   upstream,
		daemon:     s.daemonPtr.Load(),
		sessionID:  s.cfg.sessionID,
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- downstream.Run(ctx, shimDownstreamHandler{session: session}) }()
	go func() { errCh <- upstream.Run(ctx, shimUpstreamHandler{session: session}) }()
	<-errCh
	cancel()
}

type codexShimSession struct {
	downstream *codexclient.Conn
	upstream   *codexclient.Conn
	daemon     *proto.Client
	sessionID  string
}

type shimDownstreamHandler struct{ session *codexShimSession }
type shimUpstreamHandler struct{ session *codexShimSession }

func (h shimDownstreamHandler) OnNotification(method string, params json.RawMessage) {
	_ = h.session.upstream.Notify(method, params)
}

func (h shimDownstreamHandler) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	if shouldInjectFrameMessagingTools(method) {
		params = injectFrameMessagingDynamicTools(params)
	}
	result, err := h.session.upstream.Request(method, params)
	if err != nil {
		// ReplyRPCError bytes-preserves an upstream JSON-RPC error object
		// end-to-end (code / message / data verbatim) and falls back to
		// InternalErrorCode (-32603) for locally-synthesized errors (a
		// timeout, transport failure). This is the error-object twin of
		// bytes-preserving id forwarding — without it, an upstream
		// -32602 "Invalid params" surfaces to codex-cli as an outbound
		// object missing "code" and the whole reply fails to deserialize
		// as JSONRPCMessage, taking the TUI bootstrap down.
		_ = h.session.downstream.ReplyRPCError(id, err)
		return
	}
	_ = h.session.downstream.Reply(id, result)
}

func shouldInjectFrameMessagingTools(method string) bool {
	return method == codexschema.MethodThreadStart || method == codexschema.MethodThreadResume
}

type frameMessagingToolDef struct {
	canonical string
	dynamic   string
}

var frameMessagingToolDefs = []frameMessagingToolDef{
	{canonical: "agent_frames.list", dynamic: "agent_frames_list"},
	{canonical: "agent_frames.read", dynamic: "agent_frames_read"},
	{canonical: "agent_frames.send_message", dynamic: "agent_frames_send_message"},
	{canonical: "agent_frames.reply", dynamic: "agent_frames_reply"},
}

func canonicalFrameMessagingToolName(name string) (string, bool) {
	for _, def := range frameMessagingToolDefs {
		if name == def.canonical || name == def.dynamic {
			return def.canonical, true
		}
	}
	return "", false
}

func (h shimUpstreamHandler) OnNotification(method string, params json.RawMessage) {
	_ = h.session.downstream.Notify(method, params)
}

func (h shimUpstreamHandler) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	if method == codexschema.MethodItemToolCall {
		handled, reply, err := h.session.handleToolCall(params)
		if err != nil {
			// Locally-generated (frame-messaging tool) error — no
			// upstream error object to preserve; ReplyRPCError's
			// non-*RPCError branch fills in InternalErrorCode.
			_ = h.session.upstream.ReplyRPCError(id, err)
			return
		}
		if handled {
			_ = h.session.upstream.Reply(id, reply)
			return
		}
	}
	result, err := h.session.downstream.Request(method, params)
	if err != nil {
		// downstream is the codex CLI; if it replied with a JSON-RPC
		// error (e.g. approval decline), bytes-forward that object to
		// the real app-server upstream so its structured code/data
		// survive the proxy hop.
		_ = h.session.upstream.ReplyRPCError(id, err)
		return
	}
	_ = h.session.upstream.Reply(id, result)
}

type shimToolCallParams struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	ThreadID  string          `json:"threadId"`
}

type shimToolCallReply struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

type shimFrameSendArgs struct {
	TargetFrameID string `json:"targetFrameId"`
	Topic         string `json:"topic,omitempty"`
	Body          string `json:"body"`
	Priority      string `json:"priority,omitempty"`
}

type shimFrameReadArgs struct {
	PeerFrameID string `json:"peerFrameId,omitempty"`
}

type shimFrameReplyArgs struct {
	MessageID   string `json:"messageId"`
	Body        string `json:"body,omitempty"`
	FinalAnswer string `json:"finalAnswer,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
}

func (s *codexShimSession) handleToolCall(raw json.RawMessage) (bool, shimToolCallReply, error) {
	var params shimToolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return true, shimToolCallReply{}, err
	}
	canonicalTool, isFrameTool := canonicalFrameMessagingToolName(params.Tool)
	if isFrameTool && s.daemon == nil {
		// Daemon container endpoint wasn't reachable when this WebSocket
		// session started (see runCodexAppServerShim). Surface a clean tool
		// error to Codex instead of nil-dereferencing on Send.
		return true, shimToolCallReply{}, errors.New("agent-grid daemon unavailable: frame-messaging tools are disabled for this session")
	}
	var (
		resp proto.Response
		err  error
		ok   bool
	)
	switch canonicalTool {
	case "agent_frames.list":
		ok = isFrameTool
		resp, err = s.daemon.Send(context.Background(), proto.CmdFrameListByThread{SessionID: s.sessionID, ThreadID: params.ThreadID})
	case "agent_frames.read":
		ok = isFrameTool
		var args shimFrameReadArgs
		if err = decodeStrictJSON(params.Arguments, &args); err == nil {
			resp, err = s.daemon.Send(context.Background(), proto.CmdFrameReadByThread{
				SessionID:   s.sessionID,
				ThreadID:    params.ThreadID,
				PeerFrameID: args.PeerFrameID,
			})
		}
	case "agent_frames.send_message":
		ok = isFrameTool
		var args shimFrameSendArgs
		if err = decodeStrictJSON(params.Arguments, &args); err == nil {
			resp, err = s.daemon.Send(context.Background(), proto.CmdFrameSendByThread{
				SessionID:     s.sessionID,
				ThreadID:      params.ThreadID,
				TargetFrameID: args.TargetFrameID,
				Topic:         args.Topic,
				Body:          args.Body,
				Priority:      args.Priority,
			})
		}
	case "agent_frames.reply":
		ok = isFrameTool
		var args shimFrameReplyArgs
		if err = decodeStrictJSON(params.Arguments, &args); err == nil {
			resp, err = s.daemon.Send(context.Background(), proto.CmdFrameReplyByThread{
				SessionID:   s.sessionID,
				ThreadID:    params.ThreadID,
				MessageID:   args.MessageID,
				Body:        args.Body,
				FinalAnswer: args.FinalAnswer,
				Resolution:  args.Resolution,
				Confidence:  args.Confidence,
			})
		}
	}
	if !ok {
		return false, shimToolCallReply{}, nil
	}
	if err != nil {
		return true, shimToolCallReply{}, err
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return true, shimToolCallReply{}, err
	}
	return true, shimToolCallReply{Success: true, Output: string(body)}, nil
}

func injectFrameMessagingDynamicTools(raw json.RawMessage) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	var tools []any
	if existing, ok := payload["dynamicTools"].([]any); ok {
		tools = append(tools, existing...)
	}
	tools = append(tools, frameMessagingDynamicToolSpecs()...)
	payload["dynamicTools"] = tools
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return out
}

func frameMessagingDynamicToolSpecs() []any {
	specs := make([]any, 0, len(frameMessagingToolDefs))
	for _, def := range frameMessagingToolDefs {
		spec := map[string]any{
			"name":        def.dynamic,
			"description": "List same-session claude/codex frames visible to this Codex thread.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           map[string]any{},
			},
		}
		switch def.canonical {
		case "agent_frames.read":
			spec["description"] = "Read durable inbox messages for this Codex thread."
			spec["inputSchema"] = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"peerFrameId": map[string]any{"type": "string"},
				},
			}
		case "agent_frames.send_message":
			spec["description"] = "Store a durable inbox message for a same-session claude/codex frame."
			spec["inputSchema"] = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"targetFrameId", "body"},
				"properties": map[string]any{
					"targetFrameId": map[string]any{"type": "string"},
					"topic":         map[string]any{"type": "string"},
					"body":          map[string]any{"type": "string"},
					"priority":      map[string]any{"type": "string"},
				},
			}
		case "agent_frames.reply":
			spec["description"] = "Reply to a durable frame message as this Codex thread."
			spec["inputSchema"] = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"messageId"},
				"properties": map[string]any{
					"messageId":   map[string]any{"type": "string"},
					"body":        map[string]any{"type": "string"},
					"finalAnswer": map[string]any{"type": "string"},
					"resolution":  map[string]any{"type": "string"},
					"confidence":  map[string]any{"type": "string"},
				},
			}
		}
		specs = append(specs, spec)
	}
	return specs
}

func decodeStrictJSON(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

type shimWSTransport struct {
	c  *websocket.Conn
	mu sync.Mutex
}

func (t *shimWSTransport) ReadMessage(ctx context.Context) ([]byte, error) {
	_, data, err := t.c.Read(ctx)
	return data, err
}

func (t *shimWSTransport) WriteMessage(ctx context.Context, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.c.Write(ctx, websocket.MessageText, data)
}

func (t *shimWSTransport) Close() error { return t.c.CloseNow() }
