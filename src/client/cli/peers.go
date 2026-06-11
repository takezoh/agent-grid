package cli

import (
	"github.com/takezoh/agent-reactor/client/lib/peers"
	"github.com/takezoh/agent-reactor/platform/appid"
)

func init() {
	Register("peers-mcp", appid.PeersServer+" MCP server (stdio)", peers.Run)
}
