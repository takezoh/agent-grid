package gorules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexRemoteControlSystemdContract(t *testing.T) {
	root := repoRoot(t)

	unit := readText(t, filepath.Join(root, "deploy", "systemd", "codex-remote-control.service"))
	for _, want := range []string{
		"Type=oneshot",
		"app-server daemon bootstrap --remote-control",
		"app-server daemon stop",
		"RemainAfterExit=yes",
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("remote-control unit missing %q", want)
		}
	}
	if strings.Contains(unit, "remote-control pair") {
		t.Error("pairing must remain a manual operation, not part of the systemd unit")
	}

	makefile := readText(t, filepath.Join(root, "Makefile"))
	for _, want := range []string{
		"install-codex-remote-control-systemd:",
		"deploy/systemd/codex-remote-control.service",
		"systemctl --user enable --now codex-remote-control.service",
	} {
		if !strings.Contains(makefile, want) {
			t.Errorf("Makefile missing %q", want)
		}
	}

	readme := readText(t, filepath.Join(root, "README.md"))
	for _, want := range []string{
		"make install-codex-remote-control-systemd",
		"codex remote-control pair",
		"再ペアリングは不要です",
		"devcontainer ごとの daemon 起動やペアリングは不要です",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README missing pairing guidance %q", want)
		}
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
