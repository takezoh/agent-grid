package agentlaunch

import (
	"context"
	"testing"
)

// TestWrap_NormalizesCommandToArgvAndUsesFrameExec pins that DevcontainerLauncher
// always hands the sandbox manager a structured Argv LaunchSpec (never a residual
// Command string) and that quoted agent args survive SplitArgs.
func TestWrap_NormalizesCommandToArgvAndUsesFrameExec(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "")
	// Quoted config override mirrors codex -c sandbox_mode="danger-full-access".
	cmd := `codex app-server -c sandbox_mode="danger-full-access"`
	_, err := l.Wrap(context.Background(), "frame-1", LaunchPlan{
		Project:  "/p",
		StartDir: "/p",
		Command:  cmd,
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if mgr.buildSpec.Command != "" {
		t.Fatalf("BuildLaunchCommand must receive empty Command, got %q", mgr.buildSpec.Command)
	}
	if len(mgr.buildSpec.Argv) < 3 || mgr.buildSpec.Argv[0] != "codex" {
		t.Fatalf("Argv = %#v", mgr.buildSpec.Argv)
	}
	// SplitArgs cooks quoted values; -c value must remain one token.
	found := false
	for i, a := range mgr.buildSpec.Argv {
		if a == "-c" && i+1 < len(mgr.buildSpec.Argv) {
			if mgr.buildSpec.Argv[i+1] != `sandbox_mode=danger-full-access` {
				t.Fatalf("-c value = %q, want sandbox_mode=danger-full-access", mgr.buildSpec.Argv[i+1])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("quoted -c value lost: %#v", mgr.buildSpec.Argv)
	}
}
