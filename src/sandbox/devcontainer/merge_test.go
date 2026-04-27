package devcontainer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupProjectDC creates a temp project dir with a .devcontainer/devcontainer.json.
func setupProjectDC(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	dcDir := filepath.Join(dir, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setupUserDC creates a fake home with ~/.devcontainer/devcontainer.json.
func setupUserDC(t *testing.T, content string, extraFiles map[string]string) string {
	t.Helper()
	fakeHome := t.TempDir()
	dcDir := filepath.Join(fakeHome, ".devcontainer")
	os.MkdirAll(dcDir, 0o755)
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extraFiles {
		if err := os.WriteFile(filepath.Join(dcDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return fakeHome
}

// ── ProjectBaseDC ─────────────────────────────────────────────────────────────

func TestProjectBaseDC_found(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu"}`)
	got, err := ProjectBaseDC(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(project, ".devcontainer", "devcontainer.json")
	if got != want {
		t.Errorf("basePath = %q, want %q", got, want)
	}
}

func TestProjectBaseDC_notFound(t *testing.T) {
	project := t.TempDir()
	_, err := ProjectBaseDC(project)
	if !errors.Is(err, ErrNoProjectDevcontainer) {
		t.Errorf("expected ErrNoProjectDevcontainer, got %v", err)
	}
}

// ── UserBaseDC ────────────────────────────────────────────────────────────────

func TestUserBaseDC_found(t *testing.T) {
	fakeHome := setupUserDC(t, `{"image":"default"}`, nil)
	t.Setenv("HOME", fakeHome)

	got, err := UserBaseDC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, fakeHome) {
		t.Errorf("expected path under %s, got %q", fakeHome, got)
	}
}

func TestUserBaseDC_notFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := UserBaseDC()
	if !errors.Is(err, ErrNoUserDevcontainer) {
		t.Errorf("expected ErrNoUserDevcontainer, got %v", err)
	}
}

// ── MaterializeProjectConfig ──────────────────────────────────────────────────

func TestMaterialize_materializeDirUnderRoost(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu"}`)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	configPath, err := MaterializeProjectConfig(project)
	if err != nil {
		t.Fatalf("MaterializeProjectConfig: %v", err)
	}

	want := filepath.Join(fakeHome, ".roost", "projects")
	if !strings.HasPrefix(configPath, want) {
		t.Errorf("configPath %q does not start with %q", configPath, want)
	}
	if filepath.Base(configPath) != "devcontainer.json" {
		t.Errorf("configPath base = %q, want devcontainer.json", filepath.Base(configPath))
	}
}

func TestMaterialize_stableHash(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu"}`)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	p1, err := MaterializeProjectConfig(project)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := MaterializeProjectConfig(project)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Errorf("path not stable: %s vs %s", p1, p2)
	}
}

func TestMaterialize_noOverlayInjected(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu","containerEnv":{"MY_KEY":"base"}}`)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	configPath, err := MaterializeProjectConfig(project)
	if err != nil {
		t.Fatalf("MaterializeProjectConfig: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "ROOST_SOCKET") {
		t.Error("ROOST_SOCKET must not appear in materialized devcontainer.json")
	}
	if strings.Contains(content, "opt/roost/run") {
		t.Error("opt/roost/run mount must not appear in materialized devcontainer.json")
	}
}

func TestMaterialize_projectNotFound(t *testing.T) {
	project := t.TempDir() // no .devcontainer inside
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := MaterializeProjectConfig(project)
	if !errors.Is(err, ErrNoProjectDevcontainer) {
		t.Errorf("expected ErrNoProjectDevcontainer, got %v", err)
	}
}

func TestMaterialize_differentProjectsDifferentDirs(t *testing.T) {
	p1 := setupProjectDC(t, `{"image":"ubuntu"}`)
	p2 := setupProjectDC(t, `{"image":"ubuntu"}`)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	c1, err := MaterializeProjectConfig(p1)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := MaterializeProjectConfig(p2)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(c1) == filepath.Dir(c2) {
		t.Errorf("different projects got same materialize dir: %s", filepath.Dir(c1))
	}
}

// ── MaterializeUserConfig ─────────────────────────────────────────────────────

func TestMaterializeUserConfig_assetsAreCopied(t *testing.T) {
	fakeHome := setupUserDC(t, `{"build":{"dockerfile":"Dockerfile"}}`, map[string]string{
		"Dockerfile":     "FROM ubuntu",
		"post-create.sh": "#!/bin/bash\necho done",
	})
	t.Setenv("HOME", fakeHome)

	workspaceFolder, configPath, err := MaterializeUserConfig()
	if err != nil {
		t.Fatalf("MaterializeUserConfig: %v", err)
	}

	materializeDir := filepath.Dir(configPath)
	for _, f := range []string{"Dockerfile", "post-create.sh"} {
		if _, err := os.Stat(filepath.Join(materializeDir, f)); err != nil {
			t.Errorf("asset %q not copied to materialize dir: %v", f, err)
		}
	}

	wantWS := filepath.Join(fakeHome, ".roost", "user")
	if workspaceFolder != wantWS {
		t.Errorf("workspaceFolder = %q, want %q", workspaceFolder, wantWS)
	}
}

func TestMaterializeUserConfig_notFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, _, err := MaterializeUserConfig()
	if !errors.Is(err, ErrNoUserDevcontainer) {
		t.Errorf("expected ErrNoUserDevcontainer, got %v", err)
	}
}

func TestMaterializeUserConfig_sharedDir(t *testing.T) {
	// User-scope has one materialize dir regardless of project count.
	fakeHome := setupUserDC(t, `{"image":"default"}`, nil)
	t.Setenv("HOME", fakeHome)

	_, c1, err := MaterializeUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, c2, err := MaterializeUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Errorf("user-scope materialize dir should be stable: %s vs %s", c1, c2)
	}
	wantDir := filepath.Join(fakeHome, ".roost", "user", "devcontainer")
	if filepath.Dir(c1) != wantDir {
		t.Errorf("materializeDir = %q, want %q", filepath.Dir(c1), wantDir)
	}
}

// ── TranslateWorkDir ──────────────────────────────────────────────────────────

func TestTranslateWorkDir(t *testing.T) {
	const project = "/home/take/code/myapp"
	const remoteWS = "/workspaces/myapp"

	cases := []struct {
		name    string
		hostDir string
		want    string
	}{
		{"empty → remoteWS", "", remoteWS},
		{"project root → remoteWS", project, remoteWS},
		{"sub-dir", project + "/backend/api", remoteWS + "/backend/api"},
		{"worktree", project + "/.roost/worktree/feat", remoteWS + "/.roost/worktree/feat"},
		{"outside project → remoteWS", "/home/take/other", remoteWS},
		{"dotdot escape → remoteWS", project + "/../other", remoteWS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateWorkDir(tc.hostDir, project, remoteWS)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
