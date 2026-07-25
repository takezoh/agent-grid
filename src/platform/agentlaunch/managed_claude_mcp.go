package agentlaunch

import (
	"encoding/json"
	"fmt"
	"strings"
)

const managedClaudeMCPConfigFlag = "--mcp-config"

// AppendManagedClaudeMCPConfig exposes the frame-scoped agent_frames server to
// Claude without changing process-wide environment such as HOME. The config
// flag is appended last because Claude's --mcp-config accepts multiple values
// and therefore consumes all remaining arguments.
func AppendManagedClaudeMCPConfig(argv []string, selfBin, sockPath string) ([]string, error) {
	out := append([]string(nil), argv...)
	if strings.TrimSpace(selfBin) == "" || strings.TrimSpace(sockPath) == "" {
		return out, nil
	}
	config, err := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			managedAgentFramesAlias: map[string]any{
				"type":    "stdio",
				"command": selfBin,
				"args":    []string{"agent-frames-mcp", "--sock", sockPath},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("managed claude MCP config: marshal: %w", err)
	}
	return append(out, managedClaudeMCPConfigFlag, string(config)), nil
}
