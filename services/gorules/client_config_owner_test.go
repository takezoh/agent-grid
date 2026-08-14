package gorules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-config-and-fleet-identity
// (adr-20260814-shared-go-client-config-module).
//
// This is ver-every-go-host-loads-through-one-owner. `clients/config` declares
// servers.json a shared contract, but a declaration in a README cannot stop a
// second parser appearing — and a second parser is how one server acquires two
// identities and a fleet splits in half without either host noticing.
//
// The check is per language rather than per host: one loader in Go, one in
// TypeScript, one in C#, each bound to the same contract.

// clientConfigLoaders names the one owner in each client language and the
// marker that identifies a parser of the document. A file outside the owner
// that reads servers.json is the failure.
// testMarkers exclude test sources, which name the document because they write
// fixtures for the loader to read. The rule is about production parsers.
var clientConfigLoaders = []struct {
	language    string
	owner       string
	roots       []string
	suffixes    []string
	testMarkers []string
}{
	{
		language:    "Go",
		owner:       "clients/goclient/clientconfig",
		roots:       []string{"clients", "services"},
		suffixes:    []string{".go"},
		testMarkers: []string{"_test.go"},
	},
	{
		language:    "TypeScript",
		owner:       "clients/workspace/src/main/desktop-config.ts",
		roots:       []string{"clients"},
		suffixes:    []string{".ts"},
		testMarkers: []string{".test.ts", ".spec.ts", "/tests/", "/e2e/"},
	},
	{
		language:    "C#",
		owner:       "clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs",
		roots:       []string{"clients"},
		suffixes:    []string{".cs"},
		testMarkers: []string{".Tests/", "UiTests/"},
	},
}

func TestClientConfigHasExactlyOneLoaderPerLanguage(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	for _, loader := range clientConfigLoaders {
		t.Run(loader.language, func(t *testing.T) {
			t.Parallel()

			ownerPath := filepath.Join(root, filepath.FromSlash(loader.owner))
			if _, err := os.Stat(ownerPath); err != nil {
				t.Fatalf("the declared %s owner of servers.json is missing: %v", loader.language, err)
			}

			offenders := []string{}
			for _, dir := range loader.roots {
				offenders = append(offenders,
					findServersJSONParsers(t, root, dir, loader.owner, loader.suffixes, loader.testMarkers)...)
			}
			if len(offenders) > 0 {
				t.Errorf("servers.json is parsed outside its %s owner %s: %v\n"+
					"a second parser is how one server acquires two identities; import the owner instead",
					loader.language, loader.owner, offenders)
			}
		})
	}
}

// findServersJSONParsers walks dir for sources that read the document by name.
// Naming the file is the discriminating signal: a consumer that imports the
// loader never mentions it, and a parser always does.
func findServersJSONParsers(t *testing.T, root, dir, owner string, suffixes, testMarkers []string) []string {
	t.Helper()
	offenders := []string{}
	base := filepath.Join(root, filepath.FromSlash(dir))
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipDirForLoaderScan(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		slashed := filepath.ToSlash(rel)
		if strings.HasPrefix(slashed, owner) {
			return nil
		}
		if !hasAnySuffix(slashed, suffixes) || containsAny(slashed, testMarkers) {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(raw), `"servers.json"`) {
			offenders = append(offenders, slashed)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return offenders
}

// skipDirForLoaderScan drops build output and vendored trees, which hold
// compiled copies of the very sources this check is reading.
func skipDirForLoaderScan(name string) bool {
	switch name {
	case "node_modules", "dist", "bin", "obj", ".git", ".worktrees", "testdata", "generated":
		return true
	}
	return false
}

func hasAnySuffix(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func containsAny(name string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}
