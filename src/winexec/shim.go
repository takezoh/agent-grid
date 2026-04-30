package winexec

import (
	"fmt"
	"os"
	"path/filepath"
)

const ShimDirName = "winexec-shims"

func writeShims(runDir, containerBinPath string, allowedExes []string) (string, error) {
	shimDir := filepath.Join(runDir, ShimDirName)
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("shim: mkdir %s: %w", shimDir, err)
	}
	if err := os.Chmod(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("shim: chmod %s: %w", shimDir, err)
	}

	for _, exe := range allowedExes {
		if err := writeShim(shimDir, containerBinPath, exe); err != nil {
			return "", err
		}
	}
	return shimDir, nil
}

func writeShim(shimDir, containerBinPath, exe string) error {
	content := fmt.Sprintf("#!/bin/sh\nexec %s win-exec %s \"$@\"\n", containerBinPath, exe)
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
	return os.Chmod(path, 0o755)
}
