package clientconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-config-and-fleet-identity
// (adr-20260814-shared-go-client-config-module).

// TestConfigDirectoryMatchesExistingHostsPerPlatform is half of
// ver-config-never-written-and-directory-per-platform.
//
// The expectations below are transcribed from the two graphical hosts, which
// were resolving the directory before this loader existed:
// clients/workspace/src/main/desktop-config.ts defaultConfigDirectory and
// clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs
// DefaultConfigDirectory.
//
// This fails an implementation that keeps the missing Windows branch, which
// would make a Windows CLI read a different directory than the Shell and
// Workspace on the same machine — and therefore report a different fleet for
// the same operator.
func TestConfigDirectoryMatchesExistingHostsPerPlatform(t *testing.T) {
	t.Parallel()

	const home = "/home/operator"
	const windowsHome = `C:\Users\operator`

	cases := []struct {
		name string
		goos string
		env  map[string]string
		home string
		want string
	}{
		{
			name: "windows uses the roaming application data directory",
			goos: "windows",
			env:  map[string]string{"APPDATA": `C:\Users\operator\AppData\Roaming`},
			home: windowsHome,
			want: filepath.Join(`C:\Users\operator\AppData\Roaming`, "agent-grid", "config"),
		},
		{
			name: "windows falls back to the profile when APPDATA is unset",
			goos: "windows",
			env:  map[string]string{},
			home: windowsHome,
			want: filepath.Join(windowsHome, "AppData", "Roaming", "agent-grid", "config"),
		},
		{
			name: "linux honours XDG_CONFIG_HOME",
			goos: "linux",
			env:  map[string]string{"XDG_CONFIG_HOME": "/xdg"},
			home: home,
			want: filepath.Join("/xdg", "agent-grid"),
		},
		{
			name: "linux falls back to the home config directory",
			goos: "linux",
			env:  map[string]string{},
			home: home,
			want: filepath.Join(home, ".config", "agent-grid"),
		},
		{
			name: "darwin resolves like every other posix host",
			goos: "darwin",
			env:  map[string]string{},
			home: home,
			want: filepath.Join(home, ".config", "agent-grid"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := directoryFor(
				tc.goos,
				func(key string) string { return tc.env[key] },
				func() (string, error) { return tc.home, nil },
			)
			if err != nil {
				t.Fatalf("resolve directory: %v", err)
			}
			if got != tc.want {
				t.Errorf("directory = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestConfigDirectoryIsNeverWritten is the other half of
// ver-config-never-written-and-directory-per-platform.
//
// A configuration directory belongs to the operator. A loader that repairs,
// creates or rewrites what it reads makes the same document mean different
// things depending on which host opened it last — and the CLI, which can be
// run repeatedly from scripts, would be the host doing the rewriting.
//
// Both halves are checked: that a load leaves the directory byte-identical,
// and that the package holds no write primitive at all, since a write reached
// only on a path this test does not drive would still be a write.
func TestConfigDirectoryIsNeverWritten(t *testing.T) {
	t.Parallel()

	t.Run("a load leaves the directory exactly as it found it", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		document := `{"schema_version":2,"servers":[{"display_name":"A","enabled":true,` +
			`"url":"http://127.0.0.1:8765","auth":{"mode":"none"}}]}`
		path := filepath.Join(dir, "servers.json")
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := Load(dir); err != nil {
			t.Fatalf("load: %v", err)
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != document {
			t.Errorf("servers.json was rewritten:\n got %s\nwant %s", after, document)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Errorf("directory gained entries: %v", names(entries))
		}
	})

	t.Run("a missing configuration is reported, not created", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "absent")
		if _, err := Load(dir); err == nil {
			t.Fatal("expected a missing configuration to be reported")
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("loader created the configuration directory: %v", err)
		}
	})

	t.Run("the package holds no write primitive", func(t *testing.T) {
		t.Parallel()
		forbidden := []string{
			"os.WriteFile(", "os.Create(", "os.MkdirAll(", "os.Mkdir(",
			"os.Remove(", "os.Rename(", "os.Chmod(", "os.OpenFile(",
		}
		for _, file := range packageSourceFiles(t) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, primitive := range forbidden {
				if strings.Contains(string(raw), primitive) {
					t.Errorf("%s uses %s — the loader must never write a configuration directory",
						filepath.Base(file), primitive)
				}
			}
		}
	})
}

// packageSourceFiles lists the non-test sources of this package. Tests write
// into their own temporary directories, which is not the behaviour under test.
func packageSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		t.Fatal("no package sources found")
	}
	return files
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
