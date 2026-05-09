package cli

import "github.com/takezoh/agent-roost/lib/claude"

func init() {
	Register("claude", "Claude Code integration (setup)", claude.Run)
}
