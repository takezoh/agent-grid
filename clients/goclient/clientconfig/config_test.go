package clientconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuildsCredentialFreeProjection(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join("..", "..", "config", "fixtures", "servers-valid-bearer.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "servers.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Servers) != 1 || !strings.HasPrefix(snapshot.Servers[0].ID, "srv_") {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	projection := snapshot.Projection()
	if projection.Servers[0].ServerID != snapshot.Servers[0].ID {
		t.Fatalf("projection id = %q, want %q", projection.Servers[0].ServerID, snapshot.Servers[0].ID)
	}
	if !projection.Servers[0].DefaultExecution {
		t.Fatal("legacy configuration did not select its first enabled execution server")
	}
}

// An invalid default-execution marker is an entry-scoped error under per-entry
// admission (adr-20260814-server-display-name-naming-contract), so it costs the
// entries it can be blamed on and leaves the host running. This test used to
// assert that Load returned an error; the assertion is not weakened, it is
// restated against the state the entry must now reach.
func TestInvalidDefaultExecutionMarkerCostsOnlyTheMarkedEntries(t *testing.T) {
	for name, expect := range map[string]struct {
		document   string
		admissions []string
		rule       string
	}{
		"two marked entries invalidate the marked group": {
			document: `{"schema_version":2,"servers":[` +
				`{"display_name":"A","enabled":true,"default_execution":true,"url":"http://127.0.0.1:1","auth":{"mode":"none"}},` +
				`{"display_name":"B","enabled":true,"default_execution":true,"url":"http://127.0.0.1:2","auth":{"mode":"none"}},` +
				`{"display_name":"C","enabled":true,"url":"http://127.0.0.1:3","auth":{"mode":"none"}}]}`,
			admissions: []string{AdmissionConfigurationInvalid, AdmissionConfigurationInvalid, AdmissionUsable},
			rule:       RuleAtMostOneDefault,
		},
		"a disabled marked entry costs only itself": {
			document: `{"schema_version":2,"servers":[` +
				`{"display_name":"A","enabled":false,"default_execution":true,"url":"http://127.0.0.1:1","auth":{"mode":"none"}},` +
				`{"display_name":"B","enabled":true,"url":"http://127.0.0.1:2","auth":{"mode":"none"}}]}`,
			admissions: []string{AdmissionConfigurationInvalid, AdmissionUsable},
			rule:       RuleDefaultMustBeEnabled,
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(expect.document), 0o600); err != nil {
				t.Fatal(err)
			}
			snapshot, err := Load(dir)
			if err != nil {
				t.Fatalf("an entry-scoped error must not stop the host: %v", err)
			}
			for index, want := range expect.admissions {
				got := snapshot.Servers[index]
				if got.Admission() != want {
					t.Errorf("entry %d admission = %q, want %q", index, got.Admission(), want)
				}
				if want == AdmissionConfigurationInvalid && got.Violation.Rule != expect.rule {
					t.Errorf("entry %d violated %q, want %q", index, got.Violation.Rule, expect.rule)
				}
			}
		})
	}
}

func TestLoadClassifiesMissingAndInvalid(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil || !strings.HasPrefix(err.Error(), "config_unavailable:") {
		t.Fatalf("missing error = %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(dir)
	if err == nil || !strings.HasPrefix(err.Error(), "config_invalid:") {
		t.Fatalf("invalid error = %v", err)
	}
}
