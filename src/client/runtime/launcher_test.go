package runtime

import (
	"testing"

	"github.com/takezoh/agent-grid/client/state"
)

func TestDirectLauncher_passthrough(t *testing.T) {
	l := DirectLauncher{}
	plan := state.LaunchPlan{
		Command:  "claude --resume abc",
		StartDir: "/tmp/work",
	}
	env := map[string]string{"AG_FRAME_ID": "f1"}

	got, err := l.WrapLaunch("f1", plan, env)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if got.Command != plan.Command {
		t.Errorf("Command: want %q, got %q", plan.Command, got.Command)
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

func TestDirectLauncher_keepsDirectStreamCommand(t *testing.T) {
	l := DirectLauncher{}
	plan := state.LaunchPlan{
		Command: "codex resume thr_123 --remote unix:///opt/agent-grid/run/codex-foo.sock",
	}
	got, err := l.WrapLaunch("f1", plan, nil)
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if got.Command != plan.Command {
		t.Errorf("Command = %q, want %q", got.Command, plan.Command)
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
