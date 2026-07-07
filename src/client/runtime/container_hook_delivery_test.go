package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/event"
	"github.com/takezoh/agent-grid/client/runtime/framereg"
	"github.com/takezoh/agent-grid/client/state"
)

// These tests pin the container hook-delivery contract across the per-frame
// registration window. A containerized agent can launch and emit its first
// hooks (e.g. SessionStart) before the daemon has started the endpoint /
// registered this frame's token, because registration now happens on the event
// loop after the agent process is spawned. The hook must still be delivered —
// losing SessionStart silently disables transcript watching, so summary/tags
// never appear for container frames.

// TestContainerHook_deliveredWhenEndpointStartsLate covers the first frame of a
// project: the listener is not up yet when the agent sends its first hook.
func TestContainerHook_deliveredWhenEndpointStartsLate(t *testing.T) {
	dir := t.TempDir()
	sock := ContainerSockPath(dir)
	reg := framereg.New()
	fid := state.FrameID("f1")
	tok := "tok-late-endpoint"
	reg.RegisterWithMounts(fid, tok, nil)

	evCh := make(chan state.Event, 1)
	epCh := make(chan *containerEndpoint, 1)
	go func() {
		time.Sleep(120 * time.Millisecond)
		ep, err := startContainerEndpoint(sock, reg, func(ev state.Event) { evCh <- ev }, nil)
		if err != nil {
			epCh <- nil
			return
		}
		epCh <- ep
	}()

	if err := event.DeliverHookEvent(sock, tok, "SessionStart", time.Now(), nil); err != nil {
		t.Fatalf("DeliverHookEvent: %v", err)
	}
	assertDriverEvent(t, evCh, fid)
	if ep := <-epCh; ep != nil {
		ep.close()
	}
}

// TestContainerHook_deliveredWhenTokenRegisteredLate covers a subsequent frame
// of a project whose endpoint is already listening: the dial succeeds but the
// token resolves only after the loop runs registerContainerFrame.
func TestContainerHook_deliveredWhenTokenRegisteredLate(t *testing.T) {
	dir := t.TempDir()
	sock := ContainerSockPath(dir)
	reg := framereg.New()

	evCh := make(chan state.Event, 1)
	ep, err := startContainerEndpoint(sock, reg, func(ev state.Event) { evCh <- ev }, nil)
	if err != nil {
		t.Fatalf("startContainerEndpoint: %v", err)
	}
	t.Cleanup(ep.close)

	fid := state.FrameID("f2")
	tok := "tok-late-register"
	go func() {
		time.Sleep(120 * time.Millisecond)
		reg.RegisterWithMounts(fid, tok, nil)
	}()

	if err := event.DeliverHookEvent(sock, tok, "Stop", time.Now(), nil); err != nil {
		t.Fatalf("DeliverHookEvent: %v", err)
	}
	assertDriverEvent(t, evCh, fid)
}

// TestContainerHook_deliveredImmediately is the steady-state path: endpoint up
// and token registered, so delivery succeeds on the first attempt with no wait.
func TestContainerHook_deliveredImmediately(t *testing.T) {
	dir := t.TempDir()
	sock := ContainerSockPath(dir)
	reg := framereg.New()
	fid := state.FrameID("f3")
	tok := "tok-ready"
	reg.RegisterWithMounts(fid, tok, nil)

	evCh := make(chan state.Event, 1)
	ep, err := startContainerEndpoint(sock, reg, func(ev state.Event) { evCh <- ev }, nil)
	if err != nil {
		t.Fatalf("startContainerEndpoint: %v", err)
	}
	t.Cleanup(ep.close)

	start := time.Now()
	if err := event.DeliverHookEvent(sock, tok, "Stop", time.Now(), nil); err != nil {
		t.Fatalf("DeliverHookEvent: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("steady-state delivery took %v, expected immediate", elapsed)
	}
	assertDriverEvent(t, evCh, fid)
}

func assertDriverEvent(t *testing.T, evCh <-chan state.Event, want state.FrameID) {
	t.Helper()
	select {
	case ev := <-evCh:
		de, ok := ev.(state.EvDriverEvent)
		if !ok {
			t.Fatalf("expected EvDriverEvent, got %T", ev)
		}
		if de.SenderID != want {
			t.Fatalf("SenderID = %q, want %q", de.SenderID, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hook not delivered (no EvDriverEvent enqueued)")
	}
}
