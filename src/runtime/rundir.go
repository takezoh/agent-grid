package runtime

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectRunDir returns the per-project ephemeral run directory path.
// The directory is bind-mounted into the devcontainer at /opt/roost/run.
// Its inode is stable across daemon restarts; only the files inside change.
func ProjectRunDir(runBase, projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return filepath.Join(runBase, fmt.Sprintf("%x", h[:6]))
}

// EnsureProjectRunDir creates the per-project run dir and hardlinks centralSockPath
// into it as roost.sock. Returns the run dir path.
// Called once per EnsureInstance to refresh the roost.sock link after daemon restart.
func EnsureProjectRunDir(runBase, projectPath, centralSockPath string) (string, error) {
	dir := ProjectRunDir(runBase, projectPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("rundir: mkdir %s: %w", dir, err)
	}
	dst := filepath.Join(dir, "roost.sock")
	_ = os.Remove(dst)
	if err := os.Link(centralSockPath, dst); err != nil {
		return "", fmt.Errorf("rundir: link roost.sock: %w", err)
	}
	return dir, nil
}
