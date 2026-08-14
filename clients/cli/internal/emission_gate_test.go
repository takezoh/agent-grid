// Package internal holds no production code. It exists so that a gate about
// the whole module — rather than about one package — has a home, and so that
// `go test ./internal/` names something.
package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-explicit-terminate (unit-lifecycle-commands).

// terminationOwner is the one file allowed to ask a server to end a session.
const terminationOwner = "lifecycle/terminate.go"

// TestTerminateRequestHasExactlyOneEmissionSite is ver-terminate-is-the-only-emitter.
//
// FR-001 forbids a termination on detach, Ctrl-C, terminal close, stdin end,
// catchable signals, an uncatchable kill, a WebSocket close, and a post-raw-mode
// failure. Testing each of those paths shows that today's code does not emit
// one; this shows that no code can, because the call does not exist anywhere
// else to reach for.
//
// The marker is the route rather than the verb: `client.Delete` is a general
// method and could legitimately be used for something else later, but the
// session route is the termination.
func TestTerminateRequestHasExactlyOneEmissionSite(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve package directory: %v", err)
	}

	var emitters []string
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
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
		if !strings.Contains(body, `"/api/sessions/"`) {
			return nil
		}
		if !strings.Contains(body, "Delete(") {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		emitters = append(emitters, filepath.ToSlash(relative))
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk module sources: %v", walkErr)
	}

	if len(emitters) != 1 {
		t.Fatalf("%d files can emit a termination: %v (want exactly %s)",
			len(emitters), emitters, terminationOwner)
	}
	if emitters[0] != terminationOwner {
		t.Errorf("the termination is emitted from %s, not from %s", emitters[0], terminationOwner)
	}
}
