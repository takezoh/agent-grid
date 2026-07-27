// Package codexconfig manages the small part of Codex configuration owned by
// agent-grid's container launcher.
package codexconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/sys/unix"
)

// EnsureProjectTrusted marks project trusted while retaining all other config.
// Missing project entries are appended so comments and formatting stay intact.
func EnsureProjectTrusted(configPath, project string) error {
	if configPath == "" || project == "" {
		return errors.New("codex config path and project are required")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("codex config: mkdir: %w", err)
	}
	lock, err := os.OpenFile(configPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("codex config: open lock: %w", err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("codex config: lock: %w", err)
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN) //nolint:errcheck

	data, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("codex config: read: %w", err)
	}
	var root map[string]any
	if len(bytes.TrimSpace(data)) > 0 {
		if _, err := toml.Decode(string(data), &root); err != nil {
			return fmt.Errorf("codex config: parse: %w", err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	projects, _ := root["projects"].(map[string]any)
	entry, exists := projects[project].(map[string]any)
	if exists && entry["trust_level"] == "trusted" {
		return nil
	}
	if !exists {
		updated := appendProject(data, project)
		return writeAtomic(configPath, updated)
	}

	// An existing non-trusted entry is uncommon, but must be upgraded rather
	// than duplicated. Re-encoding preserves every semantic setting.
	entry["trust_level"] = "trusted"
	projects[project] = entry
	root["projects"] = projects
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(root); err != nil {
		return fmt.Errorf("codex config: encode: %w", err)
	}
	return writeAtomic(configPath, out.Bytes())
}

func appendProject(data []byte, project string) []byte {
	out := append([]byte(nil), data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	if len(bytes.TrimSpace(out)) > 0 {
		out = append(out, '\n')
	}
	header := "[projects." + strconv.Quote(project) + "]\ntrust_level = \"trusted\"\n"
	return append(out, header...)
}

func writeAtomic(configPath string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(configPath), ".config.toml-*")
	if err != nil {
		return fmt.Errorf("codex config: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("codex config: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("codex config: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("codex config: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("codex config: replace: %w", err)
	}
	return nil
}

// ConfigPath returns the user config path read by Codex: CODEX_HOME/config.toml
// when CODEX_HOME is set, otherwise HOME/.codex/config.toml.
func ConfigPath(codexHome, home string) string {
	if codexHome = strings.TrimSpace(codexHome); codexHome != "" {
		return filepath.Join(codexHome, "config.toml")
	}
	return DefaultPath(home)
}

// DefaultPath returns Codex's conventional user config path when CODEX_HOME
// is unset.
func DefaultPath(home string) string {
	return filepath.Join(strings.TrimSpace(home), ".codex", "config.toml")
}
