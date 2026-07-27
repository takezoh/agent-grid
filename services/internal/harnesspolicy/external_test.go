package harnesspolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryExternalBoundaryInventory(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	dependencyData, err := os.ReadFile(filepath.Join(root, "test-harness", "dependencies.json"))
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := ParseDependencyRegistry(dependencyData)
	if err != nil {
		t.Fatal(err)
	}
	boundaryData, err := os.ReadFile(filepath.Join(root, "test-harness", "external-boundaries.json"))
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := ParseExternalBoundaryRegistry(boundaryData)
	if err != nil {
		t.Fatal(err)
	}
	if errs := CheckExternalBoundaries(root, boundaries, dependencies); len(errs) != 0 {
		t.Fatalf("external boundary inventory drift: %v", errs)
	}
}

func TestExternalBoundaryInventoryRejectsUnknownDependency(t *testing.T) {
	registry := ExternalBoundaryRegistry{Version: 1, Entries: []ExternalBoundaryEntry{{Path: "src/x.go", Kinds: []string{"process"}, Dependencies: []string{"missing"}}}}
	err := CheckExternalBoundaries(t.TempDir(), registry, DependencyRegistry{Version: 2})
	if !errorsContain(err, "unknown dependency missing") {
		t.Fatalf("errors = %v", err)
	}
}
