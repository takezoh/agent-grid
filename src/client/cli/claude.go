package cli

import "github.com/takezoh/agent-reactor/platform/lib/claude"

func init() {
	Register("claude", "Claude Code integration (setup)", claude.Run)
}
