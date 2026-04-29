package winexec

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ContainerBinaryPath is the in-container path to the roost binary.
	roostBin = "/opt/roost/run/roost"
	// ShimDirName is the subdirectory of the per-project run dir that holds shims.
	ShimDirName = "winexec-shims"
)

// writeShims creates (or overwrites) shim shell scripts under runDir/winexec-shims/
// for each exe in allowedExes. Each shim delegates to "roost win-exec <exe>".
// Returns the absolute path to the shim directory.
func writeShims(runDir string, allowedExes []string) (string, error) {
	shimDir := filepath.Join(runDir, ShimDirName)
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("shim: mkdir %s: %w", shimDir, err)
	}
	// MkdirAll does not change mode on existing dirs; chmod explicitly so
	// container users (non-root) can traverse the dir.
	if err := os.Chmod(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("shim: chmod %s: %w", shimDir, err)
	}

	for _, exe := range allowedExes {
		if err := writeShim(shimDir, exe); err != nil {
			return "", err
		}
	}
	return shimDir, nil
}

func writeShim(shimDir, exe string) error {
	content := fmt.Sprintf("#!/bin/sh\nexec %s win-exec %s \"$@\"\n", roostBin, exe)
	path := filepath.Join(shimDir, exe)

	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		if info, serr := os.Stat(path); serr == nil && info.Mode().Perm() != 0o755 {
			return os.Chmod(path, 0o755)
		}
		return nil
	}

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("shim: write %s: %w", path, err)
	}
	// WriteFile does not change mode on existing files; chmod explicitly.
	return os.Chmod(path, 0o755)
}
