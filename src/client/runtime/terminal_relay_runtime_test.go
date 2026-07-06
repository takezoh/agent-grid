package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

func TestDispatchInternalIgnoresStaleSurfaceClosedAfterResubscribe(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	key := surfaceKey{connID: conn1, sessionID: sess1}

	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		conn1: {sess1: {}},
	}
	r.terminalRelay = &TerminalRelay{
		subs: map[surfaceKey]*surfaceSub{
			key: {
				frameID: "%1",
				subID:   2,
				cancel:  make(chan struct{}),
			},
		},
	}

	r.dispatchInternal(internalSurfaceClosed{
		ConnID:    conn1,
		SessionID: sess1,
		FrameID:   "%1",
		SubID:     1,
	})

	if sub := r.terminalRelay.subs[key]; sub == nil || sub.subID != 2 {
		t.Fatalf("terminal relay subscription = %+v, want live subID 2", sub)
	}
	if _, ok := r.state.SurfaceSubs[conn1][sess1]; !ok {
		t.Fatal("state.SurfaceSubs lost live subscription after stale close")
	}
}

func TestDispatchInternalIgnoresStaleSurfaceClosedFromPreviousFrame(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	key := surfaceKey{connID: conn1, sessionID: sess1}

	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		conn1: {sess1: {}},
	}
	r.terminalRelay = &TerminalRelay{
		subs: map[surfaceKey]*surfaceSub{
			key: {
				frameID: "%2",
				subID:   1,
				cancel:  make(chan struct{}),
			},
		},
	}

	r.dispatchInternal(internalSurfaceClosed{
		ConnID:    conn1,
		SessionID: sess1,
		FrameID:   "%1",
		SubID:     1,
	})

	if sub := r.terminalRelay.subs[key]; sub == nil || sub.frameID != "%2" || sub.subID != 1 {
		t.Fatalf("terminal relay subscription = %+v, want live frame %%2 subID 1", sub)
	}
	if _, ok := r.state.SurfaceSubs[conn1][sess1]; !ok {
		t.Fatal("state.SurfaceSubs lost live subscription after stale cross-frame close")
	}
}

func TestDispatchInternalAppliesCurrentSurfaceClosed(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})

	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		conn1: {sess1: {}},
	}
	r.terminalRelay = &TerminalRelay{
		subs: map[surfaceKey]*surfaceSub{},
	}

	r.dispatchInternal(internalSurfaceClosed{
		ConnID:    conn1,
		SessionID: sess1,
		FrameID:   "%1",
		SubID:     1,
	})

	if _, ok := r.state.SurfaceSubs[conn1][sess1]; ok {
		t.Fatal("state.SurfaceSubs retained subscription after current close")
	}
}

func TestDispatchSurfaceSubscribeReconcilesMissingRelaySubscription(t *testing.T) {
	r, _ := newTestRuntimeWithConns(t, conn1)
	b := newFakeSurfaceBackend()
	r.terminalRelay = NewTerminalRelay(
		b,
		func(ev internalEvent) bool { return true },
		func(ev internalEvent) {},
	)
	r.state.Sessions = map[state.SessionID]state.Session{
		sess1: {
			ID:        sess1,
			CreatedAt: time.Now(),
			Frames: []state.SessionFrame{
				{ID: state.FrameID("%1"), CreatedAt: time.Now()},
			},
		},
	}
	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		conn1: {sess1: {}},
	}

	r.dispatch(state.EvCmdSurfaceSubscribe{ConnID: conn1, ReqID: "r1", SessionID: sess1})

	if !r.terminalRelay.hasSubscription(conn1, sess1) {
		t.Fatal("terminal relay did not recreate missing subscription on idempotent subscribe")
	}
	if got := b.nextID.Load(); got != 1 {
		t.Fatalf("backend subscribe count = %d, want 1 recreated subscriber", got)
	}
}
