package runtime

import (
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/state"
)

func TestResolveWorkspaceRoot_Priority(t *testing.T) {
	t.Parallel()
	project := "/tmp/project"
	worktreeDir := "/tmp/project/.agent-grid/worktrees/wt-1"
	startDir := "/tmp/project/subdir"

	sess := state.Session{Project: project}
	head := state.SessionFrame{Project: project}

	got := ResolveWorkspaceRoot(head, sess)
	want := filepath.Clean(project)
	if got != want {
		t.Fatalf("project fallback = %q, want %q", got, want)
	}

	head.Driver = driver.ClaudeState{
		CommonState: driver.CommonState{StartDir: startDir},
	}
	got = ResolveWorkspaceRoot(head, sess)
	want = filepath.Clean(startDir)
	if got != want {
		t.Fatalf("StartDir = %q, want %q", got, want)
	}

	head.Driver = driver.ClaudeState{
		CommonState: driver.CommonState{
			StartDir:     worktreeDir,
			WorktreeName: "wt-1",
		},
	}
	got = ResolveWorkspaceRoot(head, sess)
	want = filepath.Clean(worktreeDir)
	if got != want {
		t.Fatalf("worktree StartDir = %q, want %q", got, want)
	}
}
