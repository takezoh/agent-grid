package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

func TestReconcileSkipsInFlightSpawnedFrame(t *testing.T) {
	backend := NewPtyBackend(0)
	r := New(Config{Backend: backend})

	const sessID = state.SessionID("proactive-session")
	const frameID = state.FrameID("proactive-frame")
	r.state.Sessions[sessID] = testShellSession(sessID, frameID)
	r.pendingSpawns[frameID] = struct{}{}

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		t.Fatalf("reconcile must skip in-flight spawn, got %T (%+v)", ev, ev)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestPendingSpawnClearedOnFrameSpawned(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{Backend: backend})

	const sessID = state.SessionID("spawned-session")
	const frameID = state.FrameID("spawned-frame")
	r.state.Sessions[sessID] = testShellSession(sessID, frameID)
	r.pendingSpawns[frameID] = struct{}{}

	r.dispatch(state.EvFrameSpawned{SessionID: sessID, FrameID: frameID})

	if _, ok := r.pendingSpawns[frameID]; ok {
		t.Fatal("pending spawn remained after EvFrameSpawned")
	}

	r.reconcileWindows()

	assertVanishedEvent(t, r.eventCh, frameID)
}

func TestPendingSpawnClearedOnSpawnFailed(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{Backend: backend})

	const sessID = state.SessionID("failed-session")
	const frameID = state.FrameID("failed-frame")
	r.state.Sessions[sessID] = testShellSession(sessID, frameID)
	r.pendingSpawns[frameID] = struct{}{}

	r.dispatch(state.EvSpawnFailed{SessionID: sessID, FrameID: frameID, Err: "boom"})

	if _, ok := r.pendingSpawns[frameID]; ok {
		t.Fatal("pending spawn remained after EvSpawnFailed")
	}

	r.state.Sessions[sessID] = testShellSession(sessID, frameID)
	r.reconcileWindows()

	assertVanishedEvent(t, r.eventCh, frameID)
}

func TestReconcileStillCollectsBootstrapFailedFrame(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{Backend: backend})

	const sessID = state.SessionID("bootstrap-session")
	const frameID = state.FrameID("bootstrap-frame")
	r.state.Sessions[sessID] = testShellSession(sessID, frameID)

	r.reconcileWindows()

	assertVanishedEvent(t, r.eventCh, frameID)
}

func testShellSession(sessID state.SessionID, frameID state.FrameID) state.Session {
	now := time.Now()
	drv := state.GetDriver("shell")
	frameDriver := drv.NewState(now)
	return state.Session{
		ID:      sessID,
		Command: "shell",
		Driver:  drv.NewState(now),
		Frames: []state.SessionFrame{{
			ID:      frameID,
			Command: "shell",
			Driver:  frameDriver,
		}},
	}
}

func assertVanishedEvent(t *testing.T, eventCh <-chan state.Event, frameID state.FrameID) {
	t.Helper()
	select {
	case ev := <-eventCh:
		vanished, ok := ev.(state.EvFrameVanished)
		if !ok {
			t.Fatalf("expected EvFrameVanished, got %T (%+v)", ev, ev)
		}
		if vanished.FrameID != frameID {
			t.Fatalf("vanish for wrong frame: got %q want %q", vanished.FrameID, frameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected EvFrameVanished for %q", frameID)
	}
}
