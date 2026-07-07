package mcpoverlay

import (
	"bytes"
	"encoding/json"
	"os"
)

type AliasEntry struct {
	Value    json.RawMessage
	Override bool
}

func WriteJSON(path, basePath string, aliases map[string]AliasEntry) error {
	merged := make(map[string]json.RawMessage)
	if raw, err := os.ReadFile(basePath); err == nil {
		var doc struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if json.Unmarshal(raw, &doc) == nil {
			for k, v := range doc.MCPServers {
				merged[k] = v
			}
		}
	}

	for alias, entry := range aliases {
		if _, exists := merged[alias]; exists && !entry.Override {
			continue
		}
		merged[alias] = entry.Value
	}

	data, err := json.MarshalIndent(map[string]any{"mcpServers": merged}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o600)
}
