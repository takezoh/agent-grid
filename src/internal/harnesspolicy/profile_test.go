package harnesspolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesRequiresEverySpeedTier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"profiles":[{"name":"save","tier":"T0","commands":["go test"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProfiles(path); err == nil {
		t.Fatal("missing profiles unexpectedly accepted")
	}
}

func TestRepositoryProfilesAreComplete(t *testing.T) {
	registry, err := LoadProfiles(filepath.Join("..", "..", "..", "test-harness", "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"save", "pre-push", "pr", "nightly"} {
		if _, ok := registry.Find(name); !ok {
			t.Fatalf("missing %s", name)
		}
	}
}
