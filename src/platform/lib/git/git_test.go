package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestDetectRemoteHost_SSH(t *testing.T) {
	dir := initRepo(t)
	gitRun(t, dir, "remote", "add", "origin", "git@github.com:user/repo.git")

	got := DetectRemoteHost(context.Background(), dir)
	if got != "github.com" {
		t.Errorf("DetectRemoteHost = %q, want %q", got, "github.com")
	}
}

func TestDetectRemoteHost_HTTPS(t *testing.T) {
	dir := initRepo(t)
	gitRun(t, dir, "remote", "add", "origin", "https://gitlab.com/user/repo.git")

	got := DetectRemoteHost(context.Background(), dir)
	if got != "gitlab.com" {
		t.Errorf("DetectRemoteHost = %q, want %q", got, "gitlab.com")
	}
}

func TestDetectRemoteHost_NoRemote(t *testing.T) {
	dir := initRepo(t)

	got := DetectRemoteHost(context.Background(), dir)
	if got != "" {
		t.Errorf("DetectRemoteHost = %q, want empty", got)
	}
}

func TestDetectRemoteHost_NotGitDir(t *testing.T) {
	dir := t.TempDir()

	got := DetectRemoteHost(context.Background(), dir)
	if got != "" {
		t.Errorf("DetectRemoteHost = %q, want empty", got)
	}
}

func TestIsWorktree_MainRepo(t *testing.T) {
	dir := initRepo(t)
	if IsWorktree(dir) {
		t.Error("IsWorktree = true for main repo, want false")
	}
}

func TestIsWorktree_LinkedWorktree(t *testing.T) {
	dir := initRepo(t)
	gitRun(t, dir, "branch", "feature")
	wtDir := t.TempDir()
	gitRun(t, dir, "worktree", "add", wtDir, "feature")

	if !IsWorktree(wtDir) {
		t.Error("IsWorktree = false for linked worktree, want true")
	}
}

func TestIsRepo_GitDir(t *testing.T) {
	dir := initRepo(t)
	if !IsRepo(dir) {
		t.Error("IsRepo = false for git repo, want true")
	}
}

func TestIsRepo_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		// t.TempDir() often lands under /tmp (when /tmp/.git exists) or inside
		// the checkout. Fall back to a hidden sibling of this package's repo root.
		dir = isolatedNonGitDir(t)
	}
	if IsRepo(dir) {
		t.Errorf("IsRepo = true for non-git dir %q, want false", dir)
	}
}

func isolatedNonGitDir(t *testing.T) string {
	t.Helper()
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		if root := findGitRoot(wd); root != "" {
			parent := filepath.Dir(root)
			if parent != root {
				candidates = append(candidates, filepath.Join(parent, fmt.Sprintf(".agent-grid-non-git-%d", os.Getpid())))
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, fmt.Sprintf(".agent-grid-non-git-%d", os.Getpid())))
	}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		if !IsRepo(dir) {
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
		_ = os.RemoveAll(dir)
	}
	t.Skip("unable to create a writable directory outside all git worktrees")
	return ""
}

func TestIsRepo_NonExistent(t *testing.T) {
	if IsRepo("/no/such/path/xyz") {
		t.Error("IsRepo = true for nonexistent path, want false")
	}
}

func TestIsRepo_SubdirOfGitRepo(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(sub) {
		t.Error("IsRepo = false for subdir inside git repo, want true")
	}
}

func TestDetectMainBranch_LinkedWorktree(t *testing.T) {
	dir := initRepo(t)
	gitRun(t, dir, "branch", "feature")
	wtDir := t.TempDir()
	gitRun(t, dir, "worktree", "add", wtDir, "feature")

	got := DetectMainBranch(context.Background(), wtDir)
	if got != "main" {
		t.Errorf("DetectMainBranch = %q, want %q", got, "main")
	}
}

func TestDetectMainBranch_NoGit(t *testing.T) {
	dir := t.TempDir()
	got := DetectMainBranch(context.Background(), dir)
	if got != "" {
		t.Errorf("DetectMainBranch = %q, want empty", got)
	}
}

func TestParseMainBranch(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "normal",
			output: "worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /tmp/wt\nHEAD def456\nbranch refs/heads/feature\n\n",
			want:   "main",
		},
		{
			name:   "detached HEAD in main",
			output: "worktree /home/user/repo\nHEAD abc123\ndetached\n\nworktree /tmp/wt\nHEAD def456\nbranch refs/heads/feature\n\n",
			want:   "",
		},
		{
			name:   "empty",
			output: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseMainBranch(tt.output); got != tt.want {
				t.Errorf("parseMainBranch = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHost(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"git@github.com:user/repo.git", "github.com"},
		{"ssh://git@github.com/user/repo.git", "github.com"},
		{"https://gitlab.com/user/repo.git", "gitlab.com"},
		{"https://bitbucket.org/user/repo.git", "bitbucket.org"},
		{"git@GitHub.COM:user/repo.git", "github.com"},
		{"/local/path/repo.git", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := parseHost(tt.raw); got != tt.want {
			t.Errorf("parseHost(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestRepoRoot(t *testing.T) {
	dir := initRepo(t)
	root, err := RepoRoot(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoRoot error: %v", err)
	}
	if root != dir {
		t.Fatalf("RepoRoot = %q, want %q", root, dir)
	}
}

func TestDiffHeadVsWorktree_Outcomes(t *testing.T) {
	ctx := context.Background()

	nonGit := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonGit, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := DiffHeadVsWorktree(ctx, nonGit, "f.txt"); got.Outcome != DiffHeadNotARepo {
		t.Fatalf("non-git outcome = %q", got.Outcome)
	}

	corrupt := t.TempDir()
	if err := os.Mkdir(filepath.Join(corrupt, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, ".git", "HEAD"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DiffHeadVsWorktree(ctx, corrupt, ""); got.Outcome != DiffHeadGitMetadataCorrupt {
		t.Fatalf("corrupt outcome = %q", got.Outcome)
	}

	if _, err := exec.LookPath("git"); err != nil {
		if got := DiffHeadVsWorktree(ctx, t.TempDir(), ""); got.Outcome != DiffHeadGitBinaryMissing {
			t.Fatalf("missing binary outcome = %q", got.Outcome)
		}
		return
	}

	dir := initRepo(t)
	path := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "tracked.txt")
	gitRun(t, dir, "commit", "-m", "add tracked")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := DiffHeadVsWorktree(ctx, dir, "tracked.txt")
	if got.Outcome != DiffHeadOK {
		t.Fatalf("ok outcome = %q", got.Outcome)
	}
	if got.Diff == "" {
		t.Fatal("expected non-empty diff")
	}
}

func TestCreateWorktree(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	wtDir, err := CreateWorktree(ctx, dir, "feature-test")
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}
	want := filepath.Join(dir, ".agent-grid", "worktrees", "feature-test")
	if wtDir != want {
		t.Fatalf("CreateWorktree path = %q, want %q", wtDir, want)
	}
	if !IsWorktree(wtDir) {
		t.Fatal("created path is not a linked worktree")
	}
	if got := DetectBranch(ctx, wtDir); got != "feature-test" {
		t.Fatalf("branch = %q, want %q", got, "feature-test")
	}
}

func TestCreateWorktreeRejectsNonGitDir(t *testing.T) {
	if _, err := CreateWorktree(context.Background(), t.TempDir(), "feature-test"); err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	wtDir, err := CreateWorktree(ctx, dir, "feature-test")
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}
	if err := RemoveWorktree(ctx, wtDir); err != nil {
		t.Fatalf("RemoveWorktree error: %v", err)
	}
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists after remove: %v", err)
	}
}

func TestRemoveWorktreeRejectsUnmanagedPath(t *testing.T) {
	dir := initRepo(t)
	if err := RemoveWorktree(context.Background(), dir); err == nil {
		t.Fatal("expected error for unmanaged path")
	}
}

func TestFindGitRoot(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "sub")
	subsub := filepath.Join(sub, "subsub")
	if err := os.MkdirAll(subsub, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"repo root itself", dir, dir},
		{"direct child", sub, dir},
		{"grandchild", subsub, dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findGitRoot(tt.dir); got != tt.want {
				t.Errorf("findGitRoot(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestDetectBranch_FromSubdirectory(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "src", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DetectBranch(context.Background(), sub); got != "main" {
		t.Errorf("DetectBranch(subdir) = %q, want %q", got, "main")
	}
}

func TestIsWorktree_SubdirectoryOfLinkedWorktree(t *testing.T) {
	dir := initRepo(t)
	gitRun(t, dir, "branch", "feature")
	wtDir := t.TempDir()
	gitRun(t, dir, "worktree", "add", wtDir, "feature")
	sub := filepath.Join(wtDir, "deep", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsWorktree(sub) {
		t.Error("IsWorktree(subdir-of-worktree) = false, want true")
	}
}
