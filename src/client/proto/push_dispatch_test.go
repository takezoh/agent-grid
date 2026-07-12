package proto

import (
	"bufio"
	"net"
	"testing"
	"time"
)

// TestClientDispatchPushRoutesEmptyReqID verifies server-initiated responses are
// delivered on Pushes() instead of being silently discarded (m4 contract).
func TestClientDispatchPushRoutesEmptyReqID(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	c := DialConn(client)

	wire, err := EncodePushResponse(CmdNameSurfaceUnsubscribe, RespSurfaceUnsubscribed{
		SessionID:    "sess-1",
		SubscriberID: "web-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		w := bufio.NewWriter(server)
		_, _ = w.Write(append(wire, '\n'))
		_ = w.Flush()
	}()

	select {
	case push := <-c.Pushes():
		if push.Cmd != CmdNameSurfaceUnsubscribe {
			t.Fatalf("cmd = %q", push.Cmd)
		}
		body, ok := push.Body.(RespSurfaceUnsubscribed)
		if !ok || body.SessionID != "sess-1" || body.SubscriberID != "web-1" {
			t.Fatalf("body = %#v", push.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for push")
	}

	select {
	case <-c.Events():
		t.Fatal("push must not appear on Events()")
	case <-time.After(50 * time.Millisecond):
	}

	<-done
	_ = c.Close()
}
