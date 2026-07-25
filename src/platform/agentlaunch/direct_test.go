package agentlaunch

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-grid/platform/framelaunch"
)

func TestDirectDispatcher_WrapManagedFrameMessagingPreservesHomeAndInjectsMCPConfig(t *testing.T) {
	realHome := t.TempDir()
	d := DirectDispatcher{
		SockPath: "/run/agent-grid/server.sock",
		SelfBin:  "/usr/bin/server",
	}
	wrapped, err := d.Wrap(context.Background(), "frame-1", LaunchPlan{
		Command:               "claude --resume session-1",
		ManagedFrameMessaging: true,
		Env:                   map[string]string{"HOME": realHome, "AG_SOCKET_TOKEN": "tok"},
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if wrapped.Env["HOME"] != realHome {
		t.Fatalf("HOME = %q, want preserved %q", wrapped.Env["HOME"], realHome)
	}
	if wrapped.Cleanup != nil {
		t.Fatal("launch-scoped MCP configuration must not install HOME cleanup")
	}
	var spec framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(wrapped.Env[framelaunch.EnvVar]), &spec); err != nil {
		t.Fatalf("AG_FRAME_SPEC JSON: %v", err)
	}
	assertManagedClaudeMCPConfig(t, spec.MainCommand, "/usr/bin/server", "/run/agent-grid/server.sock")
}

func assertManagedClaudeMCPConfig(t *testing.T, argv []string, wantBin, wantSock string) {
	t.Helper()
	if len(argv) < 3 || argv[len(argv)-2] != "--mcp-config" {
		t.Fatalf("MainCommand = %#v, want trailing --mcp-config JSON", argv)
	}
	var config struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(argv[len(argv)-1]), &config); err != nil {
		t.Fatalf("managed MCP config JSON: %v", err)
	}
	entry, ok := config.MCPServers["agent_frames"]
	if !ok {
		t.Fatalf("managed MCP config lacks agent_frames: %#v", config.MCPServers)
	}
	if entry.Command != wantBin {
		t.Fatalf("agent_frames.command = %q, want %q", entry.Command, wantBin)
	}
	if len(entry.Args) != 3 || entry.Args[0] != "agent-frames-mcp" || entry.Args[1] != "--sock" || entry.Args[2] != wantSock {
		t.Fatalf("agent_frames.args = %#v, want socket %q", entry.Args, wantSock)
	}
}

func TestDirectDispatcher_ArgvPath_SpawnsSelfBinFrameExecWithSpec(t *testing.T) {
	d := DirectDispatcher{SelfBin: "/usr/local/bin/server"}
	wrapped, err := d.Wrap(context.Background(), "f1", LaunchPlan{
		Argv:        []string{"codex", "--remote", "unix:///tmp/x.sock"},
		PreCommands: [][]string{{ContainerBinaryPath, "codex-trust-project"}},
		StartDir:    "/repo",
		Env:         map[string]string{"KEEP": "1"},
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if wrapped.Command != "/usr/local/bin/server frame-exec" {
		t.Fatalf("Command = %q, want SelfBin frame-exec", wrapped.Command)
	}
	raw := wrapped.Env[framelaunch.EnvVar]
	if raw == "" {
		t.Fatal("AG_FRAME_SPEC not set")
	}
	var fs framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(raw), &fs); err != nil {
		t.Fatalf("AG_FRAME_SPEC JSON: %v", err)
	}
	if len(fs.MainCommand) != 3 || fs.MainCommand[0] != "codex" {
		t.Errorf("MainCommand = %#v", fs.MainCommand)
	}
	if wrapped.Env["KEEP"] != "1" {
		t.Errorf("caller env lost: %v", wrapped.Env)
	}
}

func TestDirectDispatcher_CommandStringBecomesFrameExecMain(t *testing.T) {
	d := DirectDispatcher{SelfBin: "/usr/local/bin/server"}
	wrapped, err := d.Wrap(context.Background(), "f1", LaunchPlan{
		Command:  "claude --resume x",
		StartDir: "/w",
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if wrapped.Command != "/usr/local/bin/server frame-exec" {
		t.Errorf("Command = %q", wrapped.Command)
	}
	var fs framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(wrapped.Env[framelaunch.EnvVar]), &fs); err != nil {
		t.Fatalf("AG_FRAME_SPEC: %v", err)
	}
	if len(fs.MainCommand) < 2 || fs.MainCommand[0] != "claude" {
		t.Errorf("MainCommand = %#v", fs.MainCommand)
	}
}
