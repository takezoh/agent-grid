package cli

import "github.com/takezoh/agent-reactor/platform/lib/gemini"

func init() {
	Register("gemini", "Gemini CLI integration (setup)", gemini.Run)
}
