package devcontainer

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveImage_noImages(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found")
	}

	// Use a unique temp path so no roost images can exist for it.
	projectPath := t.TempDir()
	_, _, err := ResolveImage(context.Background(), projectPath)
	if err == nil {
		t.Fatal("expected error when no images exist")
	}
	if !strings.Contains(err.Error(), "roost build") {
		t.Errorf("error should mention 'roost build', got: %v", err)
	}
}

func TestProjectScopeImage_format(t *testing.T) {
	img := ProjectScopeImage("abc123")
	if img != "roost-proj-abc123:latest" {
		t.Errorf("got %q, want %q", img, "roost-proj-abc123:latest")
	}
}

func TestUserScopeImage_format(t *testing.T) {
	img := UserScopeImage()
	if img != "roost-user:latest" {
		t.Errorf("got %q, want %q", img, "roost-user:latest")
	}
}

func TestProjectMaterializeDir_format(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := ProjectMaterializeDir("abc123")
	if !strings.Contains(dir, "projects") || !strings.Contains(dir, "abc123") {
		t.Errorf("unexpected ProjectMaterializeDir: %q", dir)
	}
	if !strings.HasSuffix(dir, "devcontainer") {
		t.Errorf("expected dir to end with 'devcontainer', got: %q", dir)
	}
}

func TestUserMaterializeDir_format(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := UserMaterializeDir()
	wantSuffix := ".roost/user/devcontainer"
	if !strings.HasSuffix(dir, wantSuffix) {
		t.Errorf("UserMaterializeDir = %q, want suffix %q", dir, wantSuffix)
	}
}
