package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestRunCodexTrustProjectDefaultsToWorkingDirectory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	if err := runCodexTrustProject([]string{"-config", configPath}); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if _, err := toml.DecodeFile(configPath, &decoded); err != nil {
		t.Fatal(err)
	}
	entry := decoded["projects"].(map[string]any)[project].(map[string]any)
	if entry["trust_level"] != "trusted" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestRunCodexTrustProjectUsesCodexHomeConfig(t *testing.T) {
	project := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	codexHome := filepath.Join(t.TempDir(), "custom-codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "unrelated-home"))

	if err := runCodexTrustProject(nil); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(codexHome, "config.toml")
	var decoded map[string]any
	if _, err := toml.DecodeFile(configPath, &decoded); err != nil {
		t.Fatal(err)
	}
	entry := decoded["projects"].(map[string]any)[project].(map[string]any)
	if entry["trust_level"] != "trusted" {
		t.Fatalf("entry = %#v", entry)
	}
}
