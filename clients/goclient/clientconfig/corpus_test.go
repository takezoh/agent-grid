package clientconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-client-config-parity, contract-config-entry-admission,
// contract-server-naming-prerequisite
// (adr-20260814-shared-go-client-config-module,
// adr-20260814-server-display-name-naming-contract).
//
// clients/config declared servers.json a shared contract long before anything
// could check it: the Go test read one fixture and asserted its shape, the
// TypeScript test asserted a bare .toThrow(), and the C# test asserted
// Assert.Throws. Three loaders that each merely throw are not three loaders
// that agree — they can reject the same document for different reasons and
// accept the same document with different results, which is how one gateway
// acquired two identities across hosts.
//
// The corpus fixes that by recording the outcome rather than the shape. This
// file is the Go reader of it; clients/workspace/tests/config-corpus.test.ts
// and the C# ConfigCorpusTests read the same file and assert the same record.

type corpusFile struct {
	Cases []corpusCase `json:"cases"`
}

type corpusServer struct {
	Index            int    `json:"index"`
	DisplayName      string `json:"display_name"`
	Admission        string `json:"admission"`
	Enabled          bool   `json:"enabled"`
	DefaultExecution bool   `json:"default_execution"`
	CanonicalURL     string `json:"canonical_url"`
	ServerID         string `json:"server_id"`
	Rule             string `json:"rule"`
}

type corpusCase struct {
	Name     string   `json:"name"`
	Rules    []string `json:"rules"`
	Why      string   `json:"why"`
	Document string   `json:"document"`
	Expected struct {
		Decision string         `json:"decision"`
		Servers  []corpusServer `json:"servers"`
	} `json:"expected"`
}

func loadCorpus(t *testing.T) corpusFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "fixtures", "servers-corpus.json"))
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus corpusFile
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("corpus carries no cases")
	}
	return corpus
}

// writeDocument seeds a temporary configuration directory with one document.
func writeDocument(t *testing.T, document string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestExpectedOutcomeCorpus is ver-go-loader-matches-the-expected-outcome-corpus.
//
// It fails an implementation that passes a fixture without matching its
// recorded decision, admission states, canonical URL and derived identifier —
// the failure mode a bare throws-or-does-not-throw assertion cannot see.
func TestExpectedOutcomeCorpus(t *testing.T) {
	t.Parallel()

	for _, testCase := range loadCorpus(t).Cases {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			snapshot, err := Load(writeDocument(t, testCase.Document))

			if testCase.Expected.Decision == "stop" {
				if err == nil {
					t.Fatalf("the host started but the corpus records a file-level stop (rules %v)", testCase.Rules)
				}
				return
			}
			if err != nil {
				t.Fatalf("the host stopped but the corpus records a load: %v", err)
			}
			assertSnapshotMatches(t, snapshot, testCase)
		})
	}
}

func assertSnapshotMatches(t *testing.T, snapshot Snapshot, testCase corpusCase) {
	t.Helper()
	if len(snapshot.Servers) != len(testCase.Expected.Servers) {
		t.Fatalf("loaded %d entries, corpus records %d",
			len(snapshot.Servers), len(testCase.Expected.Servers))
	}
	for position, want := range testCase.Expected.Servers {
		got := snapshot.Servers[position]
		if got.Index != want.Index {
			t.Errorf("entry %d index = %d, want %d", position, got.Index, want.Index)
		}
		if got.DisplayName != want.DisplayName {
			t.Errorf("entry %d display_name = %q, want %q — the name is reported as written",
				position, got.DisplayName, want.DisplayName)
		}
		if got.Admission() != want.Admission {
			t.Errorf("entry %d admission = %q, want %q", position, got.Admission(), want.Admission)
		}
		if want.Admission == AdmissionConfigurationInvalid {
			if got.Violation == nil {
				t.Fatalf("entry %d carries no violation", position)
			}
			if got.Violation.Rule != want.Rule {
				t.Errorf("entry %d violated rule = %q, want %q", position, got.Violation.Rule, want.Rule)
			}
			continue
		}
		if got.Enabled != want.Enabled {
			t.Errorf("entry %d enabled = %v, want %v", position, got.Enabled, want.Enabled)
		}
		if got.DefaultExecution != want.DefaultExecution {
			t.Errorf("entry %d default_execution = %v, want %v",
				position, got.DefaultExecution, want.DefaultExecution)
		}
		if got.URL != want.CanonicalURL {
			t.Errorf("entry %d canonical url = %q, want %q", position, got.URL, want.CanonicalURL)
		}
		if got.ID != want.ServerID {
			t.Errorf("entry %d server id = %q, want %q", position, got.ID, want.ServerID)
		}
	}
}

// TestExpectedOutcomeCorpusCoversEveryDeclaredRule is
// ver-corpus-covers-every-per-entry-and-file-level-rule.
//
// The rule registry lives in the schema rather than in the corpus, so the
// check is not circular: a rule the contract declares and no fixture drives is
// a promise nothing keeps. Four such rules stood before this corpus — a
// duplicate URL, more than one default execution entry, zero enabled entries,
// and an unparseable document.
func TestExpectedOutcomeCorpusCoversEveryDeclaredRule(t *testing.T) {
	t.Parallel()

	declared := declaredRules(t)
	exercised := map[string]int{}
	for _, testCase := range loadCorpus(t).Cases {
		if len(testCase.Rules) == 0 {
			t.Errorf("corpus case %q names no rule", testCase.Name)
		}
		for _, rule := range testCase.Rules {
			if _, ok := declared[rule]; !ok {
				t.Errorf("corpus case %q names rule %q, which the schema does not declare", testCase.Name, rule)
			}
			exercised[rule]++
		}
	}
	for id, rule := range declared {
		if exercised[id] == 0 {
			t.Errorf("rule %q is declared but no fixture exercises it: %s", id, rule.Statement)
		}
	}
}

// TestEveryEntryScopedRuleIsBlamedOnAnEntry pins the partition the admission
// contract turns on: an error is per-entry exactly when it can be attributed,
// and file-level exactly when no entry can be named. A rule declared
// entry-scoped whose fixture stops the host would quietly restore the old
// blast radius.
func TestEveryEntryScopedRuleIsBlamedOnAnEntry(t *testing.T) {
	t.Parallel()

	declared := declaredRules(t)
	for _, testCase := range loadCorpus(t).Cases {
		scopes := map[string]bool{}
		for _, rule := range testCase.Rules {
			scopes[declared[rule].Scope] = true
		}
		if scopes["file"] {
			continue
		}
		if testCase.Expected.Decision != "load" {
			t.Errorf("corpus case %q stops the host, but names only entry-scoped rules %v",
				testCase.Name, testCase.Rules)
		}
	}
}

type contractRule struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Statement string `json:"statement"`
}

func declaredRules(t *testing.T) map[string]contractRule {
	t.Helper()
	var schema struct {
		Rules []contractRule `json:"x-contract-rules"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "servers.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	if len(schema.Rules) == 0 {
		t.Fatal("the schema declares no contract rules")
	}
	declared := make(map[string]contractRule, len(schema.Rules))
	for _, rule := range schema.Rules {
		declared[rule.ID] = rule
	}
	return declared
}

// TestUnknownFieldIsRejected is half of
// ver-measured-divergences-are-closed-by-the-corpus.
//
// This loader used to drop an unrecognised field in silence while the other
// two stopped the whole host for it, so the same document meant different
// things depending on which host opened it — and the operator who added the
// field was told nothing either way.
func TestUnknownFieldIsRejected(t *testing.T) {
	t.Parallel()

	document := `{"schema_version":2,"servers":[{"display_name":"Extra","enabled":true,` +
		`"url":"http://127.0.0.1:8765","auth":{"mode":"none"},"health":"unknown"}]}`
	snapshot, err := Load(writeDocument(t, document))
	if err != nil {
		t.Fatalf("one bad entry must not stop the host: %v", err)
	}
	entry := snapshot.Servers[0]
	if entry.Violation == nil || entry.Violation.Rule != RuleUnknownEntryField {
		t.Fatalf("an unrecognised entry field was accepted and dropped: %+v", entry.Violation)
	}
	if !strings.Contains(entry.Violation.Detail, "health") {
		t.Errorf("the report does not name the offending field: %q", entry.Violation.Detail)
	}
}

// TestUrlCredentialsAreRejectedNotStripped is the other half.
//
// A gateway URL carrying a credential is something an operator wrote on
// purpose. Stripping it — which the TypeScript and C# loaders did — leaves
// them believing authentication is configured while the client sends none, and
// it also splits identity, because the stripped and unstripped spellings
// digest differently in the loader that keeps them.
func TestUrlCredentialsAreRejectedNotStripped(t *testing.T) {
	t.Parallel()

	for name, rawURL := range map[string]string{
		"credentials": "http://user:secret@127.0.0.1:8765",
		"query":       "http://127.0.0.1:8765/?token=ignored",
		"fragment":    "http://127.0.0.1:8765/#section",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			document := `{"schema_version":2,"servers":[{"display_name":"X","enabled":true,` +
				`"url":"` + rawURL + `","auth":{"mode":"none"}}]}`
			snapshot, err := Load(writeDocument(t, document))
			if err != nil {
				t.Fatalf("one bad entry must not stop the host: %v", err)
			}
			entry := snapshot.Servers[0]
			if entry.Violation == nil || entry.Violation.Rule != RuleURLBareAuthority {
				t.Fatalf("%s in a gateway url was accepted; it must be rejected, not stripped: %+v",
					name, entry.Violation)
			}
			if entry.URL != rawURL {
				t.Errorf("the entry's url was rewritten to %q; a loader never repairs what it reads", entry.URL)
			}
		})
	}
}
