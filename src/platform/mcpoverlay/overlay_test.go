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
