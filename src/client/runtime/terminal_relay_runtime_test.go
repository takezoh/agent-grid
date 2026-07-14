package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

func TestDispatchInternalIgnoresStaleSurfaceClosedAfterResubscribe(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	key := surfaceKey{connID: conn1, sessionID: sess1}

	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
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
	if _, ok := r.state.SurfaceSubs[conn1][state.SurfaceSubscription{SessionID: sess1}]; !ok {
		t.Fatal("state.SurfaceSubs lost live subscription after stale close")
	}
}

func TestDispatchInternalIgnoresStaleSurfaceClosedFromPreviousFrame(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	key := surfaceKey{connID: conn1, sessionID: sess1}

	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
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
	if _, ok := r.state.SurfaceSubs[conn1][state.SurfaceSubscription{SessionID: sess1}]; !ok {
		t.Fatal("state.SurfaceSubs lost live subscription after stale cross-frame close")
	}
}

func TestDispatchInternalAppliesCurrentSurfaceClosed(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})

	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
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

	if _, ok := r.state.SurfaceSubs[conn1][state.SurfaceSubscription{SessionID: sess1}]; ok {
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
	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
	}

	r.dispatch(state.EvCmdSurfaceSubscribe{ConnID: conn1, ReqID: "r1", SessionID: sess1})

	if !r.terminalRelay.hasSubscription(conn1, sess1) {
		t.Fatal("terminal relay did not recreate missing subscription on idempotent subscribe")
	}
	if got := b.nextID.Load(); got != 1 {
		t.Fatalf("backend subscribe count = %d, want 1 recreated subscriber", got)
	}
}

func TestSetHeadFrameRebindsLiveSurfaceSubscription(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	b := newFakeSurfaceBackend()
	r.terminalRelay = NewTerminalRelay(
		b,
		func(ev internalEvent) bool { return true },
		func(ev internalEvent) {},
	)
	defer r.terminalRelay.Close()

	r.state.Sessions = map[state.SessionID]state.Session{
		sess1: {
			ID:          sess1,
			CreatedAt:   time.Now(),
			HeadFrameID: "%1",
			Frames: []state.SessionFrame{
				{ID: "%1", CreatedAt: time.Now()},
				{ID: "%2", CreatedAt: time.Now()},
			},
		},
	}
	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
	}
	if err := r.terminalRelay.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("subscribe old head: %v", err)
	}

	r.dispatch(state.EvEvent{
		Event:   state.EventSetHeadFrame,
		Payload: []byte(`{"session_id":"sess-1","frame_id":"%2"}`),
	})

	key := surfaceKey{connID: conn1, sessionID: sess1}
	r.terminalRelay.mu.Lock()
	sub := r.terminalRelay.subs[key]
	r.terminalRelay.mu.Unlock()
	if sub == nil || sub.frameID != "%2" {
		t.Fatalf("terminal relay subscription = %+v, want current head frame %%2", sub)
	}
}

func TestSurfaceSubscriptionFollowsHeadAcrossPushAndFallback(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	b := newFakeSurfaceBackend()
	r.terminalRelay = NewTerminalRelay(
		b,
		func(ev internalEvent) bool { return true },
		func(ev internalEvent) {},
	)
	defer r.terminalRelay.Close()

	now := time.Now()
	drv := state.GetDriver("shell")
	if drv == nil {
		t.Fatal("shell driver is not registered")
	}
	r.state.Now = now
	r.state.Sessions = map[state.SessionID]state.Session{
		sess1: {
			ID:          sess1,
			Project:     "/tmp/project",
			Command:     "shell",
			CreatedAt:   now,
			HeadFrameID: "%root",
			Frames: []state.SessionFrame{
				{
					ID:      "%root",
					Project: "/tmp/project",
					Command: "shell",
					Driver:  drv.NewState(now),
				},
			},
		},
	}
	r.state.SurfaceSubs = map[state.ConnID]map[state.SurfaceSubscription]struct{}{
		conn1: {{SessionID: sess1}: {}},
	}
	if err := r.terminalRelay.Subscribe(conn1, sess1, "%root"); err != nil {
		t.Fatalf("subscribe root head: %v", err)
	}

	next, _ := state.Reduce(r.state, state.EvEvent{
		Event: state.EventPushDriver,
		Payload: []byte(
			`{"session_id":"sess-1","project":"/tmp/project","command":"shell"}`,
		),
	})
	childID := next.Sessions[sess1].HeadFrameID
	if childID == "" || childID == "%root" {
		t.Fatalf("head after push = %q, want new child frame", childID)
	}
	r.state = next
	r.reconcileSurfaceRelay()
	assertRelayFrame(t, r.terminalRelay, conn1, sess1, string(childID))

	next, _ = state.Reduce(r.state, state.EvFrameVanished{FrameID: childID})
	r.state = next
	r.reconcileSurfaceRelay()
	assertRelayFrame(t, r.terminalRelay, conn1, sess1, "%root")
}

func assertRelayFrame(
	t *testing.T,
	relay *TerminalRelay,
	connID state.ConnID,
	sessionID state.SessionID,
	want string,
) {
	t.Helper()
	key := surfaceKey{connID: connID, sessionID: sessionID}
	relay.mu.Lock()
	sub := relay.subs[key]
	relay.mu.Unlock()
	if sub == nil || sub.frameID != want {
		t.Fatalf("terminal relay subscription = %+v, want frame %q", sub, want)
	}
}
