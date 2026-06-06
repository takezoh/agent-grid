package devcontainer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-roost/platform/sandbox"
)

const devcontainerSubdir = ".devcontainer"

// ErrNoProjectDevcontainer is returned when <project>/.devcontainer/devcontainer.json is not found.
var ErrNoProjectDevcontainer = errors.New("devcontainer: <project>/.devcontainer/devcontainer.json not found")

// ErrNoUserDevcontainer is returned when ~/.devcontainer/devcontainer.json is not found.
var ErrNoUserDevcontainer = errors.New("devcontainer: ~/.devcontainer/devcontainer.json not found")

// OverlayFunc computes roost overlay (env + mounts) to apply at container creation time.
// plan carries the shared-vs-project decision; the overlay derives the container
// key, overlay project, and workspace fallback from it via plan.ContainerKey /
// OverlayProject / WorkspaceFallbackProject (all keyed by projectPath, the actual
// project launching the frame). dcDir is the resolved devcontainer config
// directory. Must not trigger image builds.
type OverlayFunc func(plan sandbox.IsolationPlan, projectPath, dcDir string) (SpecOverlay, error)

// FindDevcontainerPath returns the devcontainer.json path for projectPath.
// If override is non-empty it is used directly and must contain devcontainer.json
// (no fallback — an explicit path that doesn't exist is always an error).
// Otherwise tries <project>/.devcontainer then ~/.devcontainer.
func FindDevcontainerPath(projectPath, override string) (string, error) {
	if override != "" {
		p := filepath.Join(override, "devcontainer.json")
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("devcontainer: specified path %q: devcontainer.json not found", override)
		}
		return p, nil
	}
	dcPath, err := ProjectBaseDC(projectPath)
	if errors.Is(err, ErrNoProjectDevcontainer) {
		dcPath, err = UserBaseDC()
	}
	return dcPath, err
}

// ProjectBaseDC returns the path to <project>/.devcontainer/devcontainer.json.
// Returns ErrNoProjectDevcontainer if not found.
func ProjectBaseDC(projectPath string) (string, error) {
	return findDC(projectPath, ErrNoProjectDevcontainer)
}

// UserBaseDC returns the path to ~/.devcontainer/devcontainer.json.
// Returns ErrNoUserDevcontainer if not found.
func UserBaseDC() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoUserDevcontainer
	}
	return findDC(home, ErrNoUserDevcontainer)
}

func findDC(basePath string, notFoundErr error) (string, error) {
	p := filepath.Join(basePath, devcontainerSubdir, "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", notFoundErr
	}
	return p, nil
}
