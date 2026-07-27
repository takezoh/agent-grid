package gorules_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallTargetsPlaceBridgeAtLibexecRoot(t *testing.T) {
	for _, tc := range []struct {
		target     string
		serverName string
	}{
		{target: "install", serverName: "server"},
		{target: "install-server", serverName: "agent-grid-server"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			root := repoRoot(t)
			home := t.TempDir()
			server := writeExecutable(t, filepath.Join(t.TempDir(), "server"))
			bridge := writeExecutable(t, filepath.Join(t.TempDir(), "bridge"))

			cmd := exec.Command(
				"make",
				"--no-print-directory",
				"-o", "build",
				tc.target,
				"HOME="+home,
				"SERVER="+server,
				"BRIDGE="+bridge,
			)
			cmd.Dir = root
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s failed: %v\n%s", tc.target, err, output)
			}

			assertExecutable(t, filepath.Join(home, ".local", "bin", tc.serverName))
			assertExecutable(t, filepath.Join(home, ".local", "lib", "agent-grid", "bridge"))
		})
	}
}

func writeExecutable(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
	return path
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat installed executable %s: %v", path, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("installed executable %s mode = %o, want 755", path, info.Mode().Perm())
	}
}
