package agentlaunch

import (
	"fmt"
	"path/filepath"
)

// RepositoryLaunchScope binds a repository environment mount to the sessions
// root while each frame uses an independent session clone as cwd.
type RepositoryLaunchScope struct {
	SessionsRoot string
	ClonePath    string
}

// ValidateRepositoryLaunchScope enforces FR-GH-013 / contract-repository-devcontainer-scope:
// mount scope is sessions root only; frame cwd is the session clone under that root.
func ValidateRepositoryLaunchScope(s RepositoryLaunchScope) error {
	if s.SessionsRoot == "" {
		return fmt.Errorf("repository scope: sessions root required")
	}
	if s.ClonePath == "" {
		return fmt.Errorf("repository scope: clone path required")
	}
	root := filepath.Clean(s.SessionsRoot)
	clone := filepath.Clean(s.ClonePath)
	if root == clone {
		return fmt.Errorf("repository scope: clone path must not equal sessions root")
	}
	rel, err := filepath.Rel(root, clone)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 2 && rel[:2] == ".." {
		return fmt.Errorf("repository scope: clone path must be under sessions root")
	}
	return nil
}

// RepositoryMountHostPath returns the host path that must be bind-mounted for
// a repository environment (sessions root, never a single clone).
func RepositoryMountHostPath(s RepositoryLaunchScope) (string, error) {
	if err := ValidateRepositoryLaunchScope(s); err != nil {
		return "", err
	}
	return filepath.Clean(s.SessionsRoot), nil
}

// RepositoryFrameCWD returns the frame working directory (session clone).
func RepositoryFrameCWD(s RepositoryLaunchScope) (string, error) {
	if err := ValidateRepositoryLaunchScope(s); err != nil {
		return "", err
	}
	return filepath.Clean(s.ClonePath), nil
}
