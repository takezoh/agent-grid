package agentlaunch

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/takezoh/agent-grid/services/platform/mcpoverlay"
)

// ManagedMCPTool identifies a single managed MCP endpoint kind. Values that
// are not present in managedMCPCatalog are rejected by NewManagedMCPTools.
type ManagedMCPTool string

const (
	// ManagedMCPToolAgentFrames exposes the frame-scoped agent_frames MCP server.
	ManagedMCPToolAgentFrames ManagedMCPTool = "agent_frames"
)

// managedMCPDescriptor captures the shared catalog entry for one tool: how it
// is exposed to the child process (alias / stdio type / subcommand), whether it
// overrides an existing user alias on collision, and whether the host must mint
// a frame-scoped AG_SOCKET_TOKEN when this tool is present.
type managedMCPDescriptor struct {
	// Alias is the mcpServers key the launched agent sees.
	Alias string
	// Type is the mcpServers entry `type` value; currently always "stdio".
	Type string
	// HostSubcommand is the first argv token after the transport binary path.
	HostSubcommand string
	// Override, when true, replaces any existing entry with the same alias in
	// the merged .mcp.json base. When false, an existing user entry wins.
	Override bool
	// RequiresAuthority, when true, means the launched process must receive a
	// frame-scoped AG_SOCKET_TOKEN so agent_frames MCP calls can authenticate.
	RequiresAuthority bool
}

// managedMCPCatalog is the single production catalog for validated managed MCP
// endpoints. Adding a new kind requires an entry here plus a corresponding
// ManagedMCPTool constant.
var managedMCPCatalog = map[ManagedMCPTool]managedMCPDescriptor{
	ManagedMCPToolAgentFrames: {
		Alias:             "agent_frames",
		Type:              "stdio",
		HostSubcommand:    "agent-frames-mcp",
		Override:          false,
		RequiresAuthority: true,
	},
}

// ManagedMCPTools is an immutable, validated set of managed MCP tool kinds.
// The zero value is the empty set (no managed tools requested). Construct
// non-empty sets with NewManagedMCPTools; the constructor rejects any tool
// value that is not registered in managedMCPCatalog.
type ManagedMCPTools struct {
	// tools is a sorted, deduplicated slice of validated tools. Kept
	// unexported so callers cannot mutate the set post-construction.
	tools []ManagedMCPTool
}

// NewManagedMCPTools returns a validated set containing tools. Duplicates are
// collapsed. Passing zero tools returns the empty set with a nil error.
// Passing any tool not registered in the catalog returns an error and the
// zero-value set — callers should treat this as fail-closed.
func NewManagedMCPTools(tools ...ManagedMCPTool) (ManagedMCPTools, error) {
	if len(tools) == 0 {
		return ManagedMCPTools{}, nil
	}
	seen := make(map[ManagedMCPTool]struct{}, len(tools))
	for _, t := range tools {
		if _, ok := managedMCPCatalog[t]; !ok {
			return ManagedMCPTools{}, fmt.Errorf("agentlaunch: unknown managed MCP tool %q", string(t))
		}
		seen[t] = struct{}{}
	}
	out := make([]ManagedMCPTool, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return ManagedMCPTools{tools: out}, nil
}

// MustNewManagedMCPTools panics if any input is unknown. Reserved for
// compile-time-known-good sets (e.g. package-level container defaults).
func MustNewManagedMCPTools(tools ...ManagedMCPTool) ManagedMCPTools {
	set, err := NewManagedMCPTools(tools...)
	if err != nil {
		panic(err)
	}
	return set
}

// Empty reports whether the set contains no tools.
func (m ManagedMCPTools) Empty() bool { return len(m.tools) == 0 }

// Contains reports whether t is a member of the set.
func (m ManagedMCPTools) Contains(t ManagedMCPTool) bool {
	for _, existing := range m.tools {
		if existing == t {
			return true
		}
	}
	return false
}

// Tools returns a defensive copy of the sorted, deduplicated tool list.
func (m ManagedMCPTools) Tools() []ManagedMCPTool {
	if len(m.tools) == 0 {
		return nil
	}
	out := make([]ManagedMCPTool, len(m.tools))
	copy(out, m.tools)
	return out
}

// RequiresAuthority reports whether any tool in the set demands that the host
// mint an AG_SOCKET_TOKEN before spawn. The empty set does not require authority.
func (m ManagedMCPTools) RequiresAuthority() bool {
	for _, t := range m.tools {
		if managedMCPCatalog[t].RequiresAuthority {
			return true
		}
	}
	return false
}

// Equal reports whether two sets contain the same tools. Order of construction
// arguments is irrelevant because the underlying storage is sorted.
func (m ManagedMCPTools) Equal(other ManagedMCPTools) bool {
	if len(m.tools) != len(other.tools) {
		return false
	}
	for i, t := range m.tools {
		if other.tools[i] != t {
			return false
		}
	}
	return true
}

// DefaultContainerManagedMCPTools is the container-scope default set applied
// to every workspace target by the devcontainer overlay. It is deliberately
// independent of any frame LaunchPlan (FR-011): container provisioning must
// not inspect a frame plan to decide which managed tools to expose.
var DefaultContainerManagedMCPTools = MustNewManagedMCPTools(ManagedMCPToolAgentFrames)

// ManagedMCPEndpoint describes the transport target used to reach the managed
// MCP process. Both fields must be non-empty when tools is non-empty; otherwise
// the endpoint fails validation and rendering fails closed.
type ManagedMCPEndpoint struct {
	// Executable is the absolute path to the binary that hosts the managed MCP
	// subcommand. On the host this is the daemon binary; inside the container
	// it is the bridge helper installed at ContainerBinaryPath.
	Executable string
	// Socket is the absolute UDS path the managed MCP subcommand dials for
	// bearer-token authentication. On the host this is the daemon socket; inside
	// the container it is ContainerSockFilePath.
	Socket string
}

// Validate reports the first missing field. Endpoint values are treated as
// missing when empty or whitespace-only.
func (e ManagedMCPEndpoint) Validate() error {
	if strings.TrimSpace(e.Executable) == "" {
		return errors.New("agentlaunch: managed MCP endpoint missing executable path")
	}
	if strings.TrimSpace(e.Socket) == "" {
		return errors.New("agentlaunch: managed MCP endpoint missing socket path")
	}
	return nil
}

// AppendManagedMCPClaudeConfig returns argv with a trailing `--mcp-config <json>`
// pair encoding the requested tools. The empty set returns a copy of argv with
// no additional arguments and no endpoint validation. A non-empty set with an
// invalid endpoint returns an error (fail-closed, before any spawn or token
// side-effect).
//
// The `--mcp-config` flag is appended last because Claude's flag parser
// consumes every remaining argument for --mcp-config.
func AppendManagedMCPClaudeConfig(argv []string, tools ManagedMCPTools, endpoint ManagedMCPEndpoint) ([]string, error) {
	out := append([]string(nil), argv...)
	if tools.Empty() {
		return out, nil
	}
	if err := endpoint.Validate(); err != nil {
		return nil, err
	}
	servers := make(map[string]any, len(tools.tools))
	for _, t := range tools.tools {
		d := managedMCPCatalog[t]
		servers[d.Alias] = map[string]any{
			"type":    d.Type,
			"command": endpoint.Executable,
			"args":    []string{d.HostSubcommand, "--sock", endpoint.Socket},
		}
	}
	config, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return nil, fmt.Errorf("agentlaunch: marshal managed MCP config: %w", err)
	}
	return append(out, "--mcp-config", string(config)), nil
}

// RenderManagedMCPAliases returns the mcpServers alias entries for a validated
// tool set, keyed by descriptor alias. Empty sets return (nil, nil). Non-empty
// sets require a valid endpoint; missing endpoint values fail closed.
func RenderManagedMCPAliases(tools ManagedMCPTools, endpoint ManagedMCPEndpoint) (map[string]mcpoverlay.AliasEntry, error) {
	if tools.Empty() {
		return nil, nil
	}
	if err := endpoint.Validate(); err != nil {
		return nil, err
	}
	out := make(map[string]mcpoverlay.AliasEntry, len(tools.tools))
	for _, t := range tools.tools {
		d := managedMCPCatalog[t]
		entry, err := json.Marshal(map[string]any{
			"type":    d.Type,
			"command": endpoint.Executable,
			"args":    []string{d.HostSubcommand, "--sock", endpoint.Socket},
		})
		if err != nil {
			return nil, fmt.Errorf("agentlaunch: marshal %q alias: %w", d.Alias, err)
		}
		out[d.Alias] = mcpoverlay.AliasEntry{Value: entry, Override: d.Override}
	}
	return out, nil
}
