package clientconfig

import (
	"strings"
	"testing"
)

// Contract: contract-config-entry-admission, contract-server-naming-prerequisite
// (adr-20260814-server-display-name-naming-contract).

// TestHostStartsAndOnlyOffendingEntriesAreInvalid is
// ver-host-starts-and-only-the-offending-entries-are-unusable.
//
// Whole-file rejection has the wrong blast radius for a rule about one entry:
// a typo in the display name of a server the operator is not using would take
// away every other server, on every host, at once. This fails an
// implementation that refuses the whole configuration for one bad entry — and
// equally one that lets a bad entry cost a sibling that is fine.
func TestHostStartsAndOnlyOffendingEntriesAreInvalid(t *testing.T) {
	t.Parallel()

	document := `{"schema_version":2,"servers":[
		{"display_name":"-leading-dash","enabled":true,"url":"http://127.0.0.1:8765","auth":{"mode":"none"}},
		{"display_name":"Good","enabled":true,"url":"http://127.0.0.1:9000","auth":{"mode":"none"}},
		{"display_name":"Broken url","enabled":true,"url":"ws://127.0.0.1:9100","auth":{"mode":"none"}},
		{"display_name":"Also good","enabled":true,"url":"http://127.0.0.1:9200","auth":{"mode":"none"}}
	]}`

	snapshot, err := Load(writeDocument(t, document))
	if err != nil {
		t.Fatalf("the host must start with a configuration containing bad entries: %v", err)
	}
	if len(snapshot.Servers) != 4 {
		t.Fatalf("loaded %d entries, want all 4 admitted in some state", len(snapshot.Servers))
	}

	wantAdmission := []string{
		AdmissionConfigurationInvalid,
		AdmissionUsable,
		AdmissionConfigurationInvalid,
		AdmissionUsable,
	}
	for index, want := range wantAdmission {
		if got := snapshot.Servers[index].Admission(); got != want {
			t.Errorf("entry %d admission = %q, want %q", index, got, want)
		}
	}

	usable := snapshot.UsableServers()
	if len(usable) != 2 {
		t.Fatalf("usable entries = %d, want 2 — a bad entry cost a sibling that was fine", len(usable))
	}
	// The default execution target falls to the first usable entry, never to an
	// invalid one, so a broken entry cannot become the target for new work.
	if !usable[0].DefaultExecution || usable[0].DisplayName != "Good" {
		t.Errorf("default execution landed on %q; want the first usable entry", usable[0].DisplayName)
	}
	if len(snapshot.InvalidServers()) != 2 {
		t.Errorf("invalid entries = %d, want 2 — an unusable entry must be reported, not omitted",
			len(snapshot.InvalidServers()))
	}
}

// TestUnreadableEntryListIsAFileLevelStop is
// ver-unreadable-entry-list-still-stops-the-host.
//
// Per-entry admission holds exactly where an entry can be named. When the
// entry list itself cannot be read there is no entry to blame, so the host
// stops rather than starting with a silently empty fleet — which would look
// identical to an operator who genuinely configured nothing.
func TestUnreadableEntryListIsAFileLevelStop(t *testing.T) {
	t.Parallel()

	for name, document := range map[string]string{
		"truncated document":  `{"schema_version":2,"servers":[{"display_name":"Truncated"`,
		"unsupported version": `{"schema_version":1,"servers":[]}`,
		"absent entry list":   `{"schema_version":2}`,
		"entry list is not an array": `{"schema_version":2,"servers":"none"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Load(writeDocument(t, document)); err == nil {
				t.Fatal("the host started with an unreadable entry list instead of stopping")
			}
		})
	}
}

// TestDisplayNameCharsetRejection is ver-charset-violation-flagged-identically.
//
// The name is becoming addressable — it is what --server takes — so it must be
// something a command line can carry. A leading dash is read as a flag and a
// control character cannot be typed or displayed reliably.
func TestDisplayNameCharsetRejection(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		rule string
	}{
		"leading dash":      {name: "-prod", rule: RuleDisplayNameCharset},
		"control character": {name: "pro\u0007d", rule: RuleDisplayNameCharset},
		"tab":               {name: "pro\td", rule: RuleDisplayNameCharset},
		"blank":             {name: "   ", rule: RuleDisplayNameRequired},
		"empty":             {name: "", rule: RuleDisplayNameRequired},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			if violation := checkDisplayName(tc.name); violation == nil {
				t.Fatalf("display name %q was accepted", tc.name)
			} else if violation.Rule != tc.rule {
				t.Errorf("display name %q violated %q, want %q", tc.name, violation.Rule, tc.rule)
			}
		})
	}

	// Trimming is part of the rule, so a name that is sound once trimmed is
	// sound. Uniqueness uses the same trimmed key.
	for _, accepted := range []string{" prod ", "prod-1", "本番", "prod (eu)"} {
		if violation := checkDisplayName(accepted); violation != nil {
			t.Errorf("display name %q was rejected: %v", accepted, violation)
		}
	}
}

// TestNamingViolationReportsOffendingEntryWithoutRenaming is
// ver-naming-violation-names-the-entry-without-renaming.
//
// A duplicate is a property of a pair, so blaming one member requires choosing
// which — and every choice rule (first wins, last wins, lexical) silently
// redirects an operator's existing --server <name> to a different machine.
// This fails an implementation that auto-suffixes, disables, drops or picks a
// winner, and equally one that reports without naming the offending entry.
func TestNamingViolationReportsOffendingEntryWithoutRenaming(t *testing.T) {
	t.Parallel()

	document := `{"schema_version":2,"servers":[
		{"display_name":"prod","enabled":true,"url":"http://127.0.0.1:8765","auth":{"mode":"none"}},
		{"display_name":" prod ","enabled":true,"url":"http://127.0.0.1:9000","auth":{"mode":"none"}},
		{"display_name":"staging","enabled":true,"url":"http://127.0.0.1:9100","auth":{"mode":"none"}}
	]}`

	snapshot, err := Load(writeDocument(t, document))
	if err != nil {
		t.Fatalf("a name collision must not stop the host: %v", err)
	}

	collided := snapshot.Servers[:2]
	for index, entry := range collided {
		if entry.Admission() != AdmissionConfigurationInvalid {
			t.Errorf("collision member %d admission = %q; no winner may be chosen",
				index, entry.Admission())
		}
		if entry.Violation == nil || entry.Violation.Rule != RuleDisplayNameUnique {
			t.Errorf("collision member %d violated %+v, want %q", index, entry.Violation, RuleDisplayNameUnique)
		}
		if entry.Index != index {
			t.Errorf("collision member %d reports position %d; the report must locate the entry",
				index, entry.Index)
		}
	}
	// The names are reported exactly as written, including the whitespace that
	// only the uniqueness key trims.
	if collided[0].DisplayName != "prod" || collided[1].DisplayName != " prod " {
		t.Errorf("display names were normalised: %q and %q", collided[0].DisplayName, collided[1].DisplayName)
	}
	for _, entry := range snapshot.Servers {
		if strings.HasSuffix(entry.DisplayName, "-2") || strings.Contains(entry.DisplayName, "(1)") {
			t.Errorf("entry %d was auto-suffixed to %q", entry.Index, entry.DisplayName)
		}
	}
	if snapshot.Servers[2].Admission() != AdmissionUsable {
		t.Errorf("the uncollided entry became %q", snapshot.Servers[2].Admission())
	}
	if !snapshot.Servers[2].DefaultExecution {
		t.Error("the default execution target must fall to a usable entry, never to a collided one")
	}
}
