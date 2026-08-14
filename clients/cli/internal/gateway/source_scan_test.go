package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// source_scan_test.go holds the helpers for the structural gates in this
// package. They read this module's own sources, which is what lets a check say
// "nowhere in the module" instead of "not on the path this test happened to
// take".
//
// Every helper takes a root, so the gates can be pointed at a synthesised tree
// containing a deliberate violation. A structural gate that has never been seen
// to fire is indistinguishable from one that cannot.

// moduleRoot walks up from the package directory to the module root.
func moduleRoot(t *testing.T) string {
	t.Helper()
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve package directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("no go.mod above the package directory")
		}
		directory = parent
	}
}

// sourcesContainingIn returns the root-relative paths of every production Go
// source under root containing any of the markers.
//
// Test sources are excluded because a gate's own test must be allowed to name
// the thing it forbids; every gate here is written so the forbidden text
// appears only inside the assertion.
func sourcesContainingIn(t *testing.T, root string, markers ...string) []string {
	t.Helper()
	var offending []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		body := string(raw)
		for _, marker := range markers {
			if strings.Contains(body, marker) {
				relative, _ := filepath.Rel(root, path)
				offending = append(offending, relative)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk sources under %s: %v", root, err)
	}
	return offending
}

// sourcesOutsideIn is sourcesContainingIn restricted to files outside the one
// directory allowed to own the marked construct. It is the shape every
// single-owner gate in this module takes.
func sourcesOutsideIn(t *testing.T, root, allowed string, markers ...string) []string {
	t.Helper()
	var offending []string
	for _, candidate := range sourcesContainingIn(t, root, markers...) {
		if strings.HasPrefix(filepath.ToSlash(candidate), allowed) {
			continue
		}
		offending = append(offending, candidate)
	}
	return offending
}

func moduleSourcesContaining(t *testing.T, markers ...string) []string {
	t.Helper()
	return sourcesContainingIn(t, moduleRoot(t), markers...)
}

func moduleSourcesOutside(t *testing.T, allowed string, markers ...string) []string {
	t.Helper()
	return sourcesOutsideIn(t, moduleRoot(t), allowed, markers...)
}
