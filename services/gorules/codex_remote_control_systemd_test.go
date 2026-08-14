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

	// The pairing guidance is asserted where it is written, not where it once
	// was. baf6a20a moved it out of README.md into a dedicated note, and this
	// check kept naming README.md — so it failed while the guidance was intact
	// and would have passed if the note were deleted. Both halves are needed:
	// the note must carry the guidance, and the README must still reach it,
	// because guidance nothing links to is guidance nobody finds.
	const guide = "docs/note/note-20260715-user-codex-remote-control.md"

	readme := readText(t, filepath.Join(root, "README.md"))
	if !strings.Contains(readme, guide) {
		t.Errorf("README no longer links to %s", guide)
	}

	note := readText(t, filepath.Join(root, filepath.FromSlash(guide)))
	for _, want := range []string{
		"make install-codex-remote-control-systemd",
		"codex remote-control pair",
		"再ペアリングが不要な場合",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("%s missing pairing guidance %q", guide, want)
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
