package view

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-discovery-projection, contract-config-entry-admission
// (unit-discovery-render).

func configuredServer(index int, name string) clientconfig.Server {
	return clientconfig.Server{
		Index: index, ID: "srv_" + name, DisplayName: name, Enabled: true,
		URL: "https://" + name + ".test", Auth: clientconfig.Auth{Mode: "none"},
	}
}

func publishedSession(id, title, lifecycle string) gateway.SessionInfo {
	return gateway.SessionInfo{
		ID: id, Project: "/home/operator/repos/agent-grid", Command: "claude",
		Status: gateway.SessionStatus{Lifecycle: lifecycle, Failure: "none"},
		View: gateway.View{
			Card:        gateway.Card{Title: title},
			DisplayName: "claude",
			Model:       "claude-opus-5",
		},
	}
}

func connected(index int, name string, sessions ...gateway.SessionInfo) fleet.Member {
	server := configuredServer(index, name)
	return fleet.Member{
		Index: index, ServerID: server.ID, DisplayName: name, Server: server,
		Observation: fleet.Observation{Kind: fleet.KindConnected}, Sessions: sessions,
	}
}

// TestEveryRowCarriesItsServer is ver-rows-carry-owning-server.
//
// A listing that spans several servers and prints only session identifiers
// leaves the operator with a value they cannot act on: the next command needs
// to know which server to send it to.
func TestEveryRowCarriesItsServer(t *testing.T) {
	t.Parallel()

	snapshot := fleet.Snapshot{Members: []fleet.Member{
		connected(0, "alpha", publishedSession("s1", "First", "active")),
		connected(1, "beta", publishedSession("s2", "Second", "idle")),
	}}

	rows := Rows(snapshot)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	for _, row := range rows {
		if row.Ref.ServerName() == "" || row.Ref.ServerID() == "" {
			t.Errorf("row %q names no server", row.Ref.SessionID())
		}
	}

	var out bytes.Buffer
	WriteTable(&out, snapshot)
	rendered := out.String()
	for _, expected := range []string{"alpha", "beta", "s1", "s2"} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("the rendered table does not carry %q:\n%s", expected, rendered)
		}
	}
}

// TestDisplayNameIsServerCardTitleVerbatim is
// ver-display-name-is-verbatim-card-title.
//
// Three wrong sources are all available and all tempting: view.display_name is
// the driver name and is the same for every session of that driver; the project
// path's basename is the same for every session in a repository; and truncating
// the title to fit a column makes two long titles identical. The title is used
// exactly as published.
func TestDisplayNameIsServerCardTitleVerbatim(t *testing.T) {
	t.Parallel()

	const long = "Wire up the terminal client and pin its wire shape with a golden vector"
	session := publishedSession("s1", long, "active")
	snapshot := fleet.Snapshot{Members: []fleet.Member{connected(0, "alpha", session)}}

	row := Rows(snapshot)[0]
	if row.Ref.DisplayName() != long {
		t.Errorf("display name = %q, want the card title verbatim", row.Ref.DisplayName())
	}
	if row.Ref.DisplayName() == session.View.DisplayName {
		t.Error("the driver name was used as the display name")
	}

	var out bytes.Buffer
	WriteTable(&out, snapshot)
	if !strings.Contains(out.String(), long) {
		t.Errorf("the rendered table truncated the display name:\n%s", out.String())
	}

	// An empty title stays empty. Substituting a fallback here would disagree
	// with what every graphical client shows for the same session.
	blank := publishedSession("s2", "", "active")
	blankRow := Rows(fleet.Snapshot{Members: []fleet.Member{connected(0, "alpha", blank)}})[0]
	if blankRow.Ref.DisplayName() != "" {
		t.Errorf("an empty card title was replaced with %q", blankRow.Ref.DisplayName())
	}
}

// TestEndedSessionsArePresentedNotFiltered is ver-cli-never-drops-a-published-session.
//
// The execution plane now retains an ended session with a terminal lifecycle
// state precisely so clients can show it. A client-side filter would throw that
// away and put the operator back to being unable to tell an ended session from
// one that never existed.
func TestEndedSessionsArePresentedNotFiltered(t *testing.T) {
	t.Parallel()

	snapshot := fleet.Snapshot{Members: []fleet.Member{connected(0, "alpha",
		publishedSession("running", "Live", "active"),
		publishedSession("ended", "Finished", "stopped"),
	)}}

	rows := Rows(snapshot)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want both the running and the ended session", len(rows))
	}
	var ended Row
	for _, row := range rows {
		if row.Ref.SessionID() == "ended" {
			ended = row
		}
	}
	if ended.Ref.SessionID() == "" {
		t.Fatal("the ended session was filtered out by the client")
	}
	if ended.State != StateStopped {
		t.Errorf("the ended session reads as %q, want %q", ended.State, StateStopped)
	}

	var out bytes.Buffer
	WriteTable(&out, snapshot)
	if !strings.Contains(out.String(), "Stopped") {
		t.Errorf("the ended session's state is not shown:\n%s", out.String())
	}
}

// TestJSONDistinguishesUnreachableFromNoSessions is
// ver-json-separates-unreachable-from-empty.
//
// Both cases contribute zero rows to the session array. If the structured
// output says nothing else about them, automation cannot tell "everything is
// fine and quiet" from "half the fleet is down", which is the only distinction
// that matters when deciding whether to page someone.
func TestJSONDistinguishesUnreachableFromNoSessions(t *testing.T) {
	t.Parallel()

	quiet := connected(0, "quiet")
	down := fleet.Member{
		Index: 1, ServerID: "srv_down", DisplayName: "down", Server: configuredServer(1, "down"),
		Observation: fleet.Observation{
			Kind: fleet.KindUnavailable, Reason: fleet.ReasonConnectionTimeout,
		},
	}
	snapshot := fleet.Snapshot{Members: []fleet.Member{quiet, down}}

	document := decodeListing(t, snapshot)

	if len(document.Sessions) != 0 {
		t.Fatalf("expected no session rows, got %d", len(document.Sessions))
	}
	if len(document.Servers) != 2 {
		t.Fatalf("%d server entries, want one per configured entry", len(document.Servers))
	}
	first, second := document.Servers[0], document.Servers[1]
	if first.Reachability != fleet.KindConnected || first.SessionCount != 0 {
		t.Errorf("a healthy but quiet server reads as %+v", first)
	}
	if second.Reachability != fleet.KindUnavailable {
		t.Errorf("an unreachable server reads as %q", second.Reachability)
	}
	if second.Reason != fleet.ReasonConnectionTimeout {
		t.Errorf("the unreachable server carries reason %q", second.Reason)
	}
	if first.Reachability == second.Reachability {
		t.Error("a quiet server and an unreachable one are indistinguishable")
	}
}

// TestConfigurationInvalidIsDisjointFromReachabilityAndLifecycle is
// ver-invalid-entry-is-a-third-vocabulary-in-discovery.
//
// Rendering an unusable entry as `unavailable` would send the operator to check
// a network for a problem that is a typo in their own file; omitting it would
// leave them wondering where the server went. It is a third thing, and it says
// which rule it broke.
func TestConfigurationInvalidIsDisjointFromReachabilityAndLifecycle(t *testing.T) {
	t.Parallel()

	invalidServer := configuredServer(1, "-dash")
	invalidServer.Violation = &clientconfig.Violation{
		Rule: clientconfig.RuleDisplayNameCharset, Detail: "display name begins with -",
	}
	invalid := fleet.Member{
		Index: 1, ServerID: invalidServer.ID, DisplayName: "-dash", Server: invalidServer,
		Observation: fleet.Observation{
			Kind:   fleet.KindConfigurationInvalid,
			Rule:   clientconfig.RuleDisplayNameCharset,
			Detail: "display name begins with -",
		},
	}
	snapshot := fleet.Snapshot{Members: []fleet.Member{
		connected(0, "alpha", publishedSession("s1", "Live", "active")),
		invalid,
	}}

	document := decodeListing(t, snapshot)

	entry := document.Servers[1]
	if entry.Admission != fleet.KindConfigurationInvalid {
		t.Errorf("admission = %q, want %q", entry.Admission, fleet.KindConfigurationInvalid)
	}
	if entry.Reachability != "" {
		t.Errorf("an entry that was never contacted carries reachability %q", entry.Reachability)
	}
	if entry.Rule != clientconfig.RuleDisplayNameCharset {
		t.Errorf("the violated rule is not reported: %q", entry.Rule)
	}
	if entry.Index != 1 || entry.DisplayName != "-dash" {
		t.Errorf("the entry is not named by its position and its name as written: %+v", entry)
	}
	// The three vocabularies must not share a word.
	for _, session := range document.Sessions {
		if session.State == fleet.KindConfigurationInvalid {
			t.Error("configuration invalidity leaked into a session lifecycle state")
		}
	}

	var out bytes.Buffer
	WriteTable(&out, snapshot)
	rendered := out.String()
	if !strings.Contains(rendered, "-dash") || !strings.Contains(rendered, "configuration invalid") {
		t.Errorf("the invalid entry is missing from the human listing:\n%s", rendered)
	}
	if strings.Contains(rendered, "unavailable") {
		t.Errorf("the invalid entry was rendered in reachability vocabulary:\n%s", rendered)
	}
	// The healthy server's session still shows: one bad entry costs that entry.
	if !strings.Contains(rendered, "Live") {
		t.Errorf("a sibling session was lost to an invalid entry:\n%s", rendered)
	}
}

// TestJSONFieldBreadthMatchesTheDefaultColumns pins cc-json-field-breadth: the
// structured output never withholds a value the table shows.
func TestJSONFieldBreadthMatchesTheDefaultColumns(t *testing.T) {
	t.Parallel()

	session := publishedSession("s1", "Live", "active")
	session.WorkspaceRoot = "/home/operator/worktrees/topic"
	snapshot := fleet.Snapshot{Members: []fleet.Member{connected(0, "alpha", session)}}

	document := decodeListing(t, snapshot)
	row := document.Sessions[0]

	for name, value := range map[string]string{
		"server_id":    row.ServerID,
		"server_name":  row.ServerName,
		"session_id":   row.SessionID,
		"display_name": row.DisplayName,
		"cwd":          row.Cwd,
		"worktree":     row.Worktree,
		"state":        row.State,
		"state_label":  row.StateLabel,
		"model":        row.Model,
	} {
		if value == "" {
			t.Errorf("%s is empty; the structured output must carry every default column", name)
		}
	}
	// Identity is two fields, never one concatenated string.
	if strings.Contains(row.SessionID, row.ServerID) {
		t.Error("session identity carries the server identity glued into it")
	}
}

func decodeListing(t *testing.T, snapshot fleet.Snapshot) jsonDocument {
	t.Helper()
	var out bytes.Buffer
	if err := WriteJSON(&out, snapshot); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var document jsonDocument
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("decode listing: %v\n%s", err, out.String())
	}
	return document
}
