package hostexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-grid/platform/appid"
)

// ShimDirName is the subdirectory within the per-project run dir that holds PATH shim scripts.
// Aliased from appid.HostExecShimsDir which is the SSOT (case D — see
// adr-20260716-provider-shim-root-appid-ssot). Retained as a package-level
// exported const for existing call sites.
const ShimDirName = appid.HostExecShimsDir

// OverlayDirName is the subdirectory within the per-project run dir that holds overlay shim scripts.
const OverlayDirName = "hostexec-overlay"

func writeShims(runDir, containerBinPath string, aliases []string) (string, error) {
	shimDir := filepath.Join(runDir, ShimDirName)
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("hostexec shim: mkdir %s: %w", shimDir, err)
	}
	if err := os.Chmod(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("hostexec shim: chmod %s: %w", shimDir, err)
	}
	for _, alias := range aliases {
		if err := writeShim(shimDir, containerBinPath, alias); err != nil {
			return "", err
		}
	}
	return shimDir, nil
}

func writeShim(shimDir, containerBinPath, alias string) error {
	if err := validBinaryName(alias); err != nil {
		return fmt.Errorf("hostexec shim: %w", err)
	}
	content := fmt.Sprintf("#!/bin/sh\nexec %s host-exec %s \"$@\"\n", containerBinPath, alias)
	path := filepath.Join(shimDir, alias)
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		if info, serr := os.Stat(path); serr == nil && info.Mode().Perm() != 0o755 {
			return os.Chmod(path, 0o755)
		}
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("hostexec shim: write %s: %w", path, err)
	}
	return os.Chmod(path, 0o755)
}
