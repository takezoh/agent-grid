package stream

// Interactive-flow integration tests: FakeAppServer + FakeCLI-in-pty + real
// Backend, wired against the same UDS socket a real deployment uses. These
// exercise the full JSON-RPC round trip through a real subprocess pty, so
// future changes to backend routing / driver dispatch are caught here rather
// than in production. The key invariant: a fresh interactive frame's CLI
// creates its own thread, backend's initState reservation keeps at most one
// adopt candidate pending, and handleThreadStarted binds them deterministically
// (ADR-0081).

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/runtime/subsystem/stream/fake"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

// newFakeServer boots an isolated FakeAppServer under t.TempDir() and returns
// it started. Cleanup is registered.
func newFakeServer(t *testing.T) *fake.AppServer {
	t.Helper()
	srv := fake.New(fake.Config{Sock: filepath.Join(t.TempDir(), "fake.sock")})
	if err := srv.Start(); err != nil {
		t.Fatalf("fake.Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv
}

// attachBackend wires a real Backend to a running fake app-server via its
// UDS socket. Replaces Backend.Start's spawnServer+dialUDS+initialize with a
// direct dial (same pattern as routing_wired_test.go's newWired but against a
// real WebSocket-over-UDS transport, not an in-memory pipe). Also launches
// the reap goroutine so the adopt-timeout safety net is active.
func attachBackend(t *testing.T, srv *fake.AppServer) (*Backend, *recordingRuntime) {
	t.Helper()
	rt := &recordingRuntime{}
	b := New(rt, nil, "sid", "sess1", "/p", "codex", nil, "", "", false, false, srv.SockPath(), 30*time.Second)

	tr, err := codexclient.DialUDS(srv.SockPath(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", srv.SockPath(), err)
	}
	b.conn = codexclient.NewConn(tr, 5*time.Second)
	b.ctx, b.cancel = context.WithCancel(context.Background())
	t.Cleanup(b.cancel)
	go func() { _ = b.conn.Run(b.ctx, b) }()
	go b.reapExpiredSlots()
	if err := codexclient.Initialize(b.conn); err != nil {
		t.Fatalf("backend initialize: %v", err)
	}
	return b, rt
}

// waitFor is a small deadline poll — same shape as fake/appserver_test.go's
// helper but package-local to avoid exporting purely for tests.
func waitForEvent(t *testing.T, rt *recordingRuntime, timeout time.Duration, pred func([]state.EvSubsystem) bool, msg string) {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		rt.mu.Lock()
		snapshot := append([]state.EvSubsystem(nil), rt.events...)
		rt.mu.Unlock()
		if pred(snapshot) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout (%s) waiting for %s; got %d events", timeout, msg, len(snapshot))
		case <-tick.C:
		}
	}
}

// TestInteractiveFlow_BackendReceivesCLIThreadBroadcast pins the wire-level
// fact that the fake app-server broadcasts a thread/started for the CLI's own
// thread to the backend's connection. This is the piece the T1/T2 problem
// depends on: backend HEARS about T2, it just can't route it today.
func TestInteractiveFlow_BackendReceivesCLIThreadBroadcast(t *testing.T) {
	srv := newFakeServer(t)
	_, _ = attachBackend(t, srv) // backend connected, its read loop is running

	// Spawn the CLI in a pty; it creates its own thread on connect.
	cli := fake.SpawnCLI(t, "--remote", "unix://"+srv.SockPath(), "--cd", "/work")
	cliThreadID := cli.Ready(t, 3*time.Second)
	if cliThreadID == "" {
		t.Fatal("CLI thread id is empty")
	}

	// The fake server must now have exactly one thread — the CLI's — since
	// nobody else has issued thread/start yet. Backend has NOT called
	// StartThread here (we bypassed Backend.Start's spawnServer path).
	threads := srv.Threads()
	if len(threads) != 1 || threads[0].ID != cliThreadID {
		t.Fatalf("fake.Threads() = %+v, want single %s", threads, cliThreadID)
	}
}

// TestInteractiveFlow_PromptBroadcastReachesBackend is the end-to-end
// acceptance test for the passive-adopt design: BindFrame reserves an initState
// slot with an empty threadID, the pty-spawned CLI issues its own thread/start,
// handleThreadStarted adopts it into the pending frame, and a subsequent
// prompt round-trips all the way to a driver Status transition.
func TestInteractiveFlow_PromptBroadcastReachesBackend(t *testing.T) {
	srv := newFakeServer(t)
	b, rt := attachBackend(t, srv)

	// BindFrame first (reserves the pending slot).
	frameID := state.FrameID("frame-A")
	if _, err := b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frameID,
		Plan:    state.LaunchPlan{StartDir: "/work"},
	}); err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	// Spawn the CLI. Its thread/start broadcast triggers adopt.
	cli := fake.SpawnCLI(t, "--remote", "unix://"+srv.SockPath(), "--cd", "/work")
	cliThreadID := cli.Ready(t, 3*time.Second)

	// Wait for adopt to complete — the binding's threadID should be filled
	// in with what the CLI told the app-server.
	adoptDeadline := time.Now().Add(2 * time.Second)
	for {
		b.mu.Lock()
		binding := b.frames[frameID]
		observedThreadID := ""
		if binding != nil {
			observedThreadID = binding.threadID
		}
		b.mu.Unlock()
		if observedThreadID == cliThreadID {
			break
		}
		if time.Now().After(adoptDeadline) {
			t.Fatalf("adopt did not bind CLI thread %q to frame %q within 2s", cliThreadID, frameID)
		}
		time.Sleep(2 * time.Millisecond)
	}
	waitForObserverSubscription(t, b, frameID, 3*time.Second)

	// User submits a prompt.
	cli.SendPrompt(t, "diagnose the app")

	// Backend should observe SubsystemTurnStarted → SubsystemTurnCompleted
	// on this frame (default TurnHandler emits both).
	waitForEvent(t, rt, 3*time.Second, func(evs []state.EvSubsystem) bool {
		var seenStart, seenComplete bool
		for _, e := range evs {
			if e.FrameID != frameID {
				continue
			}
			if e.Kind == state.SubsystemTurnStarted {
				seenStart = true
			}
			if e.Kind == state.SubsystemTurnCompleted {
				seenComplete = true
			}
		}
		return seenStart && seenComplete
	}, "TurnStarted+TurnCompleted for frame")

	// Feed the captured events through a fresh CodexDriver and assert its
	// Status ends at StatusWaiting (Idle → Running via TurnStarted → Waiting
	// via TurnCompleted). This is what the Card badge in the Web UI ultimately
	// reflects, so this assertion is the end-to-end guarantee the fake
	// harness is designed to provide.
	drv := driver.CodexDriver{}
	cs := drv.NewState(time.Now()).(driver.CodexState)
	ctx := state.FrameContext{ID: frameID, IsRoot: true}
	rt.mu.Lock()
	captured := append([]state.EvSubsystem(nil), rt.events...)
	rt.mu.Unlock()
	for _, ev := range captured {
		if ev.FrameID != frameID {
			continue
		}
		next, _, _ := drv.Step(cs, ctx, state.DEvSubsystem{
			Source:    ev.Source,
			Kind:      ev.Kind,
			Timestamp: ev.Timestamp,
			Payload:   ev.Payload,
		})
		cs = next.(driver.CodexState)
	}
	if cs.Status != state.StatusWaiting {
		t.Fatalf("driver Status = %v, want %v (event trace = %d frame events)",
			cs.Status, state.StatusWaiting, countFrameEvents(captured, frameID))
	}
}

func TestInteractiveFlow_SettingsUpdatedBroadcastReachesDriverMetadata(t *testing.T) {
	srv := fake.New(fake.Config{
		Sock: filepath.Join(t.TempDir(), "fake-settings.sock"),
		TurnHandler: func(req fake.TurnRequest, emit fake.Emitter) {
			_ = emit.Emit(codexschema.MethodTurnStarted, codexclient.TurnStartedParams(req.ThreadID, req.TurnID))
			_ = emit.Emit(codexschema.MethodThreadSettingsUpdated, map[string]any{
				"threadId": req.ThreadID,
				"threadSettings": map[string]any{
					"model":            "gpt-5-codex",
					"reasoning_effort": map[string]any{"level": "medium"},
				},
			})
			_ = emit.Emit(codexschema.MethodItemAgentMessageDelta,
				codexclient.AgentMessageDeltaParams(req.ThreadID, req.TurnID, "agent-"+req.TurnID, "done"))
			_ = emit.Emit(codexschema.MethodTurnCompleted, codexclient.TurnCompletedParams(req.ThreadID, req.TurnID))
		},
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("fake.Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	b, rt := attachBackend(t, srv)

	frameID := state.FrameID("frame-settings")
	if _, err := b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frameID,
		Plan:    state.LaunchPlan{StartDir: "/work"},
	}); err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	cli := fake.SpawnCLI(t, "--remote", "unix://"+srv.SockPath(), "--cd", "/work")
	_ = cli.Ready(t, 3*time.Second)
	cli.SendPrompt(t, "show metadata")

	waitForEvent(t, rt, 3*time.Second, func(evs []state.EvSubsystem) bool {
		for _, e := range evs {
			if e.FrameID != frameID || e.Kind != state.SubsystemMetadataUpdated {
				continue
			}
			if e.Payload.Model == "gpt-5-codex" && e.Payload.Effort == "medium" && e.Payload.ModelSet && e.Payload.EffortSet {
				return true
			}
		}
		return false
	}, "MetadataUpdated for settings event")

	drv := driver.CodexDriver{}
	cs := drv.NewState(time.Now()).(driver.CodexState)
	ctx := state.FrameContext{ID: frameID, IsRoot: true}
	rt.mu.Lock()
	captured := append([]state.EvSubsystem(nil), rt.events...)
	rt.mu.Unlock()
	for _, ev := range captured {
		if ev.FrameID != frameID {
			continue
		}
		next, _, _ := drv.Step(cs, ctx, state.DEvSubsystem{
			Source:    ev.Source,
			Kind:      ev.Kind,
			Timestamp: ev.Timestamp,
			Payload:   ev.Payload,
		})
		cs = next.(driver.CodexState)
	}
	if !cs.ModelSet || cs.Model != "gpt-5-codex" {
		t.Fatalf("driver model = %q (set=%v), want gpt-5-codex", cs.Model, cs.ModelSet)
	}
	if !cs.EffortSet || cs.Effort != "medium" {
		t.Fatalf("driver effort = %q (set=%v), want medium", cs.Effort, cs.EffortSet)
	}
}

// TestInteractiveFlow_ThreadStatusChangedBroadcastReachesBackend pins the
// wire-level dispatch of thread/status/changed (AC-007): a notification with
// no `id` member must still decode and route to handleThreadStatusChanged
// through the real Conn.Run loop after RequestID replaced the fixed-width
// int64 id. Isolated from turn/started and turn/completed so the assertion is
// specifically about the thread/status/changed → SubsystemTurnStarted path.
func TestInteractiveFlow_ThreadStatusChangedBroadcastReachesBackend(t *testing.T) {
	srv := fake.New(fake.Config{
		Sock: filepath.Join(t.TempDir(), "fake-status.sock"),
		TurnHandler: func(req fake.TurnRequest, emit fake.Emitter) {
			_ = emit.Emit(codexschema.MethodThreadStatusChanged, map[string]any{
				"threadId": req.ThreadID,
				"status":   map[string]any{"type": "active", "activeFlags": []any{}},
			})
		},
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("fake.Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	b, rt := attachBackend(t, srv)

	frameID := state.FrameID("frame-status")
	if _, err := b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frameID,
		Plan:    state.LaunchPlan{StartDir: "/work"},
	}); err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	cli := fake.SpawnCLI(t, "--remote", "unix://"+srv.SockPath(), "--cd", "/work")
	_ = cli.Ready(t, 3*time.Second)
	waitForObserverSubscription(t, b, frameID, 3*time.Second)
	cli.SendPrompt(t, "trigger status change")

	waitForEvent(t, rt, 3*time.Second, func(evs []state.EvSubsystem) bool {
		for _, e := range evs {
			if e.FrameID == frameID && e.Kind == state.SubsystemTurnStarted {
				return true
			}
		}
		return false
	}, "SubsystemTurnStarted from thread/status/changed")
}

func waitForObserverSubscription(t *testing.T, b *Backend, frameID state.FrameID, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		binding := b.frames[frameID]
		subscribed := binding != nil && binding.observerSubscribed && binding.canonicalIdentityValidated
		b.mu.Unlock()
		if subscribed {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("observer subscription for frame %s did not complete", frameID)
}

func countFrameEvents(evs []state.EvSubsystem, frameID state.FrameID) int {
	n := 0
	for _, e := range evs {
		if e.FrameID == frameID {
			n++
		}
	}
	return n
}
