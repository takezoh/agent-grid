package runtime

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// Container-side paths for files bind-mounted from the per-project run dir.
// These are the canonical sources; callers must not hard-code these literals.
const (
	ContainerRunDir           = "/opt/roost/run"
	ContainerBinaryPath       = ContainerRunDir + "/roost-bridge"
	ContainerSockFileName     = "roost.sock"
	ContainerSockFilePath     = ContainerRunDir + "/" + ContainerSockFileName
	ContainerHostExecSockPath = ContainerRunDir + "/hostexec.sock"
	ContainerMCPSockPath      = ContainerRunDir + "/mcp.sock"
)

// ProjectRunDir returns the per-project ephemeral run directory path.
// The directory is bind-mounted into the devcontainer at ContainerRunDir.
// Its inode is stable across daemon restarts; only the files inside change.
func ProjectRunDir(runBase, projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return filepath.Join(runBase, fmt.Sprintf("%x", h[:6]))
}

// EnsureProjectRunDir creates the per-project run directory.
// Returns the run dir path.
func EnsureProjectRunDir(runBase, projectPath string) (string, error) {
	dir := ProjectRunDir(runBase, projectPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("rundir: mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// ContainerSockPath returns the host-side Unix socket path for the container
// endpoint inside the given run directory. The same socket appears inside the
// container at ContainerSockFilePath via the run-dir bind mount.
func ContainerSockPath(runDir string) string {
	return filepath.Join(runDir, ContainerSockFileName)
}

// FindHelperFile returns the absolute path to a helper file (binary, script,
// asset) if it can be located alongside the executable or in the libexec
// directory (~/.local/lib/roost/). Returns "" when not found at either location.
func FindHelperFile(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	if candidate := filepath.Join(filepath.Dir(exe), name); fileExists(candidate) {
		return candidate
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".local", "lib", "roost", name)
	if fileExists(candidate) {
		return candidate
	}
	return ""
}

// findHelperBinary resolves the path to a helper binary. Returns the located
// path when found; otherwise returns the standard libexec path so the caller
// can fail with a clear stat error.
func findHelperBinary(name string) (string, error) {
	if p := FindHelperFile(name); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("rundir: home dir: %w", err)
	}
	return filepath.Join(home, ".local", "lib", "roost", name), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
