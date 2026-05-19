package cli

import "github.com/takezoh/agent-roost/lib/peers"

func init() {
	Register("peers-mcp", "roost-peers MCP server (stdio)", peers.Run)
}
