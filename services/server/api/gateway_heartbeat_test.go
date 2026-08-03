package api

import (
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/proto"
)

func TestLifecycleWSSendsApplicationHeartbeatAfterHello(t *testing.T) {
	fake := newFakeLifecycleAttacher()
	server := startLifecycleServer(t, fake)
	connection := dialLifecycleWS(t, server)

	fake.events <- proto.EvtSessionsChanged{}
	hello := readJSONFrame(t, connection)
	if hello["k"] != "h" {
		t.Fatalf("first frame k = %v, want h", hello["k"])
	}

	// Browser WebSocket does not expose client ping, so a dedicated server
	// application frame is the host-neutral liveness evidence. Its cadence must
	// remain below the hosts' five-second Connected freshness window.
	ctx, cancel := context.WithTimeout(context.Background(), 4900*time.Millisecond)
	defer cancel()
	_, data, err := connection.Read(ctx)
	if err != nil {
		t.Fatalf("read heartbeat before freshness expiry: %v", err)
	}
	if string(data) != `{"k":"p"}` {
		t.Fatalf("heartbeat frame = %s, want dedicated k:p frame", data)
	}
}
