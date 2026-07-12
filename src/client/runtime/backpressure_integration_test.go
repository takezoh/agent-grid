package runtime

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/termvt"
)

// compositeSurfaceBackend wires the generic frame fake with a surface fake so
// integration tests can drive terminal relay severance through real IPC.
type compositeSurfaceBackend struct {
	*fakeBackend
	surface *fakeSurfaceBackend
}

func (c *compositeSurfaceBackend) SubscribeSurface(frameID string) (int, <-chan termvt.Event, error) {
	return c.surface.SubscribeSurface(frameID)
}

func (c *compositeSurfaceBackend) SubscribeSurfaceWithBuffer(frameID string, buffer int) (int, <-chan termvt.Event, error) {
	return c.surface.SubscribeSurfaceWithBuffer(frameID, buffer)
}

func (c *compositeSurfaceBackend) UnsubscribeSurface(frameID string, id int) error {
	return c.surface.UnsubscribeSurface(frameID, id)
}

func (c *compositeSurfaceBackend) WriteSurface(frameID string, data []byte) error {
	return c.surface.WriteSurface(frameID, data)
}

func (c *compositeSurfaceBackend) ResizeSurface(frameID string, cols, rows int) error {
	return c.surface.ResizeSurface(frameID, cols, rows)
}

func startBackpressureRuntime(t *testing.T, ctx context.Context) (*Runtime, *fakeSurfaceBackend, string) {
	t.Helper()

	surface := newFakeSurfaceBackend()
	backend := &compositeSurfaceBackend{
		fakeBackend: newFakeBackend(),
		surface:     surface,
	}

	dir := t.TempDir()
	sock := filepath.Join(dir, "backpressure.sock")

	r := New(Config{
		DataDir:      dir,
		TickInterval: time.Hour,
		Backend:      backend,
	})
	r.state.Sessions = map[state.SessionID]state.Session{
		sess1: {
			ID:          sess1,
			CreatedAt:   time.Now(),
			HeadFrameID: "%1",
			Frames: []state.SessionFrame{
				{ID: "%1", CreatedAt: time.Now()},
			},
		},
	}
	r.terminalRelay = NewTerminalRelay(
		surface,
		r.enqueueInternal,
		r.sendInternalNow,
		WithTerminalRelaySubscriberBuffer(1),
		WithSeveranceThreshold(1),
	)

	go func() {
		_ = r.Run(ctx)
	}()
	if err := r.StartIPC(sock); err != nil {
		t.Fatalf("StartIPC: %v", err)
	}
	t.Cleanup(func() {
		<-r.Done()
	})
	return r, surface, sock
}

// TestBackpressureSeverance_EndToEnd pins burst → severance push → resubscribe →
// sequence reset across runtime IPC and the proto.Client push channel (m6).
func TestBackpressureSeverance_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, surface, sock := startBackpressureRuntime(t, ctx)

	c := dialClient(t, sock)
	defer c.Close()

	sendCtx, sendCancel := context.WithTimeout(ctx, 2*time.Second)
	if _, err := c.Send(sendCtx, proto.CmdSubscribe{
		Filters: []string{proto.EvtNameSessionsChanged, proto.EvtNameSurfaceOutput},
	}); err != nil {
		t.Fatalf("CmdSubscribe: %v", err)
	}
	sendCancel()

	drainOneEvent(t, c.Events())

	sendCtx, sendCancel = context.WithTimeout(ctx, 2*time.Second)
	if _, err := c.Send(sendCtx, proto.CmdSurfaceSubscribe{SessionID: string(sess1)}); err != nil {
		t.Fatalf("CmdSurfaceSubscribe: %v", err)
	}
	sendCancel()

	// Do not drain surface-output events: flood the relay until severance fires.
	for i := 0; i < 8; i++ {
		surface.Broadcast("%1", termvt.Event{
			Kind: termvt.EventOutput,
			Data: []byte{byte('a' + i)},
		})
	}

	var push proto.PushNotification
	select {
	case push = <-c.Pushes():
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for severance push")
	}
	if push.Cmd != proto.CmdNameSurfaceUnsubscribe {
		t.Fatalf("push cmd = %q, want %q", push.Cmd, proto.CmdNameSurfaceUnsubscribe)
	}
	body, ok := push.Body.(proto.RespSurfaceUnsubscribed)
	if !ok || body.SessionID != string(sess1) {
		t.Fatalf("push body = %#v, want RespSurfaceUnsubscribed{sess-1}", push.Body)
	}

	sendCtx, sendCancel = context.WithTimeout(ctx, 2*time.Second)
	if _, err := c.Send(sendCtx, proto.CmdSurfaceSubscribe{SessionID: string(sess1)}); err != nil {
		t.Fatalf("re-subscribe: %v", err)
	}
	sendCancel()

	surface.Broadcast("%1", termvt.Event{
		Kind: termvt.EventOutput,
		Data: []byte("resync"),
	})

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-c.Events():
			out, ok := ev.(proto.EvtSurfaceOutput)
			if !ok || out.SessionID != string(sess1) {
				continue
			}
			if out.Sequence != 0 {
				t.Fatalf("resync sequence = %d, want 0", out.Sequence)
			}
			decoded, err := base64.StdEncoding.DecodeString(out.DataB64)
			if err != nil {
				t.Fatalf("decode resync payload: %v", err)
			}
			if string(decoded) != "resync" {
				t.Fatalf("resync data = %q, want %q", decoded, "resync")
			}
			return
		case <-deadline:
			t.Fatal("timeout waiting for resync surface-output with sequence 0")
		}
	}
}

func drainOneEvent(t *testing.T, ch <-chan proto.ServerEvent) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout draining one event")
	}
}