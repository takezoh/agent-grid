package gorules_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Contract: contract-wire-shape-conformance
// (adr-20260814-sessioninfo-golden-vector-conformance).
//
// This is ver-golden-vector-renaming-fails-a-gate. The golden vector only
// protects the consumers that read it, so a consumer added without registering
// it is a consumer the gate silently stops covering — the failure mode of every
// checked-in fixture that nobody enumerates.
//
// The check is symmetric on purpose: a registered consumer that stops reading
// the vector is just as wrong as an unregistered one that does, because it
// means the registry is describing coverage that no longer exists.

// sessionInfoGoldenVectorPath is the repository-relative fixture.
const sessionInfoGoldenVectorPath = "protocol/fixtures/session-info-golden.json"

// sessionInfoGoldenConsumers is every place that reads the vector. Both sides
// of the conformance pair must be here: the server serializer that reproduces
// it, and the client decoder that reads it.
var sessionInfoGoldenConsumers = []string{
	"services/host/state/view/view_golden_test.go",
	"clients/cli/internal/gateway/wire_test.go",
}

func TestSessionInfoGoldenVectorIsRegistered(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, sessionInfoGoldenVectorPath)); err != nil {
		t.Fatalf("the golden vector is missing: %v", err)
	}

	for _, consumer := range sessionInfoGoldenConsumers {
		raw, err := os.ReadFile(filepath.Join(root, consumer))
		if err != nil {
			t.Errorf("registered consumer %s: %v", consumer, err)
			continue
		}
		if !readsGoldenVector(string(raw)) {
			t.Errorf("registered consumer %s no longer reads the golden vector", consumer)
		}
	}

	for _, found := range goldenVectorReaders(t, root) {
		if !slices.Contains(sessionInfoGoldenConsumers, found) {
			t.Errorf("%s reads the golden vector but is not registered in "+
				"sessionInfoGoldenConsumers, so the gate does not cover it", found)
		}
	}
}

// readsGoldenVector reports whether a source opens the vector rather than
// merely mentioning it. The distinction is that a reader ends a string literal
// with the file name — whether the literal is the bare name or the whole
// repository-relative path — while prose in a comment runs on into the next
// word. Without the distinction the gate would fire on every doc comment that
// explains what the vector is for, which teaches the next author to delete the
// explanation instead of registering the consumer.
func readsGoldenVector(body string) bool {
	for _, quote := range []string{`"`, `'`, "`"} {
		if strings.Contains(body, "session-info-golden.json"+quote) {
			return true
		}
	}
	return false
}

// goldenVectorReaders finds every source that names the vector, excluding the
// registry itself and the documentation that describes it.
func goldenVectorReaders(t *testing.T, root string) []string {
	t.Helper()
	var readers []string
	for _, tree := range []string{"services", "clients", "protocol"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == "obj" ||
					entry.Name() == "bin" {
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(path) {
			case ".go", ".ts", ".tsx", ".cs":
			default:
				return nil
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if !readsGoldenVector(string(raw)) {
				return nil
			}
			relative, _ := filepath.Rel(root, path)
			relative = filepath.ToSlash(relative)
			if relative == "services/gorules/session_info_golden_vector_test.go" {
				return nil
			}
			readers = append(readers, relative)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
	}
	return readers
}
