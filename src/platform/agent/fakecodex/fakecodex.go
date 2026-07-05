// Package fakecodex publishes a reusable in-process fake of the Codex
// app-server (stdio JSON-RPC v2). It replies to initialize / thread/start /
// thread/resume, and — on turn/start — emits the thread/started ▸ turn/started ▸
// (optional item events) ▸ thread/tokenUsage/updated ▸
// turn/completed sequence a real server produces.
//
// Two invariants keep the fake honest:
//
//  1. codex_appserver_e2e_test.go (build tag e2e) drives a real `codex
//     app-server` and asserts that the method set the fake advertises is a
//     subset of the method set the real binary emits. When it fails, this
//     fake must be updated to match.
//
//  2. This package depends only on other platform/agent/* packages. It never
//     imports client/* or orchestrator/*.
//
// The fake is safe to import from tests in any layer.
package fakecodex

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

// DefaultThreadID is used when Config.ThreadID is empty.
const DefaultThreadID = "thread-abc"

// DefaultTurnID is used when Config.TurnID is empty.
const DefaultTurnID = "turn-xyz"

// TokenUsageSpec controls what thread/tokenUsage/updated carries.
// Zero-valued spec ⇒ Last defaults to 10/5, and Total accumulates from zero.
type TokenUsageSpec struct {
	Last TokenBreakdown
}

// TokenBreakdown mirrors the wire fields of tokenUsage.last / total.
type TokenBreakdown struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
}

// TurnRequest is the decoded turn/start notification handed to a TurnHandler.
type TurnRequest struct {
	ThreadID string
	CWD      string
	Message  string
	Raw      json.RawMessage
}

// TurnEmitter is the subset of codexclient.Server methods a handler may call
// during a turn. Wrapped so a handler cannot bypass the fake's bookkeeping
// (usage totals, turn id assignment, etc.).
type TurnEmitter interface {
	AgentDelta(delta string) error
	ItemStarted(item map[string]any) error
	ItemCompleted(item map[string]any) error
	ToolCallRequest(tool string, arguments any, callID string) (json.RawMessage, error)
	ThreadSettingsUpdated(settings map[string]any) error
}

// TurnHandler drives one turn. Return err != nil to fail the turn: the fake
// emits `error` with err.Error() and skips turn/completed. Return err == nil
// to complete the turn: the fake emits thread/tokenUsage/updated followed by
// turn/completed with the returned text.
//
// ctx is derived from Serve/Attach's context, so a handler that blocks on
// external I/O should honour ctx.Done() to let stop() return cleanly.
type TurnHandler func(ctx context.Context, req TurnRequest, e TurnEmitter) (text string, err error)

// Config configures a fake Codex app-server.
type Config struct {
	// ThreadID and TurnID replace the defaults. TurnID is reused for every
	// turn; set OnTurn to derive per-turn ids when needed.
	ThreadID string
	TurnID   string

	// FailInit rejects initialize with a JSON-RPC error.
	FailInit bool

	// HangTurn keeps the turn open (no turn/completed and no error) until the
	// server context is cancelled. Useful for turn_timeout tests.
	HangTurn bool

	// Handler runs inside every turn. When nil, DefaultTurnHandler runs
	// (immediate turn/completed with text "done").
	Handler TurnHandler

	// TokenUsage controls the shape of thread/tokenUsage/updated. Zero value
	// yields last=10/5, cumulative total.
	TokenUsage TokenUsageSpec
}

// Server is one fake app-server. Construct it with New, then Serve or Attach.
//
// A Server is single-shot: do not call Serve (or Attach) more than once on
// the same instance. The second call overwrites srv / serveCtx while the
// first Serve's turn-handler goroutines may still be running, and the
// shared handlerWG would join a goroutine that belongs to the wrong
// lifecycle. Create a new Server for each Serve/Attach cycle.
type Server struct {
	cfg Config

	mu               sync.Mutex
	srv              *codexclient.Server
	lastThreadParams json.RawMessage
	lastTurnParams   json.RawMessage
	lastResumeParams json.RawMessage
	lastCWD          string
	lastMessage      string
	totalUsage       TokenBreakdown
	toolReplies      []json.RawMessage

	// serveCtx is the ctx passed to the current Serve call. Turn-handler
	// goroutines derive their context from this so a Serve/Attach shutdown
	// signals cooperating handlers to exit. Nil until Serve is entered.
	serveCtx context.Context
	// handlerWG tracks in-flight turn-handler goroutines so Attach's stop
	// can join them cleanly. Serve does not join automatically — that would
	// deadlock a handler blocked on a Conn.Request whose reply cannot arrive.
	handlerWG sync.WaitGroup
}

// New constructs a Server with the given config. It does not open a transport;
// call Serve or Attach with a transport (typically a StdioTransport over
// io.Pipe ends) to start responding.
func New(cfg Config) *Server {
	if cfg.ThreadID == "" {
		cfg.ThreadID = DefaultThreadID
	}
	if cfg.TurnID == "" {
		cfg.TurnID = DefaultTurnID
	}
	if cfg.TokenUsage.Last.InputTokens == 0 && cfg.TokenUsage.Last.OutputTokens == 0 {
		cfg.TokenUsage.Last = TokenBreakdown{InputTokens: 10, OutputTokens: 5}
	}
	return &Server{cfg: cfg}
}

// Serve runs the fake server against transport until ctx is cancelled or the
// remote end closes. Any codexclient.Conn error is returned. Turn-handler
// goroutines spawned by OnNotification inherit their context from ctx.
func (s *Server) Serve(ctx context.Context, transport codexclient.Transport) error {
	conn := codexclient.NewConn(transport, 0)
	s.mu.Lock()
	s.srv = codexclient.NewServer(conn)
	s.serveCtx = ctx
	s.mu.Unlock()
	return conn.Run(ctx, s)
}

// Attach wires the server to Reader/Writer ends (typically io.Pipe pairs)
// and returns a stop function. It runs the loop in a goroutine so callers may
// interact with the client end concurrently.
//
// stop is idempotent. Both r and w must be Close-able because
// codexclient's stdio transport uses a bufio.Scanner that ignores context —
// stop closes r to force ReadMessage to unblock and Serve to exit.
func (s *Server) Attach(ctx context.Context, r io.ReadCloser, w io.WriteCloser) (stop func()) {
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Serve(subCtx, codexclient.StdioTransport(r, w))
		_ = w.Close()
	}()
	return func() {
		cancel()
		_ = r.Close()
		_ = w.Close()
		<-done
		// Bounded join. A handler blocked in Conn.Request cannot observe
		// ctx cancellation (Request only selects on the reply channel and
		// a fixed readTimeout timer), so waiting unbounded can hang a test
		// for the full readTimeout. Give handlers a short window to notice
		// the closed transport and exit; anything longer than that is a
		// bug in the handler and leaking a goroutine for a few hundred
		// milliseconds is better than hanging teardown.
		joined := make(chan struct{})
		go func() {
			s.handlerWG.Wait()
			close(joined)
		}()
		select {
		case <-joined:
		case <-time.After(handlerJoinBudget):
		}
	}
}

// handlerJoinBudget bounds how long Attach.stop waits for outstanding
// turn-handler goroutines. See stop's inline comment for the rationale.
const handlerJoinBudget = 500 * time.Millisecond

// ---- codexclient.Handler ----

// OnServerRequest replies to initialize / thread/start / thread/resume.
func (s *Server) OnServerRequest(id int64, method string, params json.RawMessage) {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return
	}
	switch method {
	case codexschema.MethodInitialize:
		if s.cfg.FailInit {
			_ = srv.Conn().ReplyError(id, "fakecodex: init rejected")
			return
		}
		_ = srv.Conn().Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		s.mu.Lock()
		s.lastThreadParams = append(json.RawMessage(nil), params...)
		s.mu.Unlock()
		_ = srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": s.cfg.ThreadID}})
	case codexschema.MethodThreadResume:
		s.mu.Lock()
		s.lastResumeParams = append(json.RawMessage(nil), params...)
		s.mu.Unlock()
		_ = srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": s.cfg.ThreadID}})
	case codexschema.MethodTurnStart:
		_ = srv.Conn().Reply(id, map[string]any{"turn": map[string]any{"id": s.cfg.TurnID}})
		s.handleTurnStart(params)
	}
}

func (s *Server) handleTurnStart(params json.RawMessage) {
	s.mu.Lock()
	srv := s.srv
	if srv == nil {
		s.mu.Unlock()
		return
	}
	s.lastTurnParams = append(json.RawMessage(nil), params...)
	p := parseTurnStartParams(params)
	s.lastCWD = p.CWD
	s.lastMessage = p.Message
	turnID := s.cfg.TurnID
	serveCtx := s.serveCtx
	s.mu.Unlock()
	if serveCtx == nil {
		serveCtx = context.Background()
	}

	tid := s.cfg.ThreadID

	_ = srv.EmitThreadStarted(tid, p.CWD)
	_ = srv.EmitTurnStarted(tid, turnID)

	if s.cfg.HangTurn {
		return
	}

	handler := s.cfg.Handler
	if handler == nil {
		handler = DefaultTurnHandler
	}
	// Run the handler off the Conn.Run read loop. codexclient.Conn.Run
	// dispatches OnNotification synchronously (see conn.go), so a handler
	// that issues a Conn.Request (item/tool/call, approval req) would block
	// this goroutine — and the reply, which must be pulled off the wire by
	// the same read loop, would never arrive. The old orchestrator toolCallServer
	// used the same `go func()` pattern for the same reason.
	//
	// Caveat: because turn-handler goroutines run detached, back-to-back
	// turn/start notifications can interleave on the wire (turn 2's
	// thread/started arrives before turn 1's turn/completed). Tests that
	// exercise multiple turns should `waitForMethods` on the full turn 1
	// sequence before issuing turn 2. Attach's stop joins these goroutines;
	// Serve callers who need the same behavior should call WaitInflight.
	emitter := &turnEmitter{srv: srv, threadID: tid, turnID: turnID, s: s}
	s.handlerWG.Add(1)
	go func() {
		defer s.handlerWG.Done()
		text, err := handler(serveCtx, TurnRequest{
			ThreadID: tid,
			CWD:      p.CWD,
			Message:  p.Message,
			Raw:      params,
		}, emitter)
		if err != nil {
			_ = srv.EmitTurnFailed(tid, err.Error())
			return
		}
		s.emitTokenUsage(srv, tid, turnID)
		_ = srv.EmitTurnCompleted(tid, turnID, text)
	}()
}

// OnNotification handles legacy turn/start notifications.
func (s *Server) OnNotification(method string, params json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	s.handleTurnStart(params)
}

func parseTurnStartParams(params json.RawMessage) struct {
	ThreadID string
	CWD      string
	Message  string
} {
	var payload struct {
		ThreadID string `json:"threadId"`
		CWD      string `json:"cwd"`
		Message  string `json:"message"`
		Input    []struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		} `json:"input"`
	}
	_ = json.Unmarshal(params, &payload)
	message := payload.Message
	if message == "" {
		for _, item := range payload.Input {
			if item.Type == "text" && item.Text != nil {
				message = *item.Text
				break
			}
		}
	}
	return struct {
		ThreadID string
		CWD      string
		Message  string
	}{
		ThreadID: payload.ThreadID,
		CWD:      payload.CWD,
		Message:  message,
	}
}

// WaitInflight blocks until every turn-handler goroutine spawned so far has
// returned. Attach's stop calls this after cancelling ctx; Serve callers
// running the loop directly should call it before draining test-owned
// resources the handlers might touch.
func (s *Server) WaitInflight() {
	s.handlerWG.Wait()
}

func (s *Server) emitTokenUsage(srv *codexclient.Server, threadID, turnID string) {
	last := s.cfg.TokenUsage.Last
	s.mu.Lock()
	s.totalUsage.InputTokens += last.InputTokens
	s.totalUsage.OutputTokens += last.OutputTokens
	s.totalUsage.CachedTokens += last.CachedTokens
	total := s.totalUsage
	s.mu.Unlock()
	_ = srv.EmitTokenUsage(threadID, turnID, breakdownMap(last), breakdownMap(total))
}

func breakdownMap(b TokenBreakdown) map[string]any {
	return map[string]any{
		"inputTokens":  b.InputTokens,
		"outputTokens": b.OutputTokens,
		"cachedTokens": b.CachedTokens,
		"totalTokens":  b.InputTokens + b.OutputTokens,
	}
}

// ---- observation API ----

// LastThreadParams returns the raw params of the most recent thread/start.
func (s *Server) LastThreadParams() json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastThreadParams
}

// LastTurnParams returns the raw params of the most recent turn/start.
func (s *Server) LastTurnParams() json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastTurnParams
}

// LastResumeParams returns the raw params of the most recent thread/resume.
func (s *Server) LastResumeParams() json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastResumeParams
}

// LastCWD returns the cwd of the most recent turn/start.
func (s *Server) LastCWD() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCWD
}

// LastMessage returns the rendered prompt of the most recent turn/start.
func (s *Server) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMessage
}

// ToolReplies returns the raw JSON reply bodies captured from any
// TurnEmitter.ToolCallRequest calls, in order.
func (s *Server) ToolReplies() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]json.RawMessage, len(s.toolReplies))
	copy(out, s.toolReplies)
	return out
}

// ---- turnEmitter ----

type turnEmitter struct {
	srv      *codexclient.Server
	threadID string
	turnID   string
	s        *Server
}

func (e *turnEmitter) AgentDelta(delta string) error {
	return e.srv.EmitAgentMessageDelta(e.threadID, delta)
}

func (e *turnEmitter) ItemStarted(item map[string]any) error {
	return e.srv.EmitItemStarted(e.threadID, e.turnID, item)
}

func (e *turnEmitter) ItemCompleted(item map[string]any) error {
	return e.srv.EmitItemCompleted(e.threadID, e.turnID, item)
}

func (e *turnEmitter) ToolCallRequest(tool string, arguments any, callID string) (json.RawMessage, error) {
	if callID == "" {
		callID = "call-1"
	}
	raw, err := e.srv.Conn().Request(codexschema.MethodItemToolCall, map[string]any{
		"tool":      tool,
		"arguments": arguments,
		"callId":    callID,
		"threadId":  e.threadID,
		"turnId":    e.turnID,
	})
	if err == nil {
		e.s.mu.Lock()
		e.s.toolReplies = append(e.s.toolReplies, append(json.RawMessage(nil), raw...))
		e.s.mu.Unlock()
	}
	return raw, err
}

func (e *turnEmitter) ThreadSettingsUpdated(settings map[string]any) error {
	return e.srv.EmitNotification(codexschema.MethodThreadSettingsUpdated, map[string]any{
		"threadId":       e.threadID,
		"threadSettings": settings,
	})
}
