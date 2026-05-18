package runtime

import (
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestSubsystemRegistry_Inject_CLIPassthrough(t *testing.T) {
	reg := newSubsystemRegistry()
	plan := state.LaunchPlan{Command: "claude --resume abc"}
	_, env, err := reg.Inject(state.LaunchSubsystemCLI, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Errorf("env = %v, want empty", env)
	}
}

func TestSubsystemRegistry_Inject_StreamPassthrough(t *testing.T) {
	reg := newSubsystemRegistry()
	plan := state.LaunchPlan{
		Command:   "codex --model o3",
		Subsystem: state.LaunchSubsystemStream,
	}
	got, env, err := reg.Inject(state.LaunchSubsystemStream, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != plan.Command {
		t.Errorf("command = %q, want %q", got.Command, plan.Command)
	}
	if len(env) != 0 {
		t.Errorf("env = %v, want empty", env)
	}
}

func TestSubsystemRegistry_Inject_UnknownKindErrors(t *testing.T) {
	reg := newSubsystemRegistry()
	_, _, err := reg.Inject("unknown-kind", state.LaunchPlan{}, nil)
	if err == nil {
		t.Error("expected error for unknown subsystem kind")
	}
}

func TestCLISubsystemRewritePlan_passthrough(t *testing.T) {
	plan := state.LaunchPlan{
		Command:  "claude --resume abc",
		StartDir: "/repo",
	}
	got, env := cliSubsystem{}.RewritePlan(plan, nil)
	if got.Command != plan.Command {
		t.Errorf("Command: got %q, want %q", got.Command, plan.Command)
	}
	if len(env) != 0 {
		t.Errorf("env: got %v, want empty", env)
	}
}

func TestStreamSubsystemRewritePlan_passthrough(t *testing.T) {
	plan := state.LaunchPlan{
		Command:   "codex --model o3",
		StartDir:  "/repo",
		Subsystem: state.LaunchSubsystemStream,
		Stream: state.StreamLaunchOptions{
			SandboxPolicy:  state.StreamSandboxPolicyExternal,
			ApprovalPolicy: state.StreamApprovalPolicyAutoApprove,
		},
	}
	got, env := streamSubsystem{}.RewritePlan(plan, []byte("hello"))
	if got.Command != plan.Command {
		t.Errorf("command = %q, want %q", got.Command, plan.Command)
	}
	if len(env) != 0 {
		t.Errorf("env = %v, want empty", env)
	}
}

func TestStreamSubsystemRewritePlan_noExtraEnvWhenEmpty(t *testing.T) {
	plan := state.LaunchPlan{
		Command:   "codex",
		Subsystem: state.LaunchSubsystemStream,
	}
	_, env := streamSubsystem{}.RewritePlan(plan, nil)
	if len(env) != 0 {
		t.Errorf("env = %v, want empty", env)
	}
}
