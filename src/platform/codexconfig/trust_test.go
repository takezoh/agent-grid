package codexconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestEnsureProjectTrustedAppendsWithoutChangingExistingText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# user comment\nmodel = \"gpt-5\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureProjectTrusted(path, `/work/acme "api"`); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), original) {
		t.Fatalf("existing config changed:\n%s", got)
	}
	var decoded map[string]any
	if _, err := toml.Decode(string(got), &decoded); err != nil {
		t.Fatalf("generated TOML is invalid: %v", err)
	}
	projects := decoded["projects"].(map[string]any)
	entry := projects[`/work/acme "api"`].(map[string]any)
	if entry["trust_level"] != "trusted" {
		t.Fatalf("trust_level = %#v", entry["trust_level"])
	}
}

func TestEnsureProjectTrustedIsIdempotentAndUpgradesExistingEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := "[projects.\"/work/acme\"]\ntrust_level = \"untrusted\"\nmarker = 7\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureProjectTrusted(path, "/work/acme"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if err := EnsureProjectTrusted(path, "/work/acme"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("second update was not idempotent")
	}
	var decoded map[string]any
	if _, err := toml.Decode(string(second), &decoded); err != nil {
		t.Fatal(err)
	}
	entry := decoded["projects"].(map[string]any)["/work/acme"].(map[string]any)
	if entry["trust_level"] != "trusted" || entry["marker"] != int64(7) {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestDefaultPathUsesNestedContainerCodexHome(t *testing.T) {
	if got, want := DefaultPath("/home/dev"), "/home/dev/.codex/codex/config.toml"; got != want {
		t.Fatalf("DefaultPath = %q, want %q", got, want)
	}
}
