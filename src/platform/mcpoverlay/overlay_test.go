package mcpoverlay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readMCPServers(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return doc.MCPServers
}

// serverCommand decodes a raw mcpServers entry's "command" field for
// comparison, sidestepping MarshalIndent's re-formatting of nested
// json.RawMessage values.
func serverCommand(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal server entry %s: %v", raw, err)
	}
	return v.Command
}

func TestWriteJSONMergesBaseAndAliases(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	if err := os.WriteFile(basePath, []byte(`{"mcpServers":{"existing":{"command":"a"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.json")

	err := WriteJSON(outPath, basePath, map[string]AliasEntry{
		"new-alias": {Value: json.RawMessage(`{"command":"b"}`)},
	})
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got := readMCPServers(t, outPath)
	if len(got) != 2 {
		t.Fatalf("servers = %v, want 2 entries", got)
	}
	if _, ok := got["existing"]; !ok {
		t.Error("expected base entry \"existing\" to survive merge")
	}
	if _, ok := got["new-alias"]; !ok {
		t.Error("expected alias entry \"new-alias\" to be added")
	}
}

func TestWriteJSONOverridePolicy(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	if err := os.WriteFile(basePath, []byte(`{"mcpServers":{"alias":{"command":"original"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.json")

	// Override: false — an existing alias must not be replaced.
	if err := WriteJSON(outPath, basePath, map[string]AliasEntry{
		"alias": {Value: json.RawMessage(`{"command":"replacement"}`), Override: false},
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got := readMCPServers(t, outPath)
	if cmd := serverCommand(t, got["alias"]); cmd != "original" {
		t.Errorf("command = %q, want %q", cmd, "original")
	}

	// Override: true — replaces the existing alias.
	if err := WriteJSON(outPath, basePath, map[string]AliasEntry{
		"alias": {Value: json.RawMessage(`{"command":"replacement"}`), Override: true},
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got = readMCPServers(t, outPath)
	if cmd := serverCommand(t, got["alias"]); cmd != "replacement" {
		t.Errorf("command = %q, want %q", cmd, "replacement")
	}
}

func TestWriteJSONMissingBaseUsesEmptyDoc(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	err := WriteJSON(outPath, filepath.Join(dir, "does-not-exist.json"), map[string]AliasEntry{
		"only": {Value: json.RawMessage(`{"command":"x"}`)},
	})
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got := readMCPServers(t, outPath)
	if len(got) != 1 {
		t.Fatalf("servers = %v, want 1 entry", got)
	}
}

// TestWriteJSONPreservesNonMCPTopLevelKeys pins the invariant that fields
// outside `mcpServers` (hooks / permissions / statusLine / enabledPlugins
// / env — every key managed_claude_home writes overlay on top of) survive
// the overlay write. Without this, host+claude launches under
// managed_claude_home would silently strip every user-configured Claude
// hook and permission, leaving the launched claude unable to signal the
// daemon (regression on spec-20260714 FR-002 hook continuity).
func TestWriteJSONPreservesNonMCPTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	baseDoc := `{
  "mcpServers": {"existing": {"command": "a"}},
  "hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "server event claude"}]}]},
  "permissions": {"allow": ["Read(/repo/**)"]},
  "statusLine": {"type": "cmd", "command": "status.sh"},
  "enabledPlugins": {"foo@1": true},
  "env": {"MY_VAR": "1"}
}`
	if err := os.WriteFile(basePath, []byte(baseDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.json")

	if err := WriteJSON(outPath, basePath, map[string]AliasEntry{
		"agent_frames": {Value: json.RawMessage(`{"command":"bin"}`)},
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, raw)
	}

	for _, key := range []string{"hooks", "permissions", "statusLine", "enabledPlugins", "env"} {
		if _, ok := out[key]; !ok {
			t.Errorf("top-level key %q dropped by WriteJSON — this would silently strip user hooks / permissions under HOME overlay", key)
		}
	}

	// mcpServers merge still works — existing entry preserved and alias added.
	servers := readMCPServers(t, outPath)
	if _, ok := servers["existing"]; !ok {
		t.Error("mcpServers.existing dropped during merge")
	}
	if _, ok := servers["agent_frames"]; !ok {
		t.Error("agent_frames alias not added")
	}
}

// TestWriteJSONSkipsRewriteWhenUnchanged exercises the bytes.Equal
// short-circuit path by writing the same content twice; the second call
// must still succeed and leave the file's content untouched.
func TestWriteJSONSkipsRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	aliases := map[string]AliasEntry{"a": {Value: json.RawMessage(`{"command":"x"}`)}}

	if err := WriteJSON(outPath, filepath.Join(dir, "missing.json"), aliases); err != nil {
		t.Fatalf("WriteJSON (first): %v", err)
	}
	first, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteJSON(outPath, filepath.Join(dir, "missing.json"), aliases); err != nil {
		t.Fatalf("WriteJSON (second): %v", err)
	}
	second, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("content changed on unchanged rewrite: %s != %s", first, second)
	}
}
