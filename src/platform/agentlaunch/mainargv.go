package agentlaunch

import (
	"fmt"
	"strings"

	"github.com/takezoh/agent-grid/platform/shellalias"
)

// ResolveMainArgv returns the argv that frame-exec will syscall.Exec as MainCommand.
// Argv wins when non-empty; otherwise Command is tokenized. The sentinel
// Command "shell" expands to an interactive login-shell session (passwd-based
// shell lookup via shellalias.LoginShellCommand).
//
// Every host / container launch goes through frame-exec; there is no parallel
// "shell-wrap the agent command" path.
func ResolveMainArgv(argv []string, command string) ([]string, error) {
	if len(argv) > 0 {
		return argv, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("agentlaunch: empty main command (Argv and Command both empty)")
	}
	if command == "shell" {
		// Same login-shell expansion previously inlined into docker exec shell wrap.
		return []string{"sh", "-c", "exec " + shellalias.LoginShellCommand + " -l"}, nil
	}
	args, err := SplitArgs(command)
	if err != nil {
		return nil, fmt.Errorf("agentlaunch: tokenize Command: %w", err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("agentlaunch: Command tokenized to empty argv")
	}
	return args, nil
}

// NormalizePlanForFrameExec fills plan.Argv from Command when needed and clears
// Command so callers always hand frame-exec a structured MainCommand.
func NormalizePlanForFrameExec(plan LaunchPlan) (LaunchPlan, error) {
	main, err := ResolveMainArgv(plan.Argv, plan.Command)
	if err != nil {
		return LaunchPlan{}, err
	}
	plan.Argv = main
	plan.Command = ""
	return plan, nil
}
