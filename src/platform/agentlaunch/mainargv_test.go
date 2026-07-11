package agentlaunch

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveMainArgv_PrefersArgv(t *testing.T) {
	got, err := ResolveMainArgv([]string{"codex", "--remote"}, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"codex", "--remote"}) {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveMainArgv_TokenizesCommand(t *testing.T) {
	got, err := ResolveMainArgv(nil, `claude --resume "x y"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "claude" || got[2] != "x y" {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveMainArgv_ShellExpandsLoginShell(t *testing.T) {
	got, err := ResolveMainArgv(nil, "shell")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "sh" || got[1] != "-c" {
		t.Fatalf("got %#v", got)
	}
	if !strings.Contains(got[2], "getent passwd") {
		t.Fatalf("shell expansion missing getent: %q", got[2])
	}
}

func TestNormalizePlanForFrameExec_ClearsCommand(t *testing.T) {
	plan, err := NormalizePlanForFrameExec(LaunchPlan{Command: "claude --model x"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Command != "" {
		t.Errorf("Command = %q, want empty", plan.Command)
	}
	if len(plan.Argv) != 3 || plan.Argv[0] != "claude" {
		t.Errorf("Argv = %#v", plan.Argv)
	}
}

func TestNormalizePlanForFrameExec_EmptyErrors(t *testing.T) {
	if _, err := NormalizePlanForFrameExec(LaunchPlan{}); err == nil {
		t.Fatal("expected error")
	}
}
