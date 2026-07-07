package agentlaunch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectDispatcher_WrapManagedFrameMessagingUsesOverlayHome(t *testing.T) {
	realHome := t.TempDir()
	realClaudeDir := filepath.Join(realHome, ".claude")
	if err := os.MkdirAll(realClaudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realHome, ".claude.json"), []byte(`{"auth":"token"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realClaudeDir, "settings.json"), []byte(`{
  "mcpServers": {
    "obs": {
      "command": "observer"
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(realClaudeDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	d := DirectDispatcher{
		SockPath: "/run/agent-grid/server.sock",
		SelfBin:  "/usr/bin/server",
		DataDir:  t.TempDir(),
	}
	wrapped, err := d.Wrap(context.Background(), "frame-1", LaunchPlan{
		Command:               "claude",
		ManagedFrameMessaging: true,
		Env:                   map[string]string{"HOME": realHome, "AG_SOCKET_TOKEN": "tok"},
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if wrapped.Cleanup == nil {
		t.Fatal("expected cleanup for overlay home")
	}
	if wrapped.Env["HOME"] == "" || wrapped.Env["HOME"] == realHome {
		t.Fatalf("HOME = %q, want overlay home distinct from real home", wrapped.Env["HOME"])
	}
	if wrapped.Env[ManagedClaudeRealHomeEnv] != realHome {
		t.Fatalf("%s = %q, want %q", ManagedClaudeRealHomeEnv, wrapped.Env[ManagedClaudeRealHomeEnv], realHome)
	}
	settingsPath := filepath.Join(wrapped.Env["HOME"], ".claude", "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read overlay settings: %v", err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse overlay settings: %v", err)
	}
	if _, ok := doc.MCPServers["obs"]; !ok {
		t.Fatalf("existing MCP entry missing from overlay: %v", doc.MCPServers)
	}
	var frames struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(doc.MCPServers["agent_frames"], &frames); err != nil {
		t.Fatalf("parse agent_frames entry: %v", err)
	}
	if frames.Command != "/usr/bin/server" {
		t.Fatalf("agent_frames.command = %q, want /usr/bin/server", frames.Command)
	}
	if len(frames.Args) != 3 || frames.Args[2] != "/run/agent-grid/server.sock" {
		t.Fatalf("agent_frames.args = %v, want sock path", frames.Args)
	}
	if got, err := os.Readlink(filepath.Join(wrapped.Env["HOME"], ".claude", "projects")); err != nil || got != filepath.Join(realClaudeDir, "projects") {
		t.Fatalf("projects symlink = (%q, %v), want %q", got, err, filepath.Join(realClaudeDir, "projects"))
	}
	if got, err := os.Readlink(filepath.Join(wrapped.Env["HOME"], ".claude.json")); err != nil || got != filepath.Join(realHome, ".claude.json") {
		t.Fatalf(".claude.json symlink = (%q, %v), want %q", got, err, filepath.Join(realHome, ".claude.json"))
	}
	if err := wrapped.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(wrapped.Env["HOME"]); !os.IsNotExist(err) {
		t.Fatalf("overlay home still exists after cleanup: %v", err)
	}
}
