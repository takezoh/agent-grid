package clientconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const ServersSchemaVersion = 2

type Auth struct {
	Mode  string `json:"mode"`
	Token string `json:"token,omitempty"`
}

type Server struct {
	ID          string `json:"-"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url"`
	Auth        Auth   `json:"auth"`
}

type Snapshot struct {
	Servers []Server
}

type serversDocument struct {
	SchemaVersion int      `json:"schema_version"`
	Servers       []Server `json:"servers"`
}

func DefaultDirectory() (string, error) {
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, "agent-grid"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "agent-grid"), nil
}

func Load(directory string) (Snapshot, error) {
	data, err := os.ReadFile(filepath.Join(directory, "servers.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("config_unavailable: servers.json: %w", err)
	}
	var document serversDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return Snapshot{}, fmt.Errorf("config_invalid: servers.json: %w", err)
	}
	if document.SchemaVersion != ServersSchemaVersion {
		return Snapshot{}, fmt.Errorf(
			"config_invalid: servers.json: unsupported schema_version %d; expected %d",
			document.SchemaVersion,
			ServersSchemaVersion,
		)
	}
	if err := validateServers(document.Servers); err != nil {
		return Snapshot{}, fmt.Errorf("config_invalid: servers.json: %w", err)
	}
	return Snapshot{Servers: document.Servers}, nil
}

func validateServers(servers []Server) error {
	if len(servers) == 0 {
		return errors.New("at least one server is required")
	}
	ids := make(map[string]struct{}, len(servers))
	enabled := 0
	for index := range servers {
		server := &servers[index]
		if strings.TrimSpace(server.DisplayName) == "" {
			return errors.New("every server needs display_name")
		}
		canonical, err := canonicalURL(server.URL)
		if err != nil {
			return fmt.Errorf("server %q url: %w", server.DisplayName, err)
		}
		server.URL = canonical
		server.ID = ServerID(canonical)
		if _, exists := ids[server.ID]; exists {
			return fmt.Errorf("duplicate server url %q", server.URL)
		}
		ids[server.ID] = struct{}{}
		if server.Enabled {
			enabled++
		}
		switch server.Auth.Mode {
		case "none":
			if server.Auth.Token != "" {
				return fmt.Errorf("server %q auth none must not include token", server.DisplayName)
			}
		case "bearer":
			if strings.TrimSpace(server.Auth.Token) == "" {
				return fmt.Errorf("server %q bearer token is required", server.DisplayName)
			}
		default:
			return fmt.Errorf("server %q has unsupported auth mode %q", server.DisplayName, server.Auth.Mode)
		}
	}
	if enabled == 0 {
		return errors.New("at least one server must be enabled")
	}
	return nil
}

func canonicalURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("credentials, query, fragment, or empty host are not allowed")
	}
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	if parsed.Path == "" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func ServerID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "srv_" + hex.EncodeToString(sum[:])
}

func (snapshot Snapshot) EnabledServer(id string) (Server, bool) {
	for _, server := range snapshot.Servers {
		if server.Enabled && server.ID == id {
			return server, true
		}
	}
	return Server{}, false
}
