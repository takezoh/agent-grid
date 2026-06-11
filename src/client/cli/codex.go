package cli

import "github.com/takezoh/agent-reactor/platform/lib/codex"

func init() {
	Register("codex", "Codex CLI integration (setup)", codex.Run)
}
