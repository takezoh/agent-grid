package devcontainer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoProjectDevcontainer is returned when <project>/.devcontainer/devcontainer.json is not found.
var ErrNoProjectDevcontainer = errors.New("devcontainer: <project>/.devcontainer/devcontainer.json not found")

// ErrNoUserDevcontainer is returned when ~/.devcontainer/devcontainer.json is not found.
var ErrNoUserDevcontainer = errors.New("devcontainer: ~/.devcontainer/devcontainer.json not found")

// OverlayFunc computes per-project roost overlay (env + mounts) to apply at container
// creation time. materializeDir is the resolved scope's materialize directory.
// Must not trigger image builds.
type OverlayFunc func(projectPath, materializeDir string) (SpecOverlay, error)

// ProjectBaseDC returns the path to <project>/.devcontainer/devcontainer.json.
// Returns ErrNoProjectDevcontainer if not found.
func ProjectBaseDC(projectPath string) (string, error) {
	p := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", ErrNoProjectDevcontainer
	}
	return p, nil
}

// UserBaseDC returns the path to ~/.devcontainer/devcontainer.json.
// Returns ErrNoUserDevcontainer if not found.
func UserBaseDC() (string, error) {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", ErrNoUserDevcontainer
	}
	return p, nil
}

// MaterializeProjectConfig copies the project .devcontainer config and sibling assets
// into the project-scoped roost-managed directory so "devcontainer build --config" can
// resolve relative Dockerfile / context paths.
//
// The materialize dir is ~/.roost/projects/<projectHash>/devcontainer/.
// Returns the path to devcontainer.json inside that dir.
// No roost overlay (env/mounts) is injected here; overlay is applied at container
// creation time via DevcontainerSpec.Apply.
func MaterializeProjectConfig(projectPath string) (string, error) {
	basePath, err := ProjectBaseDC(projectPath)
	if err != nil {
		return "", err
	}

	hash := projectHash(projectPath)
	materializeDir := ProjectMaterializeDir(hash)
	if err := os.MkdirAll(materializeDir, 0o755); err != nil {
		return "", fmt.Errorf("devcontainer: mkdir %s: %w", materializeDir, err)
	}

	if err := copyDirFiles(filepath.Dir(basePath), materializeDir); err != nil {
		return "", fmt.Errorf("devcontainer: copy assets from %s: %w", filepath.Dir(basePath), err)
	}

	return filepath.Join(materializeDir, "devcontainer.json"), nil
}

// MaterializeUserConfig copies ~/.devcontainer config and sibling assets into the
// user-scoped materialize dir (~/.roost/user/devcontainer/).
// Returns (workspaceFolder, configPath, error).
// workspaceFolder is the --workspace-folder arg for "devcontainer build".
func MaterializeUserConfig() (workspaceFolder, configPath string, err error) {
	basePath, err := UserBaseDC()
	if err != nil {
		return "", "", err
	}

	home, _ := os.UserHomeDir()
	workspaceFolder = filepath.Join(home, ".roost", "user")
	materializeDir := filepath.Join(workspaceFolder, "devcontainer")
	if err := os.MkdirAll(materializeDir, 0o755); err != nil {
		return "", "", fmt.Errorf("devcontainer: mkdir %s: %w", materializeDir, err)
	}

	if err := copyDirFiles(filepath.Dir(basePath), materializeDir); err != nil {
		return "", "", fmt.Errorf("devcontainer: copy assets from %s: %w", filepath.Dir(basePath), err)
	}

	return workspaceFolder, filepath.Join(materializeDir, "devcontainer.json"), nil
}

// copyDirFiles copies regular files (non-recursive) from src to dst.
func copyDirFiles(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if info, err := e.Info(); err == nil {
			mode = info.Mode()
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, mode); err != nil {
			return err
		}
	}
	return nil
}
