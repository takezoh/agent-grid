package stream

// Wired routing harness: drives the backend through its real codexclient.Conn
// against a real WebSocket-over-UDS FakeAppServer. Unlike the direct-drive
// contract (routing_contract_test.go), this exercises the async read loop,
// so it runs under `go test -race`, and it pins that a real cold BindFrame
// binds a distinct thread id synchronously — making cross-talk between
// same-cwd frames structurally impossible.
//
// Uses fake.AppServer (the same fake used by interactive_flow_test.go) so
// the wire behaviour under test — broadcast of every notification to every
// connected client — is identical to production.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/runtime/subsystem/stream/fake"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

type wired struct {
	t   *testing.T
	b   *Backend
	rt  *recordingRuntime
	srv *fake.AppServer
}

func newWired(t *testing.T) *wired {
	t.Helper()
	rt := &recordingRuntime{}
	srv := fake.New(fake.Config{Sock: filepath.Join(t.TempDir(), "wired.sock")})
	if err := srv.Start(); err != nil {
		t.Fatalf("fake.Start: %v", err)
	}
	t.Cleanup(srv.Stop)

	b := New(rt, nil, "sid", "sess1", "/p", "codex", nil, "", "", false, false, srv.SockPath(), time.Second)
	tr, err := codexclient.DialUDS(srv.SockPath(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", srv.SockPath(), err)
	}
	b.conn = codexclient.NewConn(tr, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go b.conn.Run(ctx, b) //nolint:errcheck
	if err := codexclient.Initialize(b.conn); err != nil {
		t.Fatalf("backend initialize: %v", err)
	}
	return &wired{t: t, b: b, rt: rt, srv: srv}
}

// bindCold runs a fresh cold-start BindFrame and then simulates the CLI
// issuing thread/start by broadcasting thread/started on the fake. The
// Backend's adopt path binds that id into the pending frame. Returns the
// adopted thread id.
//
// This is the async equivalent of the pre-restructure synchronous
// thread/start path: the Backend no longer creates the thread itself; the
// CLI does. Tests inject a fresh id via the fake's Broadcast helper.
func (w *wired) bindCold(frame state.FrameID, dir string) string {
	w.t.Helper()
	if _, err := w.b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frame,
		Plan:    state.LaunchPlan{StartDir: dir},
	}); err != nil {
		w.t.Fatalf("BindFrame(%s): %v", frame, err)
	}
	threadID := "wired-thread-" + string(frame)
	if err := w.srv.Broadcast(codexschema.MethodThreadStarted, map[string]any{
		"thread": map[string]any{"id": threadID, "cwd": dir},
	}); err != nil {
		w.t.Fatalf("mint thread/started: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.b.mu.Lock()
		binding := w.b.frames[frame]
		observedThreadID := ""
		if binding != nil {
			observedThreadID = binding.threadID
		}
		w.b.mu.Unlock()
		if observedThreadID != "" {
			return observedThreadID
		}
		time.Sleep(2 * time.Millisecond)
	}
	w.t.Fatalf("BindFrame(%s) not adopted within timeout", frame)
	return ""
}

func (w *wired) emitMessage(threadID, delta string) {
	w.t.Helper()
	if err := w.srv.Broadcast(codexschema.MethodItemAgentMessageDelta, map[string]any{
		"threadId": threadID,
		"delta":    delta,
	}); err != nil {
		w.t.Fatalf("Broadcast agent message delta: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(w.rt.framesWithMarker(delta)) == 0 {
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for marker %s", delta)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestStreamRoutingWiredIsolation: two frames sharing a cwd each get a distinct
// thread id at creation, so the real async event stream routes each marker to
// its own frame — cross-talk is structurally impossible. Run under -race.
func TestStreamRoutingWiredIsolation(t *testing.T) {
	w := newWired(t)
	tA := w.bindCold("A", "/work")
	tB := w.bindCold("B", "/work")
	if tA == tB {
		t.Fatalf("same-cwd frames must get distinct thread ids, both = %q", tA)
	}
	w.emitMessage(tA, "MARK_A")
	w.emitMessage(tB, "MARK_B")
	assertMarkerFrames(t, w.rt, "MARK_A", "A")
	assertMarkerFrames(t, w.rt, "MARK_B", "B")
}
