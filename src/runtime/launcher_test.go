package runtime

import (
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestDirectLauncher_passthrough(t *testing.T) {
	l := DirectLauncher{}
	plan := state.LaunchPlan{
		Command:  "claude --resume abc",
		StartDir: "/tmp/work",
	}
	env := map[string]string{"ROOST_FRAME_ID": "f1"}

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
	if got.Env["ROOST_FRAME_ID"] != "f1" {
		t.Errorf("Env not forwarded, got %v", got.Env)
	}
	if got.Cleanup != nil {
		t.Error("DirectLauncher Cleanup should be nil")
	}
}

func TestDirectLauncher_injectsROOST_SOCKET(t *testing.T) {
	l := DirectLauncher{SockPath: "/opt/roost/run/roost.sock"}
	plan := state.LaunchPlan{Command: "claude", StartDir: "/work"}
	env := map[string]string{"ROOST_FRAME_ID": "f1"}

	got, err := l.WrapLaunch("f1", plan, env)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if got.Env["ROOST_SOCKET"] != "/opt/roost/run/roost.sock" {
		t.Errorf("ROOST_SOCKET = %q, want /opt/roost/run/roost.sock", got.Env["ROOST_SOCKET"])
	}
	if got.Env["ROOST_FRAME_ID"] != "f1" {
		t.Errorf("ROOST_FRAME_ID lost, got %v", got.Env)
	}
}

func TestDirectLauncher_noSockPath_noROOST_SOCKET(t *testing.T) {
	l := DirectLauncher{}
	got, err := l.WrapLaunch("f1", state.LaunchPlan{Command: "claude"}, nil)
	if err != nil {
		t.Fatalf("WrapLaunch returned error: %v", err)
	}
	if _, ok := got.Env["ROOST_SOCKET"]; ok {
		t.Errorf("ROOST_SOCKET should not be set when SockPath is empty, got %v", got.Env)
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
