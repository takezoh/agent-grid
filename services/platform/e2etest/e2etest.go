//go:build e2e

package e2etest

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// NewWorkspaceTempDir creates a temporary directory under the current
// workspace rather than the system temp directory. Current Codex rejects some
// helper-binary bootstrap operations under temporary directories, so real
// Codex E2E needs writable non-/tmp paths for HOME and unix sockets.
func NewWorkspaceTempDir(t *testing.T, prefix string) string {
	t.Helper()
	base, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd for workspace temp dir: %v", err)
	}
	dir, err := os.MkdirTemp(base, prefix)
	if err != nil {
		t.Fatalf("mkdir workspace temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// NewIsolatedHome creates a fresh HOME under the current workspace rather than
// the system temp directory. Tests that must not inherit user config should
// use this rather than cloning dot-directories from the developer's real
// home.
func NewIsolatedHome(t *testing.T, prefix string) string {
	t.Helper()
	return NewWorkspaceTempDir(t, prefix)
}

type copySpec struct {
	relPath  string
	required bool
}

var codexBootstrapSpec = []copySpec{
	{relPath: filepath.Join(".codex", "auth.json"), required: true},
	{relPath: filepath.Join(".codex", "config.toml"), required: false},
	{relPath: filepath.Join(".codex", "installation_id"), required: false},
	{relPath: filepath.Join(".codex", "version.json"), required: false},
	{relPath: filepath.Join(".codex", "hooks.json"), required: false},
}

// PrepareCodexHome creates a fresh isolated HOME and seeds
// it with the minimum Codex bootstrap files required by real-Codex E2E tests.
// If the developer machine is not prepared for authenticated Codex runs, the
// caller is skipped instead of failing later with environment-dependent errors.
func PrepareCodexHome(t *testing.T, prefix string) string {
	t.Helper()
	home := NewIsolatedHome(t, prefix)
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("real-Codex e2e requires a readable user home: %v", err)
	}

	for _, spec := range codexBootstrapSpec {
		src := filepath.Join(realHome, spec.relPath)
		dst := filepath.Join(home, spec.relPath)
		if _, statErr := os.Stat(src); statErr != nil {
			if spec.required && os.IsNotExist(statErr) {
				t.Skipf("real-Codex e2e requires %s in the developer home", spec.relPath)
			}
			if spec.required {
				t.Fatalf("stat %s: %v", src, statErr)
			}
			continue
		}
		if copyErr := copyFileOrSymlink(src, dst); copyErr != nil {
			t.Fatalf("copy %s -> %s: %v", src, dst, copyErr)
		}
	}
	return home
}

// WaitForUnixSocketReady waits until a server has bound and accepted at least
// one connection on the given UDS path.
func WaitForUnixSocketReady(t *testing.T, sock string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(sock); statErr == nil {
			conn, dialErr := net.DialTimeout("unix", sock, 200*time.Millisecond)
			if dialErr == nil {
				_ = conn.Close()
				return
			}
			lastErr = dialErr
		} else {
			lastErr = statErr
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("unix socket %q never became ready within %s: %v", sock, timeout, lastErr)
}

func copyFileOrSymlink(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
