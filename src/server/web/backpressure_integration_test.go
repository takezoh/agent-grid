package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
)

// TestBackpressureSeverance_PushFansOutToGatewaySubscriber pins daemon-initiated
// severance notifications through DaemonClient into pre-encoded WS push frames (m6).
func TestBackpressureSeverance_PushFansOutToGatewaySubscriber(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return proto.DialConn(clientConn), nil
	}, testMinDelay, testMaxDelay)
	defer d.Close()

	if !waitHealth(d, true, time.Second) {
		t.Fatal("daemon not healthy")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := d.SubscribeEvents(ctx)
	pushes := d.PushChannelFor(events)

	wire, err := proto.EncodePushResponse(proto.CmdNameSurfaceUnsubscribe, proto.RespSurfaceUnsubscribed{
		SessionID: "sess-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		w := bufio.NewWriter(serverConn)
		if _, err := w.Write(append(wire, '\n')); err != nil {
			return
		}
		_ = w.Flush()
	}()

	select {
	case frame := <-pushes:
		var control map[string]any
		if err := json.Unmarshal(frame, &control); err != nil {
			t.Fatalf("unmarshal push frame: %v", err)
		}
		if control["k"] != "c" {
			t.Fatalf("k = %v, want c", control["k"])
		}
		if control["data"] != surfaceUnsubscribedControlData {
			t.Fatalf("data = %v, want %q", control["data"], surfaceUnsubscribedControlData)
		}
		if control["sessionId"] != "sess-1" {
			t.Fatalf("sessionId = %v, want sess-1", control["sessionId"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for severance push frame")
	}

	// Severance push must not close the paired events channel.
	select {
	case <-events:
		t.Fatal("unexpected event after push-only frame")
	case <-time.After(50 * time.Millisecond):
	}

	if err := serverConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("close server conn: %v", err)
	}
}