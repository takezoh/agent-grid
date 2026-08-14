package fleet

import (
	"context"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-protocol-admission, contract-discovery-projection,
// contract-config-entry-admission (unit-fleet-reachability).

func server(index int, name, url string) clientconfig.Server {
	return clientconfig.Server{
		Index: index, ID: "srv_" + name, DisplayName: name, Enabled: true, URL: url,
		Auth: clientconfig.Auth{Mode: "bearer", Token: "t"},
	}
}

func session(id, title string) gateway.SessionInfo {
	return gateway.SessionInfo{
		ID: id, Project: "/p", Command: "claude",
		Status: gateway.SessionStatus{Lifecycle: "active", Failure: "none"},
		View:   gateway.View{Card: gateway.Card{Title: title}},
	}
}

// TestOneMismatchedServerDoesNotDisableOthers is ver-mismatch-does-not-disable-peers.
//
// The counterexample is an implementation that treats the fleet as one
// operation: the first failure aborts the whole probe and `ls` prints nothing,
// so one server upgraded out of step takes away every other server the operator
// has. Both a protocol mismatch and a plain outage are covered, because an
// implementation can easily special-case one and not the other.
func TestOneMismatchedServerDoesNotDisableOthers(t *testing.T) {
	t.Parallel()

	configured := clientconfig.Snapshot{Servers: []clientconfig.Server{
		server(0, "old", "https://old.test"),
		server(1, "good", "https://good.test"),
		server(2, "down", "https://down.test"),
	}}
	probe := func(_ context.Context, target clientconfig.Server) (Observation, []gateway.SessionInfo) {
		switch target.DisplayName {
		case "old":
			return Observation{
				Kind: KindIncompatibleProtocol, Reason: ReasonProtocolMismatch,
				Detail: "client protocol-3, server protocol-4",
			}, nil
		case "down":
			return Observation{Kind: KindUnavailable, Reason: ReasonConnectionTimeout}, nil
		default:
			return Observation{Kind: KindConnected}, []gateway.SessionInfo{session("s1", "T1")}
		}
	}

	snapshot := Observe(context.Background(), configured, probe)

	if len(snapshot.Members) != 3 {
		t.Fatalf("%d members, want one per configured entry", len(snapshot.Members))
	}
	healthy := snapshot.Usable()
	if len(healthy) != 1 || healthy[0].DisplayName != "good" {
		t.Fatalf("the healthy server did not survive its peers' failures: %+v", healthy)
	}
	if len(healthy[0].Sessions) != 1 {
		t.Errorf("the healthy server's sessions were lost: %+v", healthy[0].Sessions)
	}
	if !snapshot.HasUnreachable() {
		t.Error("a fleet with two failed servers must report that it could not look everywhere")
	}
	// Order is configuration order, so a row can be pointed at by position.
	for index, expected := range []string{"old", "good", "down"} {
		if snapshot.Members[index].DisplayName != expected {
			t.Errorf("member %d is %q, want %q", index, snapshot.Members[index].DisplayName, expected)
		}
	}
}

// TestConfigurationInvalidEntryIsNeverProbed is ver-invalid-entry-is-never-connected-to.
//
// The rule has teeth only if it is impossible rather than merely avoided: an
// entry that gets probed acquires a reachability observation, and from then on
// it is presented as unreachable — which tells the operator to check the
// network for a server whose problem is a typo in their own configuration file.
// So the prober's body fails the test if it is ever reached for such an entry.
func TestConfigurationInvalidEntryIsNeverProbed(t *testing.T) {
	t.Parallel()

	invalid := server(0, "-leading-dash", "https://invalid.test")
	invalid.Violation = &clientconfig.Violation{
		Rule: clientconfig.RuleDisplayNameCharset, Detail: "display name begins with -",
	}
	disabled := server(1, "off", "https://off.test")
	disabled.Enabled = false

	configured := clientconfig.Snapshot{Servers: []clientconfig.Server{
		invalid, disabled, server(2, "good", "https://good.test"),
	}}
	probe := func(_ context.Context, target clientconfig.Server) (Observation, []gateway.SessionInfo) {
		if target.DisplayName != "good" {
			t.Errorf("%q was probed; only usable entries may be contacted", target.DisplayName)
		}
		return Observation{Kind: KindConnected}, nil
	}

	snapshot := Observe(context.Background(), configured, probe)

	first := snapshot.Members[0].Observation
	if first.Kind != KindConfigurationInvalid {
		t.Errorf("an invalid entry observed as %q, want %q", first.Kind, KindConfigurationInvalid)
	}
	if first.Rule != clientconfig.RuleDisplayNameCharset {
		t.Errorf("the violated rule was not carried through: %q", first.Rule)
	}
	if first.Reason != "" {
		t.Errorf("an entry that was never contacted carries a reachability reason %q", first.Reason)
	}
	if snapshot.Members[0].DisplayName != "-leading-dash" || snapshot.Members[0].Index != 0 {
		t.Error("an invalid entry must keep its position and its display name as written")
	}
	if snapshot.Members[1].Observation.Kind != KindDisabled {
		t.Errorf("a disabled entry observed as %q", snapshot.Members[1].Observation.Kind)
	}
	if got := snapshot.Invalid(); len(got) != 1 {
		t.Errorf("%d invalid members reported, want 1", len(got))
	}
	// An unusable entry must never be able to look unreachable, because a
	// reachability observation is the thing it can never acquire.
	if snapshot.HasUnreachable() {
		t.Error("an unusable entry was counted as unreachable")
	}
}

// TestVocabulariesDoNotOverlap is the value-space separation FR-016 and FR-031
// require: reachability, configuration invalidity and session lifecycle state
// are three vocabularies, and no word appears in two of them.
func TestVocabulariesDoNotOverlap(t *testing.T) {
	t.Parallel()

	reachability := []string{KindConnected, KindUnavailable, KindIncompatibleProtocol}
	admission := []string{KindDisabled, KindConfigurationInvalid}
	lifecycle := []string{"active", "idle", "stopped"}

	for _, group := range [][2][]string{
		{reachability, admission},
		{reachability, lifecycle},
		{admission, lifecycle},
	} {
		for _, left := range group[0] {
			for _, right := range group[1] {
				if left == right {
					t.Errorf("%q appears in two vocabularies", left)
				}
			}
		}
	}
}

// TestEveryFailureCarriesAReasonFromTheClosedSet stops a free-form sentence
// reaching a field the contract declares closed.
func TestEveryFailureCarriesAReasonFromTheClosedSet(t *testing.T) {
	t.Parallel()

	closed := map[string]bool{
		ReasonConnectionTimeout: true, ReasonConnectionFailed: true, ReasonHTTPError: true,
		ReasonWebSocketFailed: true, ReasonEmptyTicket: true, ReasonProtocolMismatch: true,
	}
	for name, err := range map[string]error{
		"mismatch": gateway.FromCloseReason(gateway.ReasonIncompatibleProtocol, "client protocol-3, server protocol-4"),
		"rejected": gateway.FromCloseReason(gateway.ReasonUnauthenticated, "bad ticket"),
		"capacity": gateway.FromCloseReason(gateway.ReasonCapacity, "budget exhausted"),
		"backend":  gateway.FromCloseReason(gateway.ReasonBackendUnavailable, "dial tcp: i/o timeout"),
		"opaque":   context.DeadlineExceeded,
	} {
		observation := observationFor(err)
		if observation.Kind != KindUnavailable && observation.Kind != KindIncompatibleProtocol {
			t.Errorf("%s: kind %q is not a reachability observation", name, observation.Kind)
		}
		if !closed[observation.Reason] {
			t.Errorf("%s: reason %q is outside the closed reason set", name, observation.Reason)
		}
	}
}
