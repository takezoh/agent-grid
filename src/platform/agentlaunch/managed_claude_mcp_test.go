package agentlaunch

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewManagedMCPTools_RejectsUnknownTool(t *testing.T) {
	if _, err := NewManagedMCPTools("does_not_exist"); err == nil {
		t.Fatal("unknown tool must fail closed")
	}
}

func TestNewManagedMCPTools_EmptyIsNonMutatingNoop(t *testing.T) {
	set, err := NewManagedMCPTools()
	if err != nil {
		t.Fatalf("empty NewManagedMCPTools: %v", err)
	}
	if !set.Empty() {
		t.Fatal("empty variadic must produce empty set")
	}
	if set.RequiresAuthority() {
		t.Fatal("empty set must not require authority")
	}
	if got := set.Tools(); got != nil {
		t.Fatalf("empty set Tools() = %v, want nil", got)
	}
}

func TestManagedMCPTools_DedupSortAndImmutable(t *testing.T) {
	set, err := NewManagedMCPTools(ManagedMCPToolAgentFrames, ManagedMCPToolAgentFrames)
	if err != nil {
		t.Fatalf("NewManagedMCPTools: %v", err)
	}
	got := set.Tools()
	if len(got) != 1 || got[0] != ManagedMCPToolAgentFrames {
		t.Fatalf("dedup produced %v", got)
	}
	// Mutating the returned slice must not affect the set.
	got[0] = "mutated"
	if snapshot := set.Tools(); len(snapshot) != 1 || snapshot[0] != ManagedMCPToolAgentFrames {
		t.Fatalf("Tools() returned mutable alias, got %v after external mutation", snapshot)
	}
}

func TestManagedMCPTools_AuthorityDerivedFromCatalog(t *testing.T) {
	set := MustNewManagedMCPTools(ManagedMCPToolAgentFrames)
	if !set.RequiresAuthority() {
		t.Fatal("agent_frames must require authority per catalog descriptor")
	}
}

func TestAppendManagedMCPClaudeConfig_AppendsConfigLastWithoutMutatingInput(t *testing.T) {
	input := []string{"claude", "--resume", "session-1"}
	tools := MustNewManagedMCPTools(ManagedMCPToolAgentFrames)
	endpoint := ManagedMCPEndpoint{Executable: "/usr/bin/server", Socket: "/run/agent-grid/server.sock"}
	got, err := AppendManagedMCPClaudeConfig(input, tools, endpoint)
	if err != nil {
		t.Fatalf("AppendManagedMCPClaudeConfig: %v", err)
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
	entry := config.MCPServers["agent_frames"]
	if entry.Type != "stdio" || entry.Command != "/usr/bin/server" {
		t.Fatalf("agent_frames entry = %#v", entry)
	}
	if !reflect.DeepEqual(entry.Args, []string{"agent-frames-mcp", "--sock", "/run/agent-grid/server.sock"}) {
		t.Fatalf("agent_frames args = %#v", entry.Args)
	}
}

func TestAppendManagedMCPClaudeConfig_EmptyToolsLeavesArgvCopy(t *testing.T) {
	input := []string{"claude"}
	got, err := AppendManagedMCPClaudeConfig(input, ManagedMCPTools{}, ManagedMCPEndpoint{})
	if err != nil {
		t.Fatalf("AppendManagedMCPClaudeConfig: %v", err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("argv = %#v, want unchanged %#v", got, input)
	}
	// Mutate returned slice — original must remain untouched (defensive copy).
	got[0] = "mutated"
	if input[0] != "claude" {
		t.Fatalf("empty-tools path returned aliased slice; input mutated to %#v", input)
	}
}

func TestAppendManagedMCPClaudeConfig_MissingEndpointFailsClosed(t *testing.T) {
	tools := MustNewManagedMCPTools(ManagedMCPToolAgentFrames)
	for _, tc := range []struct {
		name string
		e    ManagedMCPEndpoint
	}{
		{"missing executable", ManagedMCPEndpoint{Socket: "/run/server.sock"}},
		{"missing socket", ManagedMCPEndpoint{Executable: "/usr/bin/server"}},
		{"missing both", ManagedMCPEndpoint{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := AppendManagedMCPClaudeConfig([]string{"claude"}, tools, tc.e); err == nil {
				t.Fatal("non-empty tools with invalid endpoint must fail closed")
			}
		})
	}
}

// TestHostContainerDescriptorConformance is the invariant that host and
// container materializers agree on alias, type, and subcommand for every
// tool in the catalog. Only endpoint transport values (executable, socket)
// legitimately differ (design integrated.md FR-009).
func TestHostContainerDescriptorConformance(t *testing.T) {
	tools := MustNewManagedMCPTools(ManagedMCPToolAgentFrames)
	hostEndpoint := ManagedMCPEndpoint{Executable: "/host/bin", Socket: "/host/run/server.sock"}
	containerEndpoint := ManagedMCPEndpoint{Executable: ContainerBinaryPath, Socket: ContainerSockFilePath}

	hostArgv, err := AppendManagedMCPClaudeConfig(nil, tools, hostEndpoint)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	var hostConfig struct {
		MCPServers map[string]struct {
			Type string   `json:"type"`
			Args []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(hostArgv[len(hostArgv)-1]), &hostConfig); err != nil {
		t.Fatalf("host config: %v", err)
	}

	containerAliases, err := RenderManagedMCPAliases(tools, containerEndpoint)
	if err != nil {
		t.Fatalf("container: %v", err)
	}

	for alias, hostEntry := range hostConfig.MCPServers {
		got, ok := containerAliases[alias]
		if !ok {
			t.Errorf("alias %q present in host but missing in container", alias)
			continue
		}
		var containerEntry struct {
			Type string   `json:"type"`
			Args []string `json:"args"`
		}
		if err := json.Unmarshal(got.Value, &containerEntry); err != nil {
			t.Errorf("container %q value: %v", alias, err)
			continue
		}
		if hostEntry.Type != containerEntry.Type {
			t.Errorf("%s: host type %q != container type %q", alias, hostEntry.Type, containerEntry.Type)
		}
		if len(hostEntry.Args) == 0 || len(containerEntry.Args) == 0 {
			t.Errorf("%s: missing args", alias)
			continue
		}
		if hostEntry.Args[0] != containerEntry.Args[0] {
			t.Errorf("%s: host subcommand %q != container subcommand %q", alias, hostEntry.Args[0], containerEntry.Args[0])
		}
	}
}
