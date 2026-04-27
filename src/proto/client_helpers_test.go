package proto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/state"
)

// TestCreateSessionUsesLongTimeout verifies that CreateSession waits beyond
// defaultRequestTimeout (5s) when the server is slow to respond.
func TestCreateSessionUsesLongTimeout(t *testing.T) {
	c, server := newFakeServer(t)
	defer c.Close()

	type result struct {
		id  string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		id, err := c.CreateSession("/tmp/project", "shell", state.LaunchOptions{})
		resCh <- result{id, err}
	}()

	// Simulate a slow daemon: delay longer than defaultRequestTimeout (5s)
	// but shorter than createSessionTimeout (5min).
	env := server.recv()
	time.Sleep(6 * time.Second)
	wire, _ := EncodeResponse(env.ReqID, RespCreateSession{SessionID: "slow-sess"})
	server.send(wire)

	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("CreateSession: unexpected error: %v", r.err)
		}
		if r.id != "slow-sess" {
			t.Errorf("session id = %q, want slow-sess", r.id)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("CreateSession did not complete in time")
	}
}

// TestOtherRPCsUseShortTimeout verifies that non-CreateSession RPCs still use
// the default 5s timeout (confirmed by context deadline when server is silent).
func TestOtherRPCsUseShortTimeout(t *testing.T) {
	c, server := newFakeServer(t)
	defer c.Close()

	// server reads the request but intentionally never responds.
	go func() { server.recv() }()

	start := time.Now()
	_, err := sendJSONEvent[RespOK](c, state.EventStopSession, json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Should expire around defaultRequestTimeout (5s), not 5 minutes.
	if elapsed > 10*time.Second {
		t.Errorf("timeout took %v, want ~5s", elapsed)
	}
}
