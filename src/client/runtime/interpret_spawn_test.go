package runtime

import (
	"testing"

	"github.com/takezoh/agent-grid/platform/shellalias"
)

func TestBuildSpawnCommand(t *testing.T) {
	// The bare shell command explicitly execs the user's passwd login shell
	// instead of returning "" (which would defer to a multiplexer default-shell).
	wantShell := "exec " + shellalias.LoginShellCommand + " -l"
	if got := buildSpawnCommand("shell", nil); got != wantShell {
		t.Errorf("shell spawn = %q, want %q", got, wantShell)
	}

	// Non-shell commands are exec'd directly.
	if got := buildSpawnCommand("claude --model sonnet", nil); got != "exec claude --model sonnet" {
		t.Errorf("non-shell spawn = %q, want %q", got, "exec claude --model sonnet")
	}
}
