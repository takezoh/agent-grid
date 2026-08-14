package selector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-selector-resolution, contract-server-naming-prerequisite
// (unit-selector).

func entry(index int, name, id string) clientconfig.Server {
	return clientconfig.Server{
		Index: index, ID: id, DisplayName: name, Enabled: true,
		URL: "https://" + name + ".test", Auth: clientconfig.Auth{Mode: "none"},
	}
}

func member(server clientconfig.Server, observation fleet.Observation, ids ...string) fleet.Member {
	sessions := make([]gateway.SessionInfo, 0, len(ids))
	for _, id := range ids {
		sessions = append(sessions, gateway.SessionInfo{
			ID: id, Project: "/repo",
			Status: gateway.SessionStatus{Lifecycle: "active", Failure: "none"},
			View:   gateway.View{Card: gateway.Card{Title: "T-" + id}},
		})
	}
	return fleet.Member{
		Index: server.Index, ServerID: server.ID, DisplayName: server.DisplayName,
		Server: server, Observation: observation, Sessions: sessions,
	}
}

func connected(server clientconfig.Server, ids ...string) fleet.Member {
	return member(server, fleet.Observation{Kind: fleet.KindConnected}, ids...)
}

// TestDeclaredKindDecidesResolution is ver-declared-kind-decides-resolution.
//
// The setup is the one that makes fallback indefensible: "abc" is one session's
// full identifier and simultaneously a prefix of another's. Under the id kind
// it names exactly one session; under the prefix kind it names two. A resolver
// that fell back from one kind to the other would give the same string two
// different meanings depending on what else happens to exist.
func TestDeclaredKindDecidesResolution(t *testing.T) {
	t.Parallel()

	snapshot := fleet.Snapshot{Members: []fleet.Member{
		connected(entry(0, "one", "srv_one"), "abc", "abcdef"),
	}}

	byID := Resolve(snapshot, Query{Kind: KindID, Value: "abc"})
	if byID.Kind != Resolved || byID.Ref.SessionID() != "abc" {
		t.Fatalf("id kind resolved to %+v", byID)
	}

	byPrefix := Resolve(snapshot, Query{Kind: KindPrefix, Value: "abc"})
	if byPrefix.Kind != Ambiguous {
		t.Fatalf("prefix kind resolved to %v, want Ambiguous", byPrefix.Kind)
	}
	if len(byPrefix.Candidates) != 2 {
		t.Fatalf("%d candidates, want both matches", len(byPrefix.Candidates))
	}
	// Every candidate line must name a value that uniquely specifies the next
	// invocation's target; two identical lines would not.
	if byPrefix.Candidates[0].Ref.SessionID() == byPrefix.Candidates[1].Ref.SessionID() {
		t.Error("the candidates are not mutually distinguishable")
	}

	// A full identifier that matches nothing under its own kind is not-found,
	// not a prefix search.
	missing := Resolve(snapshot, Query{Kind: KindID, Value: "abcd"})
	if missing.Kind != NotFound {
		t.Errorf("an id that matches no session resolved to %v", missing.Kind)
	}
}

// TestServerQualifierNarrowsCollisionByNameAndByStableId is
// ver-qualifier-narrows-cross-server-collision.
//
// Both vocabularies must work. The display name is what the listing shows and
// what a person types; the stable identifier is what the machine-readable
// output emits and what a script feeds back. Accepting only one of them makes
// one of those two workflows impossible.
func TestServerQualifierNarrowsCollisionByNameAndByStableId(t *testing.T) {
	t.Parallel()

	alpha := entry(0, "alpha", "srv_alpha")
	beta := entry(1, "beta", "srv_beta")
	snapshot := fleet.Snapshot{Members: []fleet.Member{
		connected(alpha, "shared"),
		connected(beta, "shared"),
	}}

	unqualified := Resolve(snapshot, Query{Kind: KindID, Value: "shared"})
	if unqualified.Kind != Ambiguous {
		t.Fatalf("the same identifier on two servers resolved to %v", unqualified.Kind)
	}

	for _, qualifier := range []string{"beta", "srv_beta"} {
		outcome := Resolve(snapshot, Query{Kind: KindID, Value: "shared", Server: qualifier})
		if outcome.Kind != Resolved {
			t.Fatalf("qualifier %q resolved to %v", qualifier, outcome.Kind)
		}
		if outcome.Ref.ServerID() != "srv_beta" {
			t.Errorf("qualifier %q selected server %q", qualifier, outcome.Ref.ServerID())
		}
	}
}

// TestUnreachableOnlyMatchIsTransportNotNotFound is
// ver-unreachable-only-match-is-transport.
//
// Reporting not-found here is a claim the observation cannot support: the
// session may exist on the server that could not be read. Automation that
// branches on not-found by creating a replacement would then start a second
// session every time the network hiccuped.
func TestUnreachableOnlyMatchIsTransportNotNotFound(t *testing.T) {
	t.Parallel()

	down := member(entry(1, "down", "srv_down"),
		fleet.Observation{Kind: fleet.KindUnavailable, Reason: fleet.ReasonConnectionTimeout})
	snapshot := fleet.Snapshot{Members: []fleet.Member{
		connected(entry(0, "up", "srv_up"), "other"),
		down,
	}}

	outcome := Resolve(snapshot, Query{Kind: KindID, Value: "missing"})
	if outcome.Kind != Unreachable {
		t.Errorf("resolved to %v, want Unreachable", outcome.Kind)
	}

	// With every scoped server readable, the same absence is a fact.
	readable := fleet.Snapshot{Members: []fleet.Member{connected(entry(0, "up", "srv_up"), "other")}}
	if got := Resolve(readable, Query{Kind: KindID, Value: "missing"}); got.Kind != NotFound {
		t.Errorf("with every server readable, a missing session resolved to %v", got.Kind)
	}

	// Qualifying to a healthy server excludes the unreachable one from the
	// scope, so the absence becomes a fact again.
	qualified := Resolve(snapshot, Query{Kind: KindID, Value: "missing", Server: "up"})
	if qualified.Kind != NotFound {
		t.Errorf("scoped to a readable server, a missing session resolved to %v", qualified.Kind)
	}
}

// TestUnusableMatchesExitUsageAndAmbiguousOnlyAppliesToUsableOnes is
// ver-qualifier-precedence-unusable-outranks-ambiguous.
//
// Three distinct outcomes have to stay distinct. Picking the first match would
// silently send the command to a different machine than the operator meant;
// offering unusable entries as candidates would ask them to choose between
// servers none of which can be connected to; and calling either case not-found
// would hide a configuration file they can fix.
func TestUnusableMatchesExitUsageAndAmbiguousOnlyAppliesToUsableOnes(t *testing.T) {
	t.Parallel()

	firstInvalid := entry(0, "twin", "srv_a")
	firstInvalid.Violation = &clientconfig.Violation{
		Rule: clientconfig.RuleDisplayNameUnique, Detail: "display name is not unique",
	}
	secondInvalid := entry(1, "twin", "srv_b")
	secondInvalid.Violation = &clientconfig.Violation{
		Rule: clientconfig.RuleDisplayNameUnique, Detail: "display name is not unique",
	}
	allInvalid := fleet.Snapshot{Members: []fleet.Member{
		member(firstInvalid, fleet.Observation{
			Kind: fleet.KindConfigurationInvalid, Rule: clientconfig.RuleDisplayNameUnique}),
		member(secondInvalid, fleet.Observation{
			Kind: fleet.KindConfigurationInvalid, Rule: clientconfig.RuleDisplayNameUnique}),
	}}

	unusable := Resolve(allInvalid, Query{Kind: KindID, Value: "s1", Server: "twin"})
	if unusable.Kind != QualifierUnusable {
		t.Fatalf("a qualifier matching only unusable entries resolved to %v", unusable.Kind)
	}
	if len(unusable.Entries) != 2 {
		t.Fatalf("%d entries reported, want both matches named", len(unusable.Entries))
	}
	for _, reported := range unusable.Entries {
		if reported.Observation.Rule == "" {
			t.Errorf("entry %d was named without the rule it violates", reported.Index)
		}
	}

	// Two usable matches are an ambiguity, and nothing is selected.
	twoUsable := fleet.Snapshot{Members: []fleet.Member{
		connected(entry(0, "twin", "srv_a"), "s1"),
		connected(entry(1, "twin", "srv_b"), "s1"),
	}}
	ambiguous := Resolve(twoUsable, Query{Kind: KindID, Value: "s1", Server: "twin"})
	if ambiguous.Kind != QualifierAmbiguous {
		t.Fatalf("two usable matches resolved to %v", ambiguous.Kind)
	}
	if !ambiguous.Ref.IsZero() {
		t.Error("an ambiguous qualifier selected a server anyway")
	}

	// One usable beside one unusable is not ambiguous: there is exactly one
	// server the command could be sent to.
	mixed := fleet.Snapshot{Members: []fleet.Member{
		member(firstInvalid, fleet.Observation{
			Kind: fleet.KindConfigurationInvalid, Rule: clientconfig.RuleDisplayNameUnique}),
		connected(entry(1, "twin", "srv_b"), "s1"),
	}}
	if got := Resolve(mixed, Query{Kind: KindID, Value: "s1", Server: "twin"}); got.Kind != Resolved {
		t.Errorf("one usable match beside an unusable one resolved to %v", got.Kind)
	}

	// A qualifier naming nothing is its own outcome, distinct from both.
	unknown := Resolve(twoUsable, Query{Kind: KindID, Value: "s1", Server: "nowhere"})
	if unknown.Kind != QualifierUnknown {
		t.Errorf("an unmatched qualifier resolved to %v", unknown.Kind)
	}
}

// TestResolutionFailurePathsHaveNoSideEffect is
// ver-failure-paths-have-no-side-effect.
//
// The strongest form of "leaves every session untouched" is that resolution has
// no capability to touch one: it is a pure function over an already-observed
// snapshot, with no context, no client and no terminal. The scan is what keeps
// it that way when someone later reaches for "just probe it here".
func TestResolutionFailurePathsHaveNoSideEffect(t *testing.T) {
	t.Parallel()

	before := fleet.Snapshot{Members: []fleet.Member{
		connected(entry(0, "one", "srv_one"), "a", "ab"),
	}}
	sessionsBefore := len(before.Members[0].Sessions)

	for _, query := range []Query{
		{Kind: KindID, Value: "missing"},
		{Kind: KindPrefix, Value: "a"},
		{Kind: KindID, Value: "a", Server: "nowhere"},
		{Kind: Kind(42), Value: "a"},
	} {
		outcome := Resolve(before, query)
		if outcome.Kind == Resolved {
			continue
		}
		if !outcome.Ref.IsZero() {
			t.Errorf("%+v: a failing resolution named a session anyway", query)
		}
	}
	if len(before.Members[0].Sessions) != sessionsBefore {
		t.Error("resolution mutated the snapshot it was given")
	}

	for _, offending := range packageSourcesContaining(t,
		".Dial(", ".Delete(", ".PostJSON(", ".GetJSON(", "MakeRaw", "http.") {
		t.Errorf("%s gives selector resolution a way to cause a side effect", offending)
	}
}

// packageSourcesContaining scans this package's production sources.
func packageSourcesContaining(t *testing.T, markers ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	var offending []string
	for _, file := range entries {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(".", name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		for _, marker := range markers {
			if strings.Contains(string(raw), marker) {
				offending = append(offending, name)
				break
			}
		}
	}
	return offending
}
