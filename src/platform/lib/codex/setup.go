package codex

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-reactor/platform/appid"
)

// RegisterMCPServer writes the <client>-peers (e.g. reactor-peers) entry to ~/.codex/mcp.json.
// Returns true if the entry was newly written, false if already present.
func RegisterMCPServer(mcpPath, roostBinary string) (bool, error) {
	servers, err := readMCPServers(mcpPath)
	if err != nil {
		return false, err
	}
	peersServer := appid.PeersServer
	if _, exists := servers[peersServer]; exists {
		return false, nil
	}
	servers[peersServer] = map[string]any{
		"command": roostBinary,
		"args":    []any{"peers-mcp"},
	}
	return true, writeMCPServers(mcpPath, servers)
}

func readMCPServers(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}
	var servers map[string]any
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func writeMCPServers(path string, servers map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
