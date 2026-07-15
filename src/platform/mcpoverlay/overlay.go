package mcpoverlay

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
)

type AliasEntry struct {
	Value    json.RawMessage
	Override bool
}

// WriteJSON merges aliases into the mcpServers section of the base JSON
// document and writes the result to path.
//
// Invariant: every non-mcpServers top-level key in the base document is
// preserved verbatim in the output. This matters for HOME-overlay callers
// (managed_claude_home) whose base is the user's ~/.claude/settings.json
// carrying hooks, permissions, statusLine, enabledPlugins, etc. — losing
// those silently would leave the launched claude with no lifecycle hooks
// reaching the daemon (spec-20260714 FR-002 hook continuity).
//
// If basePath does not exist or is unreadable, the output contains only the
// mcpServers section built from aliases.
func WriteJSON(path, basePath string, aliases map[string]AliasEntry) error {
	doc := make(map[string]json.RawMessage)
	if raw, err := os.ReadFile(basePath); err == nil {
		// Best-effort: if the base is not valid JSON, fall back to
		// emitting only the mcpServers overlay rather than failing.
		_ = json.Unmarshal(raw, &doc)
	}

	var merged map[string]json.RawMessage
	if raw, ok := doc["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &merged); err != nil {
			merged = nil
		}
	}
	if merged == nil {
		merged = make(map[string]json.RawMessage)
	}

	for alias, entry := range aliases {
		if _, exists := merged[alias]; exists && !entry.Override {
			continue
		}
		merged[alias] = entry.Value
	}

	// Re-serialize mcpServers with deterministic key order so the file
	// content is byte-stable across writes.
	mcpRaw, err := marshalOrdered(merged)
	if err != nil {
		return err
	}
	doc["mcpServers"] = mcpRaw

	data, err := marshalOrdered(doc)
	if err != nil {
		return err
	}
	// json.MarshalIndent-equivalent formatting for readability.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	out := pretty.Bytes()
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, out) {
		return nil
	}
	return os.WriteFile(path, out, 0o600)
}

// marshalOrdered serialises a map[string]json.RawMessage with keys sorted
// alphabetically so successive writes of the same logical content produce
// byte-identical files.
func marshalOrdered(m map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(m[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
