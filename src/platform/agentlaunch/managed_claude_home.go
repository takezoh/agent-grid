package agentlaunch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-grid/platform/mcpoverlay"
)

const (
	ManagedClaudeRealHomeEnv    = "AG_CLAUDE_REAL_HOME"
	ManagedClaudeOverlayHomeEnv = "AG_CLAUDE_OVERLAY_HOME"
)

func PrepareManagedClaudeHome(frameID, selfBin, sockPath, dataDir string, baseEnv map[string]string) (map[string]string, func(context.Context) error, error) {
	if strings.TrimSpace(selfBin) == "" || strings.TrimSpace(sockPath) == "" {
		return cloneEnvMap(baseEnv, 0), nil, nil
	}
	realHome := strings.TrimSpace(baseEnv["HOME"])
	if realHome == "" {
		var err error
		realHome, err = os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("managed claude home: resolve HOME: %w", err)
		}
	}

	root := dataDir
	if strings.TrimSpace(root) == "" {
		root = os.TempDir()
	}
	baseDir := filepath.Join(root, "managed-claude-home")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("managed claude home: mkdir root: %w", err)
	}
	overlayHome, err := os.MkdirTemp(baseDir, sanitizeTempPrefix(frameID)+"-")
	if err != nil {
		return nil, nil, fmt.Errorf("managed claude home: mktemp: %w", err)
	}
	cleanup := func(context.Context) error { return os.RemoveAll(overlayHome) }
	if err := populateManagedClaudeHome(overlayHome, realHome, selfBin, sockPath); err != nil {
		_ = os.RemoveAll(overlayHome)
		return nil, nil, err
	}

	env := cloneAndSet(baseEnv, "HOME", overlayHome)
	env[ManagedClaudeRealHomeEnv] = realHome
	env[ManagedClaudeOverlayHomeEnv] = overlayHome
	return env, cleanup, nil
}

func populateManagedClaudeHome(overlayHome, realHome, selfBin, sockPath string) error {
	overlayClaudeDir := filepath.Join(overlayHome, ".claude")
	if err := os.MkdirAll(overlayClaudeDir, 0o700); err != nil {
		return fmt.Errorf("managed claude home: mkdir overlay claude dir: %w", err)
	}
	if err := symlinkTopLevelClaudeConfig(realHome, overlayHome); err != nil {
		return err
	}
	realClaudeDir := filepath.Join(realHome, ".claude")
	if err := symlinkClaudeEntries(realClaudeDir, overlayClaudeDir); err != nil {
		return err
	}
	entry, err := json.Marshal(map[string]any{
		"type":    "stdio",
		"command": selfBin,
		"args":    []string{"agent-frames-mcp", "--sock", sockPath},
	})
	if err != nil {
		return fmt.Errorf("managed claude home: marshal agent_frames entry: %w", err)
	}
	settingsPath := filepath.Join(overlayClaudeDir, "settings.json")
	baseSettingsPath := filepath.Join(realClaudeDir, "settings.json")
	if err := mcpoverlay.WriteJSON(settingsPath, baseSettingsPath, map[string]mcpoverlay.AliasEntry{
		"agent_frames": {Value: entry, Override: true},
	}); err != nil {
		return fmt.Errorf("managed claude home: write settings overlay: %w", err)
	}
	return nil
}

func symlinkTopLevelClaudeConfig(realHome, overlayHome string) error {
	src := filepath.Join(realHome, ".claude.json")
	if _, err := os.Lstat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("managed claude home: stat .claude.json: %w", err)
	}
	dst := filepath.Join(overlayHome, ".claude.json")
	if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
		return fmt.Errorf("managed claude home: symlink .claude.json: %w", err)
	}
	return nil
}

func symlinkClaudeEntries(realClaudeDir, overlayClaudeDir string) error {
	entries, err := os.ReadDir(realClaudeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("managed claude home: read real claude dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case "settings.json", "settings.json.bak", "settings.json.lock":
			continue
		}
		src := filepath.Join(realClaudeDir, name)
		dst := filepath.Join(overlayClaudeDir, name)
		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return fmt.Errorf("managed claude home: symlink %s: %w", name, err)
		}
	}
	return nil
}

func sanitizeTempPrefix(frameID string) string {
	if frameID == "" {
		return "frame"
	}
	repl := strings.NewReplacer("/", "_", string(os.PathSeparator), "_", ":", "_")
	return repl.Replace(frameID)
}
