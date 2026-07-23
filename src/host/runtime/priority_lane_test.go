package runtime

import (
	"net"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

func TestPriorityLane_BulkDoesNotStarveUnderSurfaceBurst(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	cc := newIPCConn(state.ConnID(1), nil)

	const n = ipcOutboxLaneSize
	for i := 0; i < n; i++ {
		r.queueWireLane(cc, []byte("surface"), true, subscriptionKey(1, "s1", ""))
	}
	for i := 0; i < n; i++ {
		r.queueWireLane(cc, []byte("bulk"), false, SubscriptionKey{})
	}

	drainedBulk := 0
	deadline := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case <-cc.outboxBulk:
			drainedBulk++
			if drainedBulk == n {
				break loop
			}
		case <-cc.outboxInteractive:
		case <-deadline:
			t.Fatalf("bulk lane starved: drained %d/%d", drainedBulk, n)
		}
	}
}

func TestPriorityLane_InteractiveDoesNotStarveUnderBulkBurst(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	cc := newIPCConn(state.ConnID(1), nil)

	const n = ipcOutboxLaneSize
	for i := 0; i < n; i++ {
		r.queueWireLane(cc, []byte("bulk"), false, SubscriptionKey{})
	}
	for i := 0; i < n; i++ {
		r.queueWireLane(cc, []byte("surface"), true, subscriptionKey(1, "s1", ""))
	}

	drainedInteractive := 0
	deadline := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case <-cc.outboxInteractive:
			drainedInteractive++
			if drainedInteractive == n {
				break loop
			}
		case <-cc.outboxBulk:
		case <-deadline:
			t.Fatalf("interactive lane starved: drained %d/%d", drainedInteractive, n)
		}
	}
}

func TestPriorityLane_ConnWriterPrefersInteractive(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	cc := newIPCConn(state.ConnID(1), clientConn)
	go r.connWriter(cc)

	cc.outboxBulk <- []byte("bulk")
	cc.outboxInteractive <- []byte("interactive")

	buf := make([]byte, 64)
	if _, err := serverConn.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if buf[0] != 'i' {
		t.Fatalf("first write starts with %q, want interactive", buf[0])
	}
}

func TestIsInteractiveWireEvent(t *testing.T) {
	if !isInteractiveWireEvent(proto.EvtNameSurfaceOutput) {
		t.Fatal("surface-output must be interactive")
	}
	if isInteractiveWireEvent(proto.EvtNameSessionsChanged) {
		t.Fatal("sessions-changed must be bulk")
	}
}
