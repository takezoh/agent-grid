package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-grid/platform/appid"
	"github.com/takezoh/agent-grid/platform/config"
	"github.com/takezoh/agent-grid/platform/mcpoverlay"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

// SpecBuilder implements container.Provider for MCP server proxying.
// It starts a per-project Unix socket broker that runs allowlisted MCP servers
// on the host, relaying JSON-RPC stdio with tool-level policy enforcement.
type SpecBuilder struct {
	ctx    context.Context
	cfg    Config
	cfgFor func(projectPath string) config.MCPProxyConfig

	mu      sync.Mutex
	brokers map[string]*broker
}

// Config holds path configuration for the SpecBuilder.
type Config struct {
	RunBase           string // parent of per-project run directories on the host
	ContainerSockPath string // mcp.sock path inside the container
	ContainerBinPath  string // client binary path inside the container
	// WorkspaceTargetsFor returns the workspaces into which the .mcp.json
	// overlay must be projected for a project key. A single-project container
	// yields one target; a shared container yields one per bound project.
	WorkspaceTargetsFor func(projectKey string) []WorkspaceTarget
}

// WorkspaceTarget describes one workspace receiving the .mcp.json overlay.
// HostRoot is the host-side project root whose existing .mcp.json seeds the
// merge. ContainerWS is the container-side workspace path where the merged
// .mcp.json is bind-mounted and MUST be absolute — Docker rejects relative
// mount targets, and a non-absolute value signals an unresolved sentinel
// (such as the shared-container key) that must never reach `docker create`.
type WorkspaceTarget struct {
	HostRoot    string
	ContainerWS string
}

// NewSpecBuilder creates a SpecBuilder.
func NewSpecBuilder(ctx context.Context, cfg Config, cfgFor func(string) config.MCPProxyConfig) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		cfg:     cfg,
		cfgFor:  cfgFor,
		brokers: make(map[string]*broker),
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "mcpproxy" }

func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.cfg.RunBase, 0o700)
}

func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project MCP broker, generates a
// .mcp.json shim file, and returns mounts for both the broker socket and the
// .mcp.json overlay. Returns an empty Spec when no servers are configured.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string) (container.Spec, error) {
	cfg := b.cfgFor(projectPath)
	targets := b.workspaceTargets(projectPath)
	servers := b.brokerServers(cfg)
	if len(servers) == 0 && b.cfg.ContainerBinPath == "" {
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("mcpproxy: mkdir run dir: %w", err)
	}

	var mounts []string
	if len(servers) > 0 {
		if err := b.ensureBroker(projectPath, projRunDir, servers); err != nil {
			return container.Spec{}, err
		}
		sockHostPath := filepath.Join(projRunDir, filepath.Base(b.cfg.ContainerSockPath))
		mounts = append(mounts, fmt.Sprintf("type=bind,source=%s,target=%s", sockHostPath, b.cfg.ContainerSockPath))
	}

	overlayMounts, err := b.overlayMounts(projRunDir, projectPath, targets, servers)
	if err != nil {
		return container.Spec{}, err
	}
	mounts = append(mounts, overlayMounts...)

	spec := container.Spec{Mounts: mounts}
	if len(servers) > 0 {
		spec.Env = map[string]string{"AG_MCP_SOCK": b.cfg.ContainerSockPath}
	}
	return spec, nil
}

func (b *SpecBuilder) brokerServers(cfg config.MCPProxyConfig) map[string]config.MCPProxyServer {
	out := make(map[string]config.MCPProxyServer, len(cfg.Servers)+1)
	for alias, server := range cfg.Servers {
		out[alias] = server
	}
	return out
}

// overlayMounts writes one merged .mcp.json per workspace target under projRunDir
// and returns the readonly bind-mount specs. Targets whose container-side path is
// not absolute are skipped with a warning rather than emitted — Docker rejects a
// relative mount target, and a non-absolute value means a sentinel (e.g. the
// shared-container key) leaked through unresolved.
func (b *SpecBuilder) overlayMounts(projRunDir, projectKey string, targets []WorkspaceTarget, servers map[string]config.MCPProxyServer) ([]string, error) {
	mounts := make([]string, 0, len(targets))
	for _, t := range targets {
		if !filepath.IsAbs(t.ContainerWS) {
			slog.Warn("mcpproxy: skip .mcp.json overlay for non-absolute workspace target",
				"projectKey", projectKey, "containerWS", t.ContainerWS)
			continue
		}
		hostMCP := filepath.Join(projRunDir, mcpJSONFileName(t.ContainerWS))
		if err := writeMCPJSON(hostMCP, t.HostRoot+"/.mcp.json", servers, b.cfg.ContainerBinPath); err != nil {
			return nil, fmt.Errorf("mcpproxy: write mcp.json: %w", err)
		}
		mounts = append(mounts, fmt.Sprintf("type=bind,source=%s,target=%s,readonly", hostMCP, t.ContainerWS+"/.mcp.json"))
	}
	return mounts, nil
}

// workspaceTargets resolves the .mcp.json overlay targets for a project key,
// defaulting to the project itself when no resolver is configured.
func (b *SpecBuilder) workspaceTargets(projectKey string) []WorkspaceTarget {
	if b.cfg.WorkspaceTargetsFor != nil {
		return b.cfg.WorkspaceTargetsFor(projectKey)
	}
	return []WorkspaceTarget{{HostRoot: projectKey, ContainerWS: projectKey}}
}

// mcpJSONFileName derives a per-workspace host filename so multiple targets in a
// shared container don't clobber one another's merged overlay.
func mcpJSONFileName(containerWS string) string {
	return "mcp-" + container.ProjectRunHash(containerWS) + ".json"
}

// writeMCPJSON writes a merged .mcp.json to path.
// It reads projectMCPJSON (the project's .mcp.json) as a base, then overlays
// shim entries for each alias so the broker aliases shadow any direct entries.
// Entries not in servers pass through unchanged.
// Skips the write when the file already contains identical content.
func writeMCPJSON(path, projectMCPJSON string, servers map[string]config.MCPProxyServer, containerBin string) error {
	aliases := make(map[string]mcpoverlay.AliasEntry, len(servers)+1)
	for alias := range servers {
		shim, err := json.Marshal(map[string]any{
			"type":    "stdio",
			"command": containerBin,
			"args":    []string{"mcp-exec", alias},
		})
		if err != nil {
			return err
		}
		aliases[alias] = mcpoverlay.AliasEntry{Value: shim, Override: true}
	}
	if containerBin != "" {
		entry, err := json.Marshal(map[string]any{
			"type":    "stdio",
			"command": containerBin,
			"args":    []string{"agent-frames-mcp", "--sock", appid.ContainerSockFilePath},
		})
		if err != nil {
			return err
		}
		aliases["agent_frames"] = mcpoverlay.AliasEntry{Value: entry}
	}
	return mcpoverlay.WriteJSON(path, projectMCPJSON, aliases)
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, serversCfg map[string]config.MCPProxyServer) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.brokers[projectPath]; ok {
		return nil
	}

	servers, err := compileServers(config.MCPProxyConfig{Servers: serversCfg})
	if err != nil {
		return err
	}

	sockPath := filepath.Join(projRunDir, filepath.Base(b.cfg.ContainerSockPath))
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("mcpproxy: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:     b.ctx,
		sock:    sockPath,
		ln:      ln,
		project: projectPath,
		servers: servers,
		onStop: func() {
			b.mu.Lock()
			delete(b.brokers, projectPath)
			b.mu.Unlock()
		},
	}
	b.brokers[projectPath] = br
	go br.serve()
	return nil
}

func compileServers(cfg config.MCPProxyConfig) (map[string]*serverEntry, error) {
	m := make(map[string]*serverEntry, len(cfg.Servers))
	for alias, s := range cfg.Servers {
		e, err := compileServer(alias, s.Command, s.Args, s.Env, s.Allow, s.Deny)
		if err != nil {
			return nil, err
		}
		m[alias] = e
	}
	return m, nil
}

func (b *SpecBuilder) watchShutdown(ctx context.Context) {
	<-ctx.Done()
	b.mu.Lock()
	listeners := make([]net.Listener, 0, len(b.brokers))
	for _, br := range b.brokers {
		listeners = append(listeners, br.ln)
	}
	b.mu.Unlock()
	for _, ln := range listeners {
		ln.Close()
	}
}
