package agentlaunch

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/platform/framelaunch"
)

func TestEncodeFrameSpec_RoundTrip(t *testing.T) {
	plan := LaunchPlan{
		PreExec:           "mise trust",
		LoginShell:        "/bin/zsh",
		PreCommands:       [][]string{{"/opt/agent-grid/run/bridge", "codex-trust-project"}},
		Argv:              []string{"codex", "--remote", "unix:///tmp/x.sock"},
		PreCommandTimeout: 5 * time.Second,
	}
	raw, err := EncodeFrameSpec(plan)
	if err != nil {
		t.Fatalf("EncodeFrameSpec: %v", err)
	}
	var got framelaunch.FrameSpec
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got.PreExec != plan.PreExec {
		t.Errorf("PreExec = %q", got.PreExec)
	}
	if got.LoginShell != plan.LoginShell {
		t.Errorf("LoginShell = %q", got.LoginShell)
	}
	if len(got.MainCommand) != 3 || got.MainCommand[0] != "codex" {
		t.Errorf("MainCommand = %#v", got.MainCommand)
	}
	if got.PreCommandTimeout != "5s" {
		t.Errorf("PreCommandTimeout = %q, want 5s", got.PreCommandTimeout)
	}
	if len(got.PreCommands) != 1 || got.PreCommands[0][1] != "codex-trust-project" {
		t.Errorf("PreCommands = %#v", got.PreCommands)
	}
}

func TestEncodeFrameSpec_EmptyPreExecOmittedFromJSON(t *testing.T) {
	raw, err := EncodeFrameSpec(LaunchPlan{Argv: []string{"true"}})
	if err != nil {
		t.Fatalf("EncodeFrameSpec: %v", err)
	}
	if strings.Contains(raw, "pre_exec") {
		t.Fatalf("empty PreExec should be omitted: %s", raw)
	}
}
