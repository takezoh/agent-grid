package agentlaunch

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestManagedClaudeMCPLaunchContract_AppendsConfigLastWithoutMutatingInput(t *testing.T) {
	input := []string{"claude", "--resume", "session-1"}
	got, err := AppendManagedClaudeMCPConfig(input, "/usr/bin/server", "/run/agent-grid/server.sock")
	if err != nil {
		t.Fatalf("AppendManagedClaudeMCPConfig: %v", err)
	}
	if !reflect.DeepEqual(input, []string{"claude", "--resume", "session-1"}) {
		t.Fatalf("input mutated: %#v", input)
	}
	if len(got) != len(input)+2 || got[len(got)-2] != "--mcp-config" {
		t.Fatalf("argv = %#v, want MCP config as final flag/value pair", got)
	}
	var config struct {
		MCPServers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(got[len(got)-1]), &config); err != nil {
		t.Fatalf("config JSON: %v", err)
	}
	entry := config.MCPServers[managedAgentFramesAlias]
	if entry.Type != "stdio" || entry.Command != "/usr/bin/server" {
		t.Fatalf("agent_frames entry = %#v", entry)
	}
	if !reflect.DeepEqual(entry.Args, []string{"agent-frames-mcp", "--sock", "/run/agent-grid/server.sock"}) {
		t.Fatalf("agent_frames args = %#v", entry.Args)
	}
}

func TestManagedClaudeMCPLaunchContract_MissingEndpointLeavesArgvUnchanged(t *testing.T) {
	input := []string{"claude"}
	for _, tc := range []struct {
		name     string
		selfBin  string
		sockPath string
	}{
		{name: "missing executable", sockPath: "/run/server.sock"},
		{name: "missing socket", selfBin: "/usr/bin/server"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AppendManagedClaudeMCPConfig(input, tc.selfBin, tc.sockPath)
			if err != nil {
				t.Fatalf("AppendManagedClaudeMCPConfig: %v", err)
			}
			if !reflect.DeepEqual(got, input) {
				t.Fatalf("argv = %#v, want unchanged %#v", got, input)
			}
		})
	}
}
