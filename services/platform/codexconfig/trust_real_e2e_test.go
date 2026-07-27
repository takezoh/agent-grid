//go:build e2e

package codexconfig

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFakeVsRealCodexConfigPath checks ConfigPath's fallback against the real
// Codex CLI. A malformed file must be loaded from HOME/.codex/config.toml.
func TestFakeVsRealCodexConfigPath(t *testing.T) {
	bin := strings.TrimSpace(os.Getenv("AG_E2E_CODEX_BIN"))
	if bin == "" {
		t.Skip("AG_E2E_CODEX_BIN is not set")
	}
	home := t.TempDir()
	path := ConfigPath("", home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "exec", "config-path-probe")
	cmd.Env = append(envWithout(os.Environ(), "CODEX_HOME", "HOME"), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), path) || !strings.Contains(string(out), "Error loading config.toml") {
		t.Fatalf("real Codex did not load %q: err=%v output=%s", path, err, out)
	}
}

func envWithout(env []string, keys ...string) []string {
	drop := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		drop[key] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if _, ok := drop[key]; !ok {
			out = append(out, item)
		}
	}
	return out
}
