package cli

import (
	"fmt"
	"strings"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
)

const (
	driverName      = "claude"
	sandboxSkipFlag = "--allow-dangerously-skip-permissions"
	autoModeFlag    = "--enable-auto-mode"
)

type CommandConfig struct {
	CommandBin string
	Model      string
	Effort     string
}

func ParseCommand(argv []string) (CommandConfig, error) {
	if len(argv) == 0 || argv[0] != driverName {
		return CommandConfig{}, fmt.Errorf("claude: unsupported command %q", strings.Join(argv, " "))
	}
	cfg := CommandConfig{CommandBin: driverName}
	for i := 1; i < len(argv); i++ {
		switch argv[i] {
		case "--model":
			if i+1 < len(argv) {
				cfg.Model = argv[i+1]
				i++
			}
		case "--effort":
			if i+1 < len(argv) {
				cfg.Effort = argv[i+1]
				i++
			}
		default:
			switch {
			case strings.HasPrefix(argv[i], "--model="):
				cfg.Model = strings.TrimSpace(strings.TrimPrefix(argv[i], "--model="))
			case strings.HasPrefix(argv[i], "--effort="):
				cfg.Effort = strings.TrimSpace(strings.TrimPrefix(argv[i], "--effort="))
			}
		}
	}
	return cfg, nil
}

// SandboxFlags enforces sandbox-required flag adjustments on a command string:
//   - strips --enable-auto-mode (conflicts with bypass-permissions semantics)
//   - appends --allow-dangerously-skip-permissions unless already present
//
// Returns command unchanged when sandboxed is false.
func SandboxFlags(command string, sandboxed bool) string {
	if !sandboxed {
		return command
	}
	command = stripToken(command, autoModeFlag)
	if hasToken(command, sandboxSkipFlag) {
		return command
	}
	return strings.TrimSpace(command) + " " + sandboxSkipFlag
}

// AppServerArgs returns the claude CLI argv for a non-interactive prompt turn.
// --verbose is mandatory: current claude versions reject -p --output-format stream-json without it.
// When resumeSessionID is non-empty, --resume <id> is appended before the prompt.
func AppServerArgs(resumeSessionID, appendSystemPrompt, prompt string) []string {
	args := []string{"-p", "--output-format", "stream-json", "--verbose"}
	if appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", appendSystemPrompt)
	}
	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}
	return append(args, prompt)
}

// hasToken returns true when command contains the exact flag as a whitespace-delimited token.
func hasToken(command, flag string) bool {
	argv, err := agentlaunch.SplitArgs(strings.TrimSpace(command))
	if err != nil {
		return false
	}
	for _, f := range argv {
		if f == flag {
			return true
		}
	}
	return false
}

// stripToken removes exact flag tokens from command; "--flag=value" form is left intact.
func stripToken(command, flag string) string {
	parts, err := agentlaunch.LexArgs(strings.TrimSpace(command))
	if err != nil {
		return strings.TrimSpace(command)
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Value != flag {
			out = append(out, p.Raw)
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}
