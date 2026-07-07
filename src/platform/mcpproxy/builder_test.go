package mcpproxy

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/platform/config"
)

func TestSpecBuilderBasics(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	cfg := Config{
		RunBase:           filepath.Join(dir, "run"),
		ContainerSockPath: "/tmp/in/mcp.sock",
		ContainerBinPath:  "/usr/local/bin/roost",
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{}
	})
	if b.Name() != "mcpproxy" {
		t.Errorf("Name = %q", b.Name())
	}
	if b.Routes() != nil {
		t.Errorf("Routes should be nil")
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.RunBase); err != nil {
		t.Errorf("RunBase not created: %v", err)
	}
}

func TestSpecBuilderEmptyServers(t *testing.T) {
	ctx := t.Context()
	b := NewSpecBuilder(ctx, Config{}, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{}
	})
	spec, err := b.ContainerSpec(ctx, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Mounts) != 0 || len(spec.Env) != 0 {
		t.Errorf("expected empty spec, got %+v", spec)
	}
}

func TestSpecBuilderManagedAgentFramesWithoutBrokerServers(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/tmp/incontainer/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(p string) []WorkspaceTarget {
			return []WorkspaceTarget{{HostRoot: p, ContainerWS: "/workspace"}}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{}
	})
	spec, err := b.ContainerSpec(ctx, "/myproj")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Env) != 0 {
		t.Fatalf("expected no broker env, got %+v", spec.Env)
	}
	if len(spec.Mounts) != 1 {
		t.Fatalf("expected 1 overlay mount, got %d: %+v", len(spec.Mounts), spec.Mounts)
	}
	srcs := overlaySources(spec.Mounts)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 overlay source, got %v", srcs)
	}
	raw, err := os.ReadFile(srcs[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.MCPServers["agent_frames"]; !ok {
		t.Fatalf("managed agent_frames missing from overlay: %v", doc.MCPServers)
	}
}

func TestSpecBuilderWithServers(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/tmp/incontainer/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(p string) []WorkspaceTarget {
			return []WorkspaceTarget{{HostRoot: p, ContainerWS: "/workspace"}}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
			"obs": {Command: "true"},
		}}
	})
	spec, err := b.ContainerSpec(ctx, "/myproj")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Env["AG_MCP_SOCK"] != cfg.ContainerSockPath {
		t.Errorf("env AG_MCP_SOCK = %q", spec.Env["AG_MCP_SOCK"])
	}
	if len(spec.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d: %+v", len(spec.Mounts), spec.Mounts)
	}
	if !hasMountTarget(spec.Mounts, "/workspace/.mcp.json") {
		t.Errorf("expected .mcp.json overlay at /workspace/.mcp.json, got %+v", spec.Mounts)
	}
	// Second call must be idempotent (broker already exists).
	if _, err := b.ContainerSpec(ctx, "/myproj"); err != nil {
		t.Fatal(err)
	}
}

// TestSpecBuilderSharedWorkspaces verifies a shared container fans the .mcp.json
// overlay out to one absolute target per bound project, with distinct host
// source files so the overlays don't clobber each other.
func TestSpecBuilderSharedWorkspaces(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/opt/agent-grid/run/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(string) []WorkspaceTarget {
			return []WorkspaceTarget{
				{HostRoot: "/host/a", ContainerWS: "/workspace/a"},
				{HostRoot: "/host/b", ContainerWS: "/workspace/b"},
			}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{"obs": {Command: "true"}}}
	})
	spec, err := b.ContainerSpec(ctx, "__shared__")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Mounts) != 3 { // sock + 2 overlays
		t.Fatalf("expected 3 mounts, got %d: %+v", len(spec.Mounts), spec.Mounts)
	}
	for _, want := range []string{"/workspace/a/.mcp.json", "/workspace/b/.mcp.json"} {
		if !hasMountTarget(spec.Mounts, want) {
			t.Errorf("missing overlay target %q in %+v", want, spec.Mounts)
		}
	}
	srcs := overlaySources(spec.Mounts)
	if len(srcs) == 2 && srcs[0] == srcs[1] {
		t.Errorf("overlay host sources collide: %+v", srcs)
	}
}

// TestSpecBuilderSkipsNonAbsoluteTarget is the regression test for the shared
// container that failed `docker create`: an unresolved sentinel (non-absolute
// container path) must be dropped, never emitted as a relative mount target.
func TestSpecBuilderSkipsNonAbsoluteTarget(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/opt/agent-grid/run/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(string) []WorkspaceTarget {
			return []WorkspaceTarget{{HostRoot: "/host/x", ContainerWS: "__shared__"}}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{"obs": {Command: "true"}}}
	})
	spec, err := b.ContainerSpec(ctx, "__shared__")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Mounts) != 1 { // only the sock mount survives
		t.Fatalf("expected only the sock mount, got %d: %+v", len(spec.Mounts), spec.Mounts)
	}
	if strings.Contains(spec.Mounts[0], ".mcp.json") {
		t.Errorf("relative overlay should be skipped, got %q", spec.Mounts[0])
	}
}

// TestSpecBuilderDefaultsToProjectPath verifies that with no resolver the
// overlay lands at the project path itself.
func TestSpecBuilderDefaultsToProjectPath(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/opt/agent-grid/run/mcp.sock",
		ContainerBinPath:  "/bin/roost",
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{"obs": {Command: "true"}}}
	})
	spec, err := b.ContainerSpec(ctx, "/myproj")
	if err != nil {
		t.Fatal(err)
	}
	if !hasMountTarget(spec.Mounts, "/myproj/.mcp.json") {
		t.Errorf("expected overlay at /myproj/.mcp.json, got %+v", spec.Mounts)
	}
}

// TestSpecBuilder_MergesProjectMCPJSON exercises the overlayMounts→writeMCPJSON
// wiring end-to-end with a real HostRoot/.mcp.json: its servers must be merged
// into the generated overlay (preserved) and broker aliases overridden. The
// other ContainerSpec tests use non-existent HostRoots, so the merge base is
// always empty and this path goes unverified.
func TestSpecBuilder_MergesProjectMCPJSON(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	hostRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostRoot, ".mcp.json"),
		[]byte(`{"mcpServers":{"existing":{"command":"other"},"obs":{"command":"old"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/opt/agent-grid/run/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(string) []WorkspaceTarget {
			return []WorkspaceTarget{{HostRoot: hostRoot, ContainerWS: "/workspace/app"}}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{"obs": {Command: "true"}}}
	})
	spec, err := b.ContainerSpec(ctx, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	srcs := overlaySources(spec.Mounts)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 overlay source, got %v", srcs)
	}
	raw, err := os.ReadFile(srcs[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.MCPServers["existing"]; !ok {
		t.Errorf("project server 'existing' must survive ContainerSpec, got %v", doc.MCPServers)
	}
	var obs struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(doc.MCPServers["obs"], &obs); err != nil {
		t.Fatal(err)
	}
	if obs.Command != "/bin/roost" {
		t.Errorf("obs must be overridden by broker shim, command = %q", obs.Command)
	}
}

func TestSpecBuilder_PreservesExistingAgentFramesAlias(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	hostRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostRoot, ".mcp.json"),
		[]byte(`{"mcpServers":{"agent_frames":{"command":"custom-agent-frames"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/opt/agent-grid/run/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceTargetsFor: func(string) []WorkspaceTarget {
			return []WorkspaceTarget{{HostRoot: hostRoot, ContainerWS: "/workspace/app"}}
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{"obs": {Command: "true"}}}
	})
	spec, err := b.ContainerSpec(ctx, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	srcs := overlaySources(spec.Mounts)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 overlay source, got %v", srcs)
	}
	raw, err := os.ReadFile(srcs[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var entry struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(doc.MCPServers["agent_frames"], &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Command != "custom-agent-frames" {
		t.Fatalf("agent_frames.command = %q, want custom-agent-frames", entry.Command)
	}
}

// hasMountTarget reports whether any readonly overlay mount targets target.
func hasMountTarget(mounts []string, target string) bool {
	for _, m := range mounts {
		if strings.Contains(m, "target="+target+",") {
			return true
		}
	}
	return false
}

// overlaySources returns the host source paths of the readonly .mcp.json mounts.
func overlaySources(mounts []string) []string {
	var out []string
	for _, m := range mounts {
		if !strings.HasSuffix(m, ",readonly") {
			continue
		}
		for _, part := range strings.Split(m, ",") {
			if s, ok := strings.CutPrefix(part, "source="); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func TestCompileServerEmptyCommand(t *testing.T) {
	_, err := compileServer("x", "", nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestCompileServerEnvOverride(t *testing.T) {
	t.Setenv("FOO", "baseline")
	srv, err := compileServer("x", "true", []string{"-v"}, map[string]string{"FOO": "override", "NEW": "v"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundOverride := false
	foundNew := false
	for _, kv := range srv.env {
		if kv == "FOO=override" {
			foundOverride = true
		}
		if kv == "FOO=baseline" {
			t.Errorf("baseline FOO leaked despite override")
		}
		if kv == "NEW=v" {
			foundNew = true
		}
	}
	if !foundOverride || !foundNew {
		t.Errorf("env missing overrides: %v", srv.env)
	}
}

func TestCompileServers(t *testing.T) {
	cfg := config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
		"a": {Command: "true"},
		"b": {Command: "true"},
	}}
	got, err := compileServers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestCompileServersError(t *testing.T) {
	cfg := config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
		"a": {Command: ""},
	}}
	if _, err := compileServers(cfg); err == nil {
		t.Error("expected error")
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode("p", "a", nil); got != 0 {
		t.Errorf("nil err: %d", got)
	}
	if got := exitCode("p", "a", errors.New("boom")); got != 1 {
		t.Errorf("plain err: %d", got)
	}
	// Real ExitError via /bin/false
	cmd := exec.Command("false")
	err := cmd.Run()
	if got := exitCode("p", "a", err); got == 0 {
		t.Errorf("exit error should be non-zero, got %d", got)
	}
}
