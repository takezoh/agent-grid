package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/takezoh/agent-grid/services/platform/appid"
	platformconfig "github.com/takezoh/agent-grid/services/platform/config"
)

type Config struct {
	DataDir  string                        `toml:"data_dir"`
	Log      LogConfig                     `toml:"log"`
	Monitor  MonitorConfig                 `toml:"monitor"`
	Session  SessionConfig                 `toml:"session"`
	Terminal TerminalConfig                `toml:"terminal"`
	Projects platformconfig.ProjectsConfig `toml:"projects"`
	Driver   CommonDriverConfig            `toml:"driver"`
	Drivers  map[string]map[string]any     `toml:"drivers"`
	Features FeaturesConfig                `toml:"features"`
	Codex    CodexConfig                   `toml:"codex"`
}

// CodexConfig holds settings for the Codex app-server integration.
type CodexConfig struct {
	// ReadTimeoutMs is the per-request JSON-RPC read timeout in milliseconds.
	// Zero means use the default (15 s).
	ReadTimeoutMs int `toml:"read_timeout_ms"`
}

// CommonDriverConfig holds settings that apply to all drivers.
type CommonDriverConfig struct {
	SummarizeCommand string `toml:"summarize_command"`
	// CapsuleCommand is the one-shot LLM command for resume-capsule
	// generation (e.g. "claude -p"). The reducer-built prompt arrives on
	// stdin; stdout must contain the capsule JSON. Empty disables
	// resume capsules entirely.
	CapsuleCommand string `toml:"capsule_command"`
	Pager          string `toml:"pager"`
}

// FeaturesConfig holds the runtime feature-flag table from the TOML config.
type FeaturesConfig struct {
	Enabled map[string]bool `toml:"enabled"`
}

// LogConfig controls slog handler verbosity.
type LogConfig struct {
	Level string `toml:"level"`
}

type MonitorConfig struct {
	PollIntervalMs   int `toml:"poll_interval_ms"`
	IdleThresholdSec int `toml:"idle_threshold_sec"`
}

type SessionConfig struct {
	CommandProfiles []CommandProfileConfig `toml:"command_profiles"`
	PushCommands    []string               `toml:"push_commands"`
}

// TerminalConfig holds knobs that govern the server-side terminal emulator.
type TerminalConfig struct {
	// ScrollbackLines bounds the VT scrollback buffer per session, in lines.
	// A late-joining Web UI client receives this buffer as the first seed
	// frame on subscribe so it can scroll up through history printed before
	// it attached. Zero leaves the underlying emulator's default in place.
	ScrollbackLines int `toml:"scrollback_lines"`
}

func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
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
	warnRemovedSandboxTable(metadata, path)
	if err := cfg.Session.ValidateCommandProfiles(); err != nil {
		return nil, fmt.Errorf("session command profiles: %w", err)
	}
	cfg.Driver.Pager = resolvePager(cfg.Driver.Pager)
	return cfg, nil
}

func resolvePager(configured string) string {
	if configured != "" {
		return configured
	}
	if p := os.Getenv("PAGER"); p != "" {
		return p
	}
	return "less"
}

func Load() (*Config, error) {
	return LoadFrom(filepath.Join(ConfigDirPath(), "settings.toml"))
}

func DefaultConfig() *Config {
	return &Config{
		Log: LogConfig{Level: "info"},
		Monitor: MonitorConfig{
			PollIntervalMs:   1000,
			IdleThresholdSec: 30,
		},
		Session: SessionConfig{
			CommandProfiles: []CommandProfileConfig{{
				ID: "shell", Label: "Shell", Executable: "shell",
			}},
			PushCommands: []string{"shell"},
		},
		Terminal: TerminalConfig{
			ScrollbackLines: 10000,
		},
		Projects: platformconfig.ProjectsConfig{},
	}
}

func ConfigDirPath() string {
	return filepath.Join(platformconfig.ExpandPath("~"), appid.DotDir)
}

func EnsureConfigDir() string {
	dir := ConfigDirPath()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func (c *Config) ListProjects() []string {
	return c.Projects.ListProjects()
}

func (c *Config) ResolveDataDir() string {
	if v := os.Getenv("AG_DATA_DIR"); v != "" {
		return platformconfig.ExpandPath(v)
	}
	if c.DataDir != "" {
		return platformconfig.ExpandPath(c.DataDir)
	}
	return ConfigDirPath()
}
