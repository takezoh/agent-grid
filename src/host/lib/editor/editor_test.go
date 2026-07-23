package editor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/host/lib/editor"
)

var defaultExts = []string{".code-workspace"}

func TestResolveTarget_noWorkspace(t *testing.T) {
	dir := t.TempDir()
	if got := editor.ResolveTarget(dir, defaultExts); got != dir {
		t.Errorf("ResolveTarget = %q, want dir %q", got, dir)
	}
}

func TestResolveTarget_singleWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "foo.code-workspace")
	if err := os.WriteFile(ws, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := editor.ResolveTarget(dir, defaultExts); got != ws {
		t.Errorf("ResolveTarget = %q, want %q", got, ws)
	}
}

func TestResolveTarget_multipleWorkspaces(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"beta.code-workspace", "alpha.code-workspace"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(dir, "alpha.code-workspace")
	if got := editor.ResolveTarget(dir, defaultExts); got != want {
		t.Errorf("ResolveTarget = %q, want %q (lexicographic first)", got, want)
	}
}

func TestResolveTarget_pathWithGlobMetachars(t *testing.T) {
	// Directory names that contain glob metacharacters must not cause
	// ResolveTarget to scan a sibling directory instead of the project dir.
	parent := t.TempDir()
	projectDir := filepath.Join(parent, "proj[1]")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(projectDir, "real.code-workspace")
	if err := os.WriteFile(ws, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(parent, "proj1")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "decoy.code-workspace"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := editor.ResolveTarget(projectDir, defaultExts)
	if got != ws {
		t.Errorf("ResolveTarget = %q, want real workspace %q", got, ws)
	}
}

func TestLaunch_emptyCommand(t *testing.T) {
	if err := editor.Launch("", "/some/path"); err == nil {
		t.Error("Launch with empty command should return error")
	}
}

func TestLaunch_noopCommand(t *testing.T) {
	if err := editor.Launch("true", "/some/path"); err != nil {
		t.Errorf("Launch(true) unexpected error: %v", err)
	}
}
