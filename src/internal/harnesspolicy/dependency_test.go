package harnesspolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyRegistryValidFixture(t *testing.T) {
	registry := loadRegistryFixture(t, "valid.json")
	root := filepath.Join("testdata", "dependencies", "repo")
	if errs := CheckDependencyRegistry(root, registry); len(errs) != 0 {
		t.Fatalf("valid fixture rejected: %v", errs)
	}
}

func TestRepositoryDependencyRegistry(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "test-harness", "dependencies.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := ParseDependencyRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	if errs := CheckDependencyRegistry(root, registry); len(errs) != 0 {
		t.Fatalf("repository registry rejected: %v", errs)
	}
}

func TestDependencyRegistryNegativeFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		want    string
	}{
		{"empty-assertion.json", "dependency empty-assertion: missing contract.assertion"},
		{"missing-fidelity.json", "dependency missing-fidelity: missing fidelity.path"},
		{"non-pty-exception.json", "dependency database: only pty may use"},
		{"name-only-fake.json", "dependency name-only: fake symbol must be public"},
		{"marker-missing.json", "dependency marker-missing: contract.assertion marker"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			registry := loadRegistryFixture(t, test.fixture)
			errs := CheckDependencyRegistry(filepath.Join("testdata", "dependencies", "repo"), registry)
			if !errorsContain(errs, test.want) {
				t.Fatalf("errors %v do not contain %q", errs, test.want)
			}
		})
	}
}

func TestDependencyRegistryRejectsUnknownField(t *testing.T) {
	_, err := ParseDependencyRegistry([]byte(`{"version":1,"unknown":true,"dependencies":[]}`))
	if err == nil {
		t.Fatal("unknown field accepted")
	}
}

func loadRegistryFixture(t *testing.T, name string) DependencyRegistry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "dependencies", name))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := ParseDependencyRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func errorsContain(errs []error, want string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), want) {
			return true
		}
	}
	return false
}
