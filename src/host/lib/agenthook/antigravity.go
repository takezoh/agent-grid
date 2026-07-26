package agenthook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const AntigravityHookNamespace = "agent-grid"
const AntigravitySetupSubcommand = "agy-setup-hooks"

var AntigravityPluginDirs = []string{
	".gemini/config/plugins/agent-grid",
	".gemini/antigravity-cli/plugins/agent-grid",
}

const antigravityManagedMarker = ".agent-grid-managed"

func InstallAntigravityPlugins(home, binPath, dataDir string) ([]string, error) {
	if home == "" || binPath == "" {
		return nil, fmt.Errorf("agenthook[agy]: home and binPath are required")
	}
	for _, rel := range AntigravityPluginDirs {
		dir := filepath.Join(home, rel)
		if _, err := os.Stat(dir); err == nil {
			if _, markerErr := os.Stat(filepath.Join(dir, antigravityManagedMarker)); markerErr != nil {
				return nil, fmt.Errorf("agenthook[agy]: refusing unmanaged plugin %s", dir)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("agenthook[agy]: inspect %s: %w", dir, err)
		}
	}
	changed := make([]string, 0, len(AntigravityPluginDirs))
	for _, rel := range AntigravityPluginDirs {
		dir := filepath.Join(home, rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return changed, fmt.Errorf("agenthook[agy]: mkdir %s: %w", dir, err)
		}
		pluginChanged := false
		marker := filepath.Join(dir, antigravityManagedMarker)
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			if err := os.WriteFile(marker, []byte("managed by agent-grid\n"), 0o644); err != nil {
				return changed, fmt.Errorf("agenthook[agy]: marker %s: %w", dir, err)
			}
			pluginChanged = true
		}
		fileChanged, err := writeJSONFileIfChanged(filepath.Join(dir, "plugin.json"), map[string]any{
			"name":        AntigravityHookNamespace,
			"description": "Forwards passive Antigravity lifecycle hooks to Agent Grid.",
		})
		if err != nil {
			return changed, err
		}
		pluginChanged = pluginChanged || fileChanged
		fileChanged, err = writeJSONFileIfChanged(
			filepath.Join(dir, "hooks.json"), antigravityHooks(binPath, dataDir))
		if err != nil {
			return changed, err
		}
		pluginChanged = pluginChanged || fileChanged
		if pluginChanged {
			changed = append(changed, rel)
		}
	}
	return changed, nil
}

func antigravityHooks(binPath, dataDir string) map[string]any {
	direct := func(event string) []any {
		return []any{map[string]any{
			"type":    "command",
			"command": BuildAntigravityHookCmd(binPath, dataDir, event),
			"timeout": 5,
		}}
	}
	postTool := []any{map[string]any{
		"matcher": "*",
		"hooks":   direct("PostToolUse"),
	}}
	return map[string]any{AntigravityHookNamespace: map[string]any{
		"PreInvocation":  direct("PreInvocation"),
		"PostInvocation": direct("PostInvocation"),
		"PostToolUse":    postTool,
		"Stop":           direct("Stop"),
	}}
}

func BuildAntigravityHookCmd(binPath, dataDir, event string) string {
	cmd := shellQuote(binPath) + " event " + event + " -hook-contract agy"
	if dataDir != "" {
		cmd += " -data-dir " + shellQuote(dataDir)
	}
	return cmd
}

func writeJSONFileIfChanged(path string, value any) (bool, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return false, fmt.Errorf("agenthook[agy]: encode %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if previous, readErr := os.ReadFile(path); readErr == nil && string(previous) == string(raw) {
		return false, nil
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return false, fmt.Errorf("agenthook[agy]: write %s: %w", path, err)
	}
	return true, nil
}
