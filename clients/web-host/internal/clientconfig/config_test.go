package clientconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuildsCredentialFreeProjection(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join("..", "..", "..", "config", "fixtures", "servers-valid-bearer.json")
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

func TestLoadRejectsInvalidDefaultExecutionServer(t *testing.T) {
	for name, document := range map[string]string{
		"duplicate": `{"schema_version":2,"servers":[{"display_name":"A","enabled":true,"default_execution":true,"url":"http://127.0.0.1:1","auth":{"mode":"none"}},{"display_name":"B","enabled":true,"default_execution":true,"url":"http://127.0.0.1:2","auth":{"mode":"none"}}]}`,
		"disabled":  `{"schema_version":2,"servers":[{"display_name":"A","enabled":false,"default_execution":true,"url":"http://127.0.0.1:1","auth":{"mode":"none"}},{"display_name":"B","enabled":true,"url":"http://127.0.0.1:2","auth":{"mode":"none"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(dir); err == nil {
				t.Fatal("expected invalid default execution server")
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
