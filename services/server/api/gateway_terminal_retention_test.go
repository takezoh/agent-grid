package api

import (
	"testing"

	"github.com/takezoh/agent-grid/services/host/proto"
	stateview "github.com/takezoh/agent-grid/services/host/state/view"
)

// Contract: contract-session-terminal-state-retention
// (adr-20260814-session-terminal-state-retention).
//
// The execution plane now retains an ended session with a terminal lifecycle
// state. These two tests pin the gateway half of that contract: the transition
// reaches a lifecycle subscriber, and the retained row makes "ended"
// distinguishable from "never existed" through the public contract alone.

// TestLifecycleSocketPublishesTerminalTransition is
// ver-terminal-transition-is-published-on-lifecycle.
//
// An attached client must be able to attribute its departure to the session
// ending rather than to a lost connection. That is only possible if the
// terminal transition arrives as an ordinary lifecycle update — not as silence
// followed by a row disappearing.
func TestLifecycleSocketPublishesTerminalTransition(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// The session is live when the client attaches.
	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
	}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("first frame k = %q, want \"h\"", hello["k"])
	}

	// It then ends. The row is retained with a terminal lifecycle state.
	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusStopped)},
	}

	update := readJSONFrame(t, c)
	if update["k"] != "v" {
		t.Fatalf("terminal transition arrived as k = %q, want \"v\"", update["k"])
	}
	if got := lifecycleOf(t, update, "s1"); got != string(stateview.LifecycleStopped) {
		t.Errorf("published lifecycle = %q, want %q", got, stateview.LifecycleStopped)
	}
}

// TestEndedSessionIsDistinguishableFromUnknownIdentifier is
// ver-ended-and-never-existed-are-distinguishable.
//
// The counterexample this rejects: a gateway that drops ended rows would make
// both cases look identical (absent), forcing a client to keep private memory
// of what it once saw in order to tell them apart. The contract forbids that
// compensation, so absence must mean exactly one thing.
func TestEndedSessionIsDistinguishableFromUnknownIdentifier(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("ended", "T", stateview.StatusStopped)},
	}

	hello := readJSONFrame(t, c)
	if got := lifecycleOf(t, hello, "ended"); got != string(stateview.LifecycleStopped) {
		t.Errorf("ended session lifecycle = %q, want %q", got, stateview.LifecycleStopped)
	}
	if sessionEntry(t, hello, "never-existed") != nil {
		t.Error("an identifier that never existed must not appear in the session list")
	}
}

// lifecycleOf reads sessions[id].status.lifecycle out of a lifecycle frame.
func lifecycleOf(t *testing.T, frame map[string]any, id string) string {
	t.Helper()
	entry := sessionEntry(t, frame, id)
	if entry == nil {
		t.Fatalf("session %q missing from frame: %v", id, frame["sessions"])
	}
	status, ok := entry["status"].(map[string]any)
	if !ok {
		t.Fatalf("session %q has no status object: %T", id, entry["status"])
	}
	lifecycle, ok := status["lifecycle"].(string)
	if !ok {
		t.Fatalf("session %q status.lifecycle is not a string: %T", id, status["lifecycle"])
	}
	return lifecycle
}

// sessionEntry returns the session object with the given id, or nil when the
// frame does not carry it. Absence is a meaningful observation here, so this
// deliberately does not fail the test.
func sessionEntry(t *testing.T, frame map[string]any, id string) map[string]any {
	t.Helper()
	sessions, ok := frame["sessions"].([]any)
	if !ok {
		t.Fatalf("frame carries no sessions array: %T", frame["sessions"])
	}
	for _, raw := range sessions {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["id"] == id {
			return entry
		}
	}
	return nil
}
