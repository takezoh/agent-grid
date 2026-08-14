package gorules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-module-boundary (adr-20260814-cli-independent-go-module).
//
// This is ver-cli-registered-in-gates. clients/cli is a fourth Go module with
// its own go.mod, which means every repository-wide entry point that says
// "./..." misses it by construction: `make build-all`, `make test`, `make vet`,
// `make lint` and the pull-request profile each enumerate their modules by
// hand. A module that is not enumerated is a module nothing runs — it compiles
// on the author's machine and rots from the first commit after.
//
// The check is over the entry points rather than over a list of files, because
// what must not be forgotten is coverage, not any particular line of Makefile.

// cliModuleEntryPoints names each gate and the marker that proves the CLI
// module reaches it. Markers are chosen to survive reformatting: they name the
// directory variable or the profile command, not a whole line.
func TestCLIModuleRegisteredInBuildAndVerificationEntryPoints(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	makefile := readRepoFile(t, root, "Makefile")
	for _, entry := range []struct {
		gate   string
		target string
		marker string
	}{
		{"build", "build-cli", "cd $(CLI_DIR) && GOWORK=off go build"},
		{"test", "test", "cd $(CLI_DIR) && GOWORK=off go test ./..."},
		{"vet", "vet", "cd $(CLI_DIR) && GOWORK=off go vet ./..."},
		{"lint", "lint", "cd $(CLI_DIR) && GOWORK=off $(GOLANGCI) run ./..."},
	} {
		if !strings.Contains(makefile, entry.marker) {
			t.Errorf("Makefile %s target does not cover clients/cli (looked for %q)",
				entry.gate, entry.marker)
		}
	}
	// build-all must depend on build-cli, or `make build-all` silently stops
	// being a whole-repository build.
	if !strings.Contains(makefile, "build-all: build build-orchestrator build-claude-app-server build-server build-web build-cli") {
		t.Error("build-all does not depend on build-cli")
	}

	profile := prProfileCommands(t, root)
	for _, required := range []string{"cli-vet", "cli-test", "cli-module-boundary"} {
		command, present := profile[required]
		if !present {
			t.Errorf("the pull-request profile has no %q command", required)
			continue
		}
		if !strings.Contains(command, "clients/cli") {
			t.Errorf("profile command %q does not run in clients/cli: %s", required, command)
		}
	}
	// The boundary check is the one that must actually fail on a violation, so
	// it is asserted to be the dependency-graph walk rather than any command
	// that merely mentions the module.
	if boundary := profile["cli-module-boundary"]; !strings.Contains(boundary, "go list -deps") {
		t.Errorf("cli-module-boundary must walk the real dependency graph, got: %s", boundary)
	}
}

// TestCLIModuleHasNoServicesRequirement is the compile-time half of the same
// boundary: `go list -deps` catches an edge that exists, and this catches one
// declared in go.mod before any code imports it.
func TestCLIModuleHasNoServicesRequirement(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	gomod := readRepoFile(t, root, filepath.Join("clients", "cli", "go.mod"))
	for line := range strings.SplitSeq(gomod, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(trimmed, "github.com/takezoh/agent-grid/services") {
			t.Errorf("clients/cli/go.mod declares an edge to the services module: %s", trimmed)
		}
	}
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

// prProfileCommands returns the pull-request profile's commands keyed by id.
func prProfileCommands(t *testing.T, root string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "test-harness", "profiles.json"))
	if err != nil {
		t.Fatalf("read profiles.json: %v", err)
	}
	var manifest struct {
		Profiles []struct {
			Name     string `json:"name"`
			Commands []struct {
				ID      string `json:"id"`
				Command string `json:"command"`
			} `json:"commands"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode profiles.json: %v", err)
	}
	for _, profile := range manifest.Profiles {
		if profile.Name != "pr" {
			continue
		}
		commands := make(map[string]string, len(profile.Commands))
		for _, command := range profile.Commands {
			commands[command.ID] = command.Command
		}
		return commands
	}
	t.Fatal("profiles.json has no pull-request profile")
	return nil
}
