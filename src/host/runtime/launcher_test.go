package runtime

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	"github.com/takezoh/agent-grid/platform/framelaunch"
)

var agentlaunchTestTools = agentlaunch.MustNewManagedMCPTools(agentlaunch.ManagedMCPToolAgentFrames)

func TestDirectLauncher_passthrough(t *testing.T) {
	l := DirectLauncher{SelfBin: "/usr/bin/server"}
	plan := state.LaunchPlan{
		Command:  "claude --resume abc",
		StartDir: "/tmp/work",
	}
	env := map[string]string{"AG_FRAME_ID": "f1"}

	got, err := l.WrapLaunch("f1", plan, env)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if got.Command != "/usr/bin/server frame-exec" {
		t.Errorf("Command: want frame-exec, got %q", got.Command)
	}
	if got.Env["AG_FRAME_SPEC"] == "" {
		t.Error("AG_FRAME_SPEC must be set")
	}
	if got.StartDir != plan.StartDir {
		t.Errorf("StartDir: want %q, got %q", plan.StartDir, got.StartDir)
	}
	if got.Env["AG_FRAME_ID"] != "f1" {
		t.Errorf("Env not forwarded, got %v", got.Env)
	}
	if got.Cleanup != nil {
		t.Error("DirectLauncher Cleanup should be nil")
	}
}

func TestDirectLauncher_injectsAG_SOCKET(t *testing.T) {
	l := DirectLauncher{SockPath: "/opt/agent-grid/run/server.sock"}
	plan := state.LaunchPlan{Command: "claude", StartDir: "/work"}
	env := map[string]string{"AG_FRAME_ID": "f1"}

	got, err := l.WrapLaunch("f1", plan, env)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if got.Env["AG_SOCKET"] != "/opt/agent-grid/run/server.sock" {
		t.Errorf("AG_SOCKET = %q, want /opt/agent-grid/run/server.sock", got.Env["AG_SOCKET"])
	}
	if got.Env["AG_FRAME_ID"] != "f1" {
		t.Errorf("AG_FRAME_ID lost, got %v", got.Env)
	}
}

func TestDirectLauncher_noSockPath_noAG_SOCKET(t *testing.T) {
	l := DirectLauncher{}
	got, err := l.WrapLaunch("f1", state.LaunchPlan{Command: "claude"}, nil)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if _, ok := got.Env["AG_SOCKET"]; ok {
		t.Errorf("AG_SOCKET should not be set when SockPath is empty, got %v", got.Env)
	}
}

func TestDirectLauncher_IsContainer(t *testing.T) {
	l := DirectLauncher{}
	if l.IsContainer("/any/project") {
		t.Error("DirectLauncher.IsContainer should always return false")
	}
}

func TestDirectLauncher_stripsContainerToken(t *testing.T) {
	l := DirectLauncher{}
	env := map[string]string{
		"AG_SOCKET_TOKEN": "secret-token",
		"OTHER":           "keep",
	}
	got, err := l.WrapLaunch("f1", state.LaunchPlan{Command: "claude"}, env)
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if got.Env["AG_SOCKET_TOKEN"] != "" {
		t.Errorf("AG_SOCKET_TOKEN = %q, want empty", got.Env["AG_SOCKET_TOKEN"])
	}
	if got.Env["OTHER"] != "keep" {
		t.Errorf("OTHER env lost: %v", got.Env)
	}
}

func TestDirectLauncher_keepsManagedFrameMessagingToken(t *testing.T) {
	home := t.TempDir()
	l := DirectLauncher{SockPath: "/run/server.sock", SelfBin: "/usr/bin/server"}
	got, err := l.WrapLaunch("f1", state.LaunchPlan{
		Command:         "claude",
		ManagedMCPTools: agentlaunchTestTools,
	}, map[string]string{"AG_SOCKET_TOKEN": "frame-token", "HOME": home})
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if got.Env["AG_SOCKET_TOKEN"] != "frame-token" {
		t.Fatalf("AG_SOCKET_TOKEN = %q, want frame-token", got.Env["AG_SOCKET_TOKEN"])
	}
	if got.Env["HOME"] != home {
		t.Fatalf("HOME = %q, want preserved %q", got.Env["HOME"], home)
	}
	if got.Cleanup != nil {
		t.Fatal("launch-scoped MCP configuration must not install HOME cleanup")
	}
	var spec framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(got.Env["AG_FRAME_SPEC"]), &spec); err != nil {
		t.Fatalf("AG_FRAME_SPEC: %v", err)
	}
	assertManagedClaudeLaunchConfig(t, spec.MainCommand, "/usr/bin/server", "/run/server.sock")
}

func assertManagedClaudeLaunchConfig(t *testing.T, argv []string, wantBin, wantSock string) {
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
	if !ok || entry.Command != wantBin || len(entry.Args) != 3 || entry.Args[2] != wantSock {
		t.Fatalf("agent_frames config = %#v, want command %q and socket %q", entry, wantBin, wantSock)
	}
}

func TestDirectLauncher_masksAmbientContainerTokenFromSpawnEnv(t *testing.T) {
	t.Setenv("AG_SOCKET_TOKEN", "ambient-token")

	l := DirectLauncher{}
	got, err := l.WrapLaunch("f1", state.LaunchPlan{Command: "claude"}, nil)
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}

	values, counts := envSliceToMap(t, envSlice(got.Env))
	if values["AG_SOCKET_TOKEN"] != "" {
		t.Fatalf("spawn env AG_SOCKET_TOKEN = %q, want empty", values["AG_SOCKET_TOKEN"])
	}
	if counts["AG_SOCKET_TOKEN"] != 1 {
		t.Fatalf("spawn env AG_SOCKET_TOKEN count = %d, want 1", counts["AG_SOCKET_TOKEN"])
	}
}

func TestWrapLaunchForSpawn_GeneratesTokenForManagedHostFrameMessaging(t *testing.T) {
	l := DirectLauncher{SelfBin: "/usr/bin/server", SockPath: "/run/agent-grid/server.sock"}
	res, err := wrapLaunchForSpawn(l, "f1", "/repo", state.LaunchPlan{
		Command:         "claude",
		ManagedMCPTools: agentlaunchTestTools,
	}, map[string]string{"AG_FRAME_ID": "f1"})
	if err != nil {
		t.Fatalf("wrapLaunchForSpawn: %v", err)
	}
	if res.token == "" {
		t.Fatal("token not generated for managed host frame messaging")
	}
	if res.wrapped.Env["AG_SOCKET_TOKEN"] != res.token {
		t.Fatalf("wrapped env token = %q, want %q", res.wrapped.Env["AG_SOCKET_TOKEN"], res.token)
	}
}

func TestDirectLauncher_CommandStringBecomesFrameExecMain(t *testing.T) {
	l := DirectLauncher{SelfBin: "/usr/local/bin/server"}
	plan := state.LaunchPlan{
		Command: "codex resume thr_123 --remote unix:///opt/agent-grid/run/codex-foo.sock",
	}
	got, err := l.WrapLaunch("f1", plan, nil)
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if got.Command != "/usr/local/bin/server frame-exec" {
		t.Errorf("Command = %q, want frame-exec", got.Command)
	}
	var fs framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(got.Env["AG_FRAME_SPEC"]), &fs); err != nil {
		t.Fatalf("AG_FRAME_SPEC: %v", err)
	}
	if len(fs.MainCommand) < 2 || fs.MainCommand[0] != "codex" {
		t.Errorf("MainCommand = %#v", fs.MainCommand)
	}
}

func TestDirectLauncher_ArgvPath_SpawnsSelfBinFrameExecWithSpec(t *testing.T) {
	l := DirectLauncher{SelfBin: "/usr/local/bin/server"}
	plan := state.LaunchPlan{
		Argv:        []string{"codex", "--remote", "unix:///tmp/x.sock"},
		PreCommands: [][]string{{"/opt/agent-grid/run/bridge", "codex-trust-project"}},
		StartDir:    "/repo",
	}
	got, err := l.WrapLaunch("f1", plan, map[string]string{"KEEP": "1"})
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if got.Command != "/usr/local/bin/server frame-exec" {
		t.Fatalf("Command = %q, want SelfBin frame-exec", got.Command)
	}
	raw := got.Env["AG_FRAME_SPEC"]
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
	if len(fs.PreCommands) != 1 {
		t.Errorf("PreCommands = %#v", fs.PreCommands)
	}
	if got.Env["KEEP"] != "1" {
		t.Errorf("caller env lost: %v", got.Env)
	}
	if got.StartDir != "/repo" {
		t.Errorf("StartDir = %q", got.StartDir)
	}
}

func TestLauncher_nilFallback(t *testing.T) {
	cfg := Config{} // Launcher is nil
	l := launcher(cfg)
	_, isDirect := l.(DirectLauncher)
	if !isDirect {
		t.Errorf("expected DirectLauncher fallback, got %T", l)
	}
}
