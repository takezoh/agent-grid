package cli

import (
	"fmt"
	"strings"
)

const (
	driverName = "claude"
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
