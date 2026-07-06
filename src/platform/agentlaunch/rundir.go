package agentlaunch

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-grid/platform/appid"
)

// Container-side paths for files bind-mounted from the per-project run dir.
const (
	ContainerRunDir           = appid.ContainerRunDir
	ContainerBinaryPath       = appid.ContainerBinaryPath
	ContainerSockFileName     = appid.ContainerSockFileName
	ContainerSockFilePath     = appid.ContainerSockFilePath
	ContainerHostExecSockPath = appid.ContainerHostExecSockPath
	ContainerMCPSockPath      = appid.ContainerMCPSockPath
)

// ProjectRunDir returns the per-project ephemeral run directory path.
// The directory is bind-mounted into the devcontainer at ContainerRunDir.
func ProjectRunDir(runBase, projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return filepath.Join(runBase, fmt.Sprintf("%x", h[:6]))
}

// EnsureProjectRunDir creates the per-project run directory.
func EnsureProjectRunDir(runBase, projectPath string) (string, error) {
	dir := ProjectRunDir(runBase, projectPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("rundir: mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// ContainerSockPath returns the host-side Unix socket path for the container
// endpoint inside the given run directory.
func ContainerSockPath(runDir string) string {
	return filepath.Join(runDir, ContainerSockFileName)
}

// InstallBinaryInRunDir copies the bridge binary into runDir as
// the bridge binary (mode 0o755).
func InstallBinaryInRunDir(runDir string) (string, error) {
	src, err := findHelperBinary(appid.BridgeBin)
	if err != nil {
		return "", err
	}
	return installBridgeInRunDir(src, runDir)
}

func installBridgeInRunDir(src, runDir string) (string, error) {
	if err := installExecInRunDir(src, filepath.Join(runDir, appid.BridgeBin)); err != nil {
		return "", err
	}
	return ContainerBinaryPath, nil
}

// FindHelperFile returns the absolute path to a helper file if located
// alongside the executable or in ~/.local/lib/agent-grid/. Returns "" when not found.
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
	candidate := filepath.Join(home, ".local", "lib", appid.LibDirName, name)
	if fileExists(candidate) {
		return candidate
	}
	return ""
}

func findHelperBinary(name string) (string, error) {
	if p := FindHelperFile(name); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("rundir: home dir: %w", err)
	}
	return filepath.Join(home, ".local", "lib", appid.LibDirName, name), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func installExecInRunDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("rundir: stat %s: %w", src, err)
	}
	if dstInfo, e := os.Stat(dst); e == nil &&
		dstInfo.Size() == srcInfo.Size() &&
		dstInfo.ModTime().Equal(srcInfo.ModTime()) {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	_ = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("rundir: open binary: %w", err)
	}
	defer in.Close()
	// Write to a temp file in the same directory then rename atomically.
	// O_TRUNC on an in-use ELF binary fails with ETXTBSY on Linux;
	// os.Rename replaces the directory entry without touching the running inode.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".install-*.tmp")
	if err != nil {
		return fmt.Errorf("rundir: create %s: %w", dst, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("rundir: chmod %s: %w", dst, err)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return fmt.Errorf("rundir: copy binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("rundir: close %s: %w", dst, err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("rundir: create %s: %w", dst, err)
	}
	return nil
}
