package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/takezoh/agent-grid/services/platform/appid"
	platformconfig "github.com/takezoh/agent-grid/services/platform/config"
)

// ProjectConfig holds the contents of a project-level .agent-grid/settings.toml.
type ProjectConfig struct {
	Workspace ProjectWorkspaceConfig        `toml:"workspace"`
	Sandbox   *platformconfig.SandboxConfig `toml:"sandbox"`
	Session   ProjectSessionConfig          `toml:"session"`
}

type ProjectSessionConfig struct {
	CommandProfiles []CommandProfileConfig `toml:"command_profiles"`
}

// ProjectWorkspaceConfig is the [workspace] table inside a project settings file.
type ProjectWorkspaceConfig struct {
	Name string `toml:"name"`
}

// DefaultWorkspaceName is the workspace assigned to any project that does not
// explicitly set one.
const DefaultWorkspaceName = "default"

// MaxWorkspaceNameLen is the maximum rune-length of a workspace name.
const MaxWorkspaceNameLen = 64

// LoadProjectFrom reads the file at path as a project-level settings.toml.
// A missing file is not an error; it returns a zero-value *ProjectConfig.
func LoadProjectFrom(path string) (*ProjectConfig, error) {
	cfg := &ProjectConfig{}
	metadata, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := rejectRemovedCommandProfileFields(metadata); err != nil {
		return nil, err
	}
	if cfg.Sandbox != nil {
		if err := cfg.Sandbox.Proxy.GCP.Validate(); err != nil {
			return nil, err
		}
	}
	session := SessionConfig{CommandProfiles: cfg.Session.CommandProfiles}
	if err := session.validateCommandProfiles(true); err != nil {
		return nil, fmt.Errorf("session.command_profiles: %w", err)
	}
	cfg.Session.CommandProfiles = session.CommandProfiles
	return cfg, nil
}

// LoadProject resolves the project settings file for the given project directory.
func LoadProject(projectDir string) (*ProjectConfig, error) {
	path := findProjectSettings(projectDir)
	if path == "" {
		return &ProjectConfig{}, nil
	}
	return LoadProjectFrom(path)
}

// WorkspaceName returns the configured workspace name, or DefaultWorkspaceName.
func (pc *ProjectConfig) WorkspaceName() string {
	name := strings.TrimSpace(pc.Workspace.Name)
	if name == "" {
		return DefaultWorkspaceName
	}
	return name
}

// Validate reports an error when the workspace name is invalid.
func (pc *ProjectConfig) Validate() error {
	name := strings.TrimSpace(pc.Workspace.Name)
	if name == "" {
		return nil
	}
	runes := []rune(name)
	if len(runes) > MaxWorkspaceNameLen {
		return fmt.Errorf("workspace.name: too long (%d runes, max %d)", len(runes), MaxWorkspaceNameLen)
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return fmt.Errorf("workspace.name: contains control character %U", r)
		}
	}
	return nil
}

// findProjectSettings walks up from dir searching for .agent-grid/settings.toml.
func findProjectSettings(dir string) string {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, appid.DotDir, "settings.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
