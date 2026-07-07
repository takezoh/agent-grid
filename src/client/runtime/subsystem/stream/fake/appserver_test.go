package fake

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

// recorderClient captures every notification a client receives, keyed by
// method. Broadcast assertions read from this in the tests below.
type recorderClient struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	Method string
	Params json.RawMessage
}

func (r *recorderClient) OnNotification(method string, params json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{Method: method, Params: append(json.RawMessage(nil), params...)})
}
func (r *recorderClient) OnServerRequest(codexclient.RequestID, string, json.RawMessage) {}

func (r *recorderClient) filter(method string) []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedEvent
	for _, e := range r.events {
		if e.Method == method {
			out = append(out, e)
		}
	}
	return out
}

// dialClient starts a new client conn against the fake and drives its read
// loop until the test completes.
func dialClient(t *testing.T, sock string) (*codexclient.Conn, *recorderClient) {
	t.Helper()
	tr, err := codexclient.DialUDS(sock, 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn := codexclient.NewConn(tr, 5*time.Second)
	rec := &recorderClient{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = conn.Run(ctx, rec) }()
	if err := codexclient.Initialize(conn); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return conn, rec
}

func startFake(t *testing.T, cfg Config) *AppServer {
	t.Helper()
	if cfg.Sock == "" {
		cfg.Sock = filepath.Join(t.TempDir(), "fake.sock")
	}
	srv := New(cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv
}

// waitFor loops until predicate() returns true or the deadline hits. Avoids
// hard-coded sleep; broadcast delivery is async through the coder/websocket
// write path.
func waitFor(t *testing.T, timeout time.Duration, pred func() bool, msg string) {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		if pred() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s", msg)
		case <-tick.C:
		}
	}
}

func TestThreadStartReturnsUniqueIDsAndBroadcasts(t *testing.T) {
	srv := startFake(t, Config{})
	conn, rec := dialClient(t, srv.SockPath())

	sess, err := codexclient.StartThread(conn, "/work-a", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread A: %v", err)
	}
	if sess.ThreadID == "" || sess.ThreadID != sess.SessionID {
		t.Fatalf("unexpected session: %+v", sess)
	}

	sess2, err := codexclient.StartThread(conn, "/work-b", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread B: %v", err)
	}
	if sess2.ThreadID == sess.ThreadID {
		t.Fatalf("duplicate thread id %q", sess.ThreadID)
	}

	// Both thread.started notifications must have reached the same client that
	// issued the requests (broadcast fan-out includes the requester).
	waitFor(t, time.Second, func() bool { return len(rec.filter(codexschema.MethodThreadStarted)) >= 2 },
		"two thread.started broadcasts")

	threads := srv.Threads()
	if len(threads) != 2 {
		t.Fatalf("srv.Threads() = %d, want 2", len(threads))
	}
}

func TestThreadStartedIsBroadcastToOtherClient(t *testing.T) {
	// This is the invariant that makes the T1/T2 problem reproducible: when
	// client B (the CLI) creates a thread, client A (the backend) still hears
	// about it via broadcast.
	srv := startFake(t, Config{})
	connA, recA := dialClient(t, srv.SockPath())
	_, recB := dialClient(t, srv.SockPath())

	if got := srv.ClientCount(); got != 2 {
		t.Fatalf("ClientCount = %d, want 2", got)
	}

	sess, err := codexclient.StartThread(connA, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}

	// Both clients must receive thread/started for the same thread.
	waitFor(t, time.Second, func() bool { return len(recA.filter(codexschema.MethodThreadStarted)) == 1 }, "A hears thread.started")
	waitFor(t, time.Second, func() bool { return len(recB.filter(codexschema.MethodThreadStarted)) == 1 }, "B hears thread.started")

	// And both events carry the same thread id.
	extract := func(events []recordedEvent) string {
		if len(events) == 0 {
			return ""
		}
		var payload map[string]any
		_ = json.Unmarshal(events[0].Params, &payload)
		thread, _ := payload["thread"].(map[string]any)
		id, _ := thread["id"].(string)
		return id
	}
	if extract(recA.filter(codexschema.MethodThreadStarted)) != sess.ThreadID {
		t.Fatalf("A saw wrong thread id")
	}
	if extract(recB.filter(codexschema.MethodThreadStarted)) != sess.ThreadID {
		t.Fatalf("B saw wrong thread id")
	}
}

func TestTurnStartDrivesDefaultLifecycleBroadcast(t *testing.T) {
	srv := startFake(t, Config{})
	conn, rec := dialClient(t, srv.SockPath())
	sess, err := codexclient.StartThread(conn, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if err := codexclient.StartTurn(conn, sess.ThreadID, "/work", []byte("hello"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// Default TurnHandler emits: turn/started, thread/status active,
	// turn/completed, thread/status idle. In addition thread/started was
	// broadcast earlier.
	wantSeq := []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodThreadStatusChanged,
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodTurnCompleted,
		codexschema.MethodThreadStatusChanged,
	}
	waitFor(t, 2*time.Second, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.events) >= len(wantSeq)
	}, "default lifecycle broadcast")
	rec.mu.Lock()
	got := make([]string, len(rec.events))
	for i, e := range rec.events {
		got[i] = e.Method
	}
	rec.mu.Unlock()
	for i, m := range wantSeq {
		if got[i] != m {
			t.Fatalf("event[%d] = %q, want %q (all=%v)", i, got[i], m, got)
		}
	}

	completed := rec.filter(codexschema.MethodTurnCompleted)
	if len(completed) != 1 {
		t.Fatalf("turn/completed count = %d, want 1", len(completed))
	}
	var payload map[string]any
	if err := json.Unmarshal(completed[0].Params, &payload); err != nil {
		t.Fatalf("unmarshal turn/completed: %v", err)
	}
	if got := payload["threadId"]; got != sess.ThreadID {
		t.Fatalf("turn/completed threadId = %v, want %q", got, sess.ThreadID)
	}
	turn, _ := payload["turn"].(map[string]any)
	if got := turn["id"]; got != "fake-turn-1" {
		t.Fatalf("turn/completed turn.id = %v, want %q", got, "fake-turn-1")
	}
	deltas := rec.filter(codexschema.MethodItemAgentMessageDelta)
	if len(deltas) != 1 {
		t.Fatalf("delta count = %d, want 1", len(deltas))
	}
	var deltaPayload map[string]any
	if err := json.Unmarshal(deltas[0].Params, &deltaPayload); err != nil {
		t.Fatalf("unmarshal delta: %v", err)
	}
	if got := deltaPayload["delta"]; got != "echo: hello" {
		t.Fatalf("delta = %v, want %q", got, "echo: hello")
	}
}

func TestTurnStartScopesTurnNotificationsToInitiator(t *testing.T) {
	srv := startFake(t, Config{})
	initiator, initRec := dialClient(t, srv.SockPath())
	_, observerRec := dialClient(t, srv.SockPath())

	sess, err := codexclient.StartThread(initiator, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if err := codexclient.StartTurn(initiator, sess.ThreadID, "/work", []byte("hello"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		return len(initRec.filter(codexschema.MethodTurnStarted)) == 1 &&
			len(initRec.filter(codexschema.MethodTurnCompleted)) == 1 &&
			len(initRec.filter(codexschema.MethodThreadStatusChanged)) >= 2
	}, "initiator turn lifecycle")
	waitFor(t, 2*time.Second, func() bool {
		return len(observerRec.filter(codexschema.MethodThreadStatusChanged)) >= 2
	}, "observer thread status lifecycle")

	if got := len(observerRec.filter(codexschema.MethodTurnStarted)); got != 0 {
		t.Fatalf("observer turn.started count = %d, want 0", got)
	}
	if got := len(observerRec.filter(codexschema.MethodTurnCompleted)); got != 0 {
		t.Fatalf("observer turn.completed count = %d, want 0", got)
	}
}

func TestTurnStartTriggersHandlerScripting(t *testing.T) {
	// Custom handler emits a scripted notification; assert the fake actually
	// invokes it and its output reaches the client via broadcast.
	handlerCalls := new(atomic.Int32)
	srv := startFake(t, Config{
		TurnHandler: func(req TurnRequest, emit Emitter) {
			handlerCalls.Add(1)
			_ = emit.Emit("test/custom", map[string]any{
				"threadId": req.ThreadID,
				"echo":     req.Input,
			})
		},
	})
	conn, rec := dialClient(t, srv.SockPath())
	sess, _ := codexclient.StartThread(conn, "/work", nil, codexclient.ThreadOptions{})
	if err := codexclient.StartTurn(conn, sess.ThreadID, "/work", []byte("hi there"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	waitFor(t, time.Second, func() bool { return handlerCalls.Load() == 1 }, "handler invocation")
	waitFor(t, time.Second, func() bool { return len(rec.filter("test/custom")) == 1 }, "custom broadcast")
}

func TestCustomTurnHandlerCannotLeakTurnNotificationsToObserver(t *testing.T) {
	srv := startFake(t, Config{
		TurnHandler: func(req TurnRequest, emit Emitter) {
			_ = emit.Emit(codexschema.MethodTurnStarted, codexclient.TurnStartedParams(req.ThreadID, req.TurnID))
			_ = emit.Emit(codexschema.MethodTurnCompleted, codexclient.TurnCompletedParams(req.ThreadID, req.TurnID))
			_ = emit.Emit(codexschema.MethodThreadStatusChanged, map[string]any{
				"threadId": req.ThreadID,
				"status":   map[string]any{"type": "idle"},
			})
		},
	})
	initiator, initRec := dialClient(t, srv.SockPath())
	_, observerRec := dialClient(t, srv.SockPath())

	sess, err := codexclient.StartThread(initiator, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if err := codexclient.StartTurn(initiator, sess.ThreadID, "/work", []byte("hello"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		return len(initRec.filter(codexschema.MethodTurnStarted)) == 1 &&
			len(initRec.filter(codexschema.MethodTurnCompleted)) == 1
	}, "initiator sees turn notifications")
	waitFor(t, time.Second, func() bool {
		return len(observerRec.filter(codexschema.MethodThreadStatusChanged)) == 1
	}, "observer sees thread notification")
	if got := len(observerRec.filter(codexschema.MethodTurnStarted)); got != 0 {
		t.Fatalf("observer turn.started count = %d, want 0", got)
	}
	if got := len(observerRec.filter(codexschema.MethodTurnCompleted)); got != 0 {
		t.Fatalf("observer turn.completed count = %d, want 0", got)
	}
}

func TestThreadResumeReloadsPreviouslyCreatedThread(t *testing.T) {
	dir := t.TempDir()
	srv := startFake(t, Config{RolloutDir: dir})

	// Client A creates a thread; grab its id.
	connA, _ := dialClient(t, srv.SockPath())
	sess, err := codexclient.StartThread(connA, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}

	// A separate client B resumes it. Fake looks up by threadId first.
	connB, _ := dialClient(t, srv.SockPath())
	resumed, err := codexclient.ResumeThread(connB, codexclient.ResumeOptions{ThreadID: sess.ThreadID})
	if err != nil {
		t.Fatalf("ResumeThread: %v", err)
	}
	if resumed.ThreadID != sess.ThreadID {
		t.Fatalf("resume returned %q, want %q", resumed.ThreadID, sess.ThreadID)
	}
	if resumed.RolloutPath == "" {
		t.Fatalf("resume returned empty rollout path; RolloutDir=%s", dir)
	}
}

// TestOnServerRequestEchoesOpaqueRequestIDBytes pins FR-007: serverConn's
// Handler implementation must pass the wire "id" member through to Reply
// verbatim rather than decoding it into a fixed int64, so a JSON string id
// (and, separately, the historic decimal id) both round-trip as the exact
// same bytes. Dials with the raw Transport (bypassing codexclient.Conn's own
// request bookkeeping, which always mints decimal ids) so the id on the wire
// is fully under the test's control.
func TestOnServerRequestEchoesOpaqueRequestIDBytes(t *testing.T) {
	srv := startFake(t, Config{})

	tr, err := codexclient.DialUDS(srv.SockPath(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, wireID := range []string{`"initialize"`, `42`} {
		req := []byte(`{"jsonrpc":"2.0","id":` + wireID + `,"method":"initialize","params":{}}`)
		if err := tr.WriteMessage(ctx, req); err != nil {
			t.Fatalf("WriteMessage(id=%s): %v", wireID, err)
		}

		data, err := tr.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage(id=%s): %v", wireID, err)
		}
		var resp struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("unmarshal reply(id=%s): %v", wireID, err)
		}
		if string(resp.ID) != wireID {
			t.Fatalf("reply id = %s, want %s (echoed bytes must match request exactly)", resp.ID, wireID)
		}
	}
}

func TestThreadResumeUnknownReturnsError(t *testing.T) {
	srv := startFake(t, Config{})
	conn, _ := dialClient(t, srv.SockPath())
	_, err := codexclient.ResumeThread(conn, codexclient.ResumeOptions{ThreadID: "does-not-exist"})
	if err == nil {
		t.Fatal("ResumeThread on unknown id should error")
	}
}
