package runtime

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	roostgit "github.com/takezoh/agent-roost/lib/git"
	"github.com/takezoh/agent-roost/state"
)

// CleanupUntrackedWorktrees removes managed worktrees under each project's
// .roost/worktrees/ that are not referenced by any session in the current state.
func (r *Runtime) CleanupUntrackedWorktrees(ctx context.Context, projects []string) {
	untracked := collectUntrackedWorktrees(r.state.Sessions, projects, osListWorktreeDirs)
	for _, path := range untracked {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := roostgit.RemoveWorktree(c, path); err != nil {
			slog.Warn("runtime: cleanup untracked worktree failed", "path", path, "err", err)
		}
		cancel()
	}
}

// collectUntrackedWorktrees returns paths under <project>/.roost/worktrees/
// not tracked by any session frame. listDir receives a directory path and
// returns the full paths of its subdirectories.
func collectUntrackedWorktrees(
	sessions map[state.SessionID]state.Session,
	projects []string,
	listDir func(string) ([]string, error),
) []string {
	tracked := make(map[string]struct{})
	for _, sess := range sessions {
		for _, frame := range sess.Frames {
			drv := state.GetDriver(frame.Command)
			if provider, ok := drv.(state.ManagedWorktreeProvider); ok {
				if path := provider.ManagedWorktreePath(frame.Driver); path != "" {
					tracked[filepath.Clean(path)] = struct{}{}
				}
			}
		}
	}

	var untracked []string
	for _, project := range projects {
		worktreesDir := filepath.Join(project, ".roost", "worktrees")
		entries, err := listDir(worktreesDir)
		if err != nil {
			continue
		}
		for _, path := range entries {
			if _, ok := tracked[filepath.Clean(path)]; !ok {
				untracked = append(untracked, path)
			}
		}
	}
	return untracked
}

func osListWorktreeDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}
