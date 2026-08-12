package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/agentlaunch"
)

type fakeRuntime struct {
	events []state.Event
}

func (f *fakeRuntime) Enqueue(e state.Event) { f.events = append(f.events, e) }

func TestStopBeforeStartIsNoop(t *testing.T) {
	b, _ := newTestBackend()
	// Never Started: cancel and done are nil. Stop must not panic or block.
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)
}

func TestWaitProcessIntentionalShutdownDoesNotFailFrames(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	close(b.done)
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return context.Canceled }}
	b.waitProcess()
	if len(rt.events) != 0 {
		t.Fatalf("intentional Stop emitted subsystem failures: %#v", rt.events)
	}
}

func TestWaitProcessUnexpectedExitFailsEachBoundFrameOnce(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return errors.New("boom") }}

	b.waitProcess()
	if len(rt.events) != 1 {
		t.Fatalf("unexpected Wait events = %d, want one per bound frame: %#v", len(rt.events), rt.events)
	}
	event, ok := rt.events[0].(state.EvSubsystem)
	if !ok || event.FrameID != "f1" || event.Kind != state.SubsystemFailed {
		t.Fatalf("unexpected Wait event = %#v", rt.events[0])
	}
}

func TestWaitProcessWinsTerminalRaceAndLaterStopCannotReclassifyExit(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return errors.New("boom") }}

	b.waitProcess()
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	b.mu.Lock()
	observed, expected, cause := b.terminalObserved, b.terminalExpected, b.terminalCause
	b.mu.Unlock()
	if !observed || expected || cause != 0 {
		t.Fatalf("terminal observation = (observed=%v expected=%v cause=%v), want first unexpected observation retained",
			observed, expected, cause)
	}
	if len(rt.events) != 1 {
		t.Fatalf("later Stop reclassified or duplicated unexpected exit events: %#v", rt.events)
	}
}

func TestStopCancelsAndWaitsForReap(t *testing.T) {
	b, _ := newTestBackend()
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.done = make(chan struct{})
	// Emulate waitProcess: closes done once the subsystem ctx is cancelled.
	go func() {
		<-b.ctx.Done()
		close(b.done)
	}()

	start := time.Now()
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)
	if elapsed := time.Since(start); elapsed >= stopGrace {
		t.Fatalf("Stop blocked %v (>= grace %v); did not observe reap", elapsed, stopGrace)
	}
	select {
	case <-b.done:
	default:
		t.Fatal("Stop returned before done was closed")
	}
}

func TestShimArgs_UsesHostHelperPath(t *testing.T) {
	b, _ := newTestBackend()
	b.helperBin = "/host/bin/bridge"
	b.listenSock = "/tmp/codex.sock"
	b.serverBin = "codex"
	args := b.shimArgs()
	if len(args) == 0 {
		t.Fatal("shimArgs returned empty argv")
	}
	if got := args[0]; got != "/host/bin/bridge" {
		t.Fatalf("shim argv[0] = %q, want host helper path", got)
	}
}

func TestBackendKind(t *testing.T) {
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", "", "/sock", 0)
	if b.Kind() != state.LaunchSubsystemStream {
		t.Errorf("Kind = %v", b.Kind())
	}
}

func TestReleaseFrameAndLookup(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	if got := b.frameForThread("t1"); got != "f1" {
		t.Errorf("frameForThread = %q", got)
	}
	if got := b.frameForThread(""); got != "" {
		t.Errorf("empty threadID should return empty FrameID, got %q", got)
	}
	if got := b.frameForThread("unknown"); got != "" {
		t.Errorf("unknown should return empty, got %q", got)
	}

	b.ReleaseFrame("f1")
	b.mu.Lock()
	_, frameOK := b.frames["f1"]
	_, threadOK := b.threads["t1"]
	b.mu.Unlock()
	if frameOK || threadOK {
		t.Errorf("ReleaseFrame did not clean up: frames=%v threads=%v", frameOK, threadOK)
	}

	// idempotent
	b.ReleaseFrame("nonexistent")
}

func TestFactoryRange(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	f.backends["a"] = &Backend{subsystemID: "a"}
	f.backends["b"] = &Backend{subsystemID: "b"}
	seen := map[state.SubsystemID]bool{}
	f.Range(func(b *Backend) bool {
		seen[b.subsystemID] = true
		return true
	})
	if len(seen) != 2 {
		t.Errorf("Range visited %d, want 2", len(seen))
	}
	// early termination
	count := 0
	f.Range(func(*Backend) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("early-stop visited %d, want 1", count)
	}
}
