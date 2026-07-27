package harnesspolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryE2ESuitesContainEveryRequiredFidelityPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "test-harness", "e2e-suites.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := ParseE2ESuiteRegistry(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"stream-routing", "claude-cli", "grok-cli", "codex-app-server", "agent-hook", "docker-cli"} {
		if _, ok := registry.Find(id); !ok {
			t.Errorf("required T3 suite %q is absent", id)
		}
	}
}

func TestE2ESuiteRegistryRejectsDuplicateAndNonE2EPackage(t *testing.T) {
	_, err := ParseE2ESuiteRegistry([]byte(`{"version":1,"suites":[{"id":"dup","package":"./x/...","dependency_id":"x"},{"id":"dup","package":"./x/...","dependency_id":"x"}]}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate registry error = %v", err)
	}
	_, err = ParseE2ESuiteRegistry([]byte(`{"version":1,"suites":[{"id":"unit","package":"./x","dependency_id":"x"}]}`))
	if err == nil || !strings.Contains(err.Error(), "must end in /...") {
		t.Fatalf("package shape error = %v", err)
	}
}
