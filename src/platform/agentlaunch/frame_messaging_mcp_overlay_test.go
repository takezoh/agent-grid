package agentlaunch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/platform/mcpproxy"
)

func TestMergeManagedContainerMCPMounts_AddsBuiltInToProxyOverlay(t *testing.T) {
	runDir := t.TempDir()
	base := filepath.Join(t.TempDir(), "proxy.mcp.json")
	if err := os.WriteFile(base, []byte(`{"mcpServers":{"obs":{"command":"`+ContainerBinaryPath+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mounts, err := mergeManagedContainerMCPMounts(runDir, []string{
		"type=bind,source=" + base + ",target=/workspace/app/.mcp.json,readonly",
	}, []mcpproxy.WorkspaceTarget{{HostRoot: t.TempDir(), ContainerWS: "/workspace/app"}},
		DefaultContainerManagedMCPTools)
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 {
		t.Fatalf("mount count = %d, want 1", len(mounts))
	}
	overlay := mountField(mounts[0], "source")
	raw, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.MCPServers["obs"]; !ok {
		t.Fatalf("proxy overlay entry missing: %v", doc.MCPServers)
	}
	var entry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(doc.MCPServers["agent_frames"], &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Command != ContainerBinaryPath {
		t.Fatalf("agent_frames.command = %q, want %q", entry.Command, ContainerBinaryPath)
	}
	if len(entry.Args) != 3 || entry.Args[0] != "agent-frames-mcp" || entry.Args[1] != "--sock" || entry.Args[2] != ContainerSockFilePath {
		t.Fatalf("agent_frames.args = %v", entry.Args)
	}
}

func TestMergeManagedContainerMCPMounts_PreservesWorkspaceCustomAlias(t *testing.T) {
	runDir := t.TempDir()
	hostA := t.TempDir()
	hostB := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostA, ".mcp.json"),
		[]byte(`{"mcpServers":{"agent_frames":{"command":"custom-agent-frames"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	baseB := filepath.Join(t.TempDir(), "proxy-b.mcp.json")
	if err := os.WriteFile(baseB, []byte(`{"mcpServers":{"obs":{"command":"bridge"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mounts, err := mergeManagedContainerMCPMounts(runDir, []string{
		"type=bind,source=" + baseB + ",target=/workspace/b/.mcp.json,readonly",
	}, []mcpproxy.WorkspaceTarget{
		{HostRoot: hostA, ContainerWS: "/workspace/a"},
		{HostRoot: hostB, ContainerWS: "/workspace/b"},
	}, DefaultContainerManagedMCPTools)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, mount := range mounts {
		got[mountField(mount, "target")] = mountField(mount, "source")
	}
	for _, target := range []string{"/workspace/a/.mcp.json", "/workspace/b/.mcp.json"} {
		if got[target] == "" {
			t.Fatalf("missing mount for %s in %v", target, mounts)
		}
	}

	readEntry := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
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
		return entry.Command
	}

	if got := readEntry(got["/workspace/a/.mcp.json"]); got != "custom-agent-frames" {
		t.Fatalf("workspace a command = %q, want custom-agent-frames", got)
	}
	if got := readEntry(got["/workspace/b/.mcp.json"]); got != ContainerBinaryPath {
		t.Fatalf("workspace b command = %q, want %q", got, ContainerBinaryPath)
	}
}

func TestMergeManagedContainerMCPMounts_EmptyToolSetIsNoOp(t *testing.T) {
	runDir := t.TempDir()
	input := []string{"type=bind,source=/tmp/x,target=/workspace/app/.mcp.json,readonly"}
	mounts, err := mergeManagedContainerMCPMounts(runDir, input,
		[]mcpproxy.WorkspaceTarget{{HostRoot: "/host", ContainerWS: "/workspace/app"}},
		ManagedMCPTools{})
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != len(input) || mounts[0] != input[0] {
		t.Fatalf("empty tool set must return mounts unchanged, got %v (want %v)", mounts, input)
	}
}

func TestMergeManagedContainerMCPMounts_SkipsNonAbsoluteTarget(t *testing.T) {
	runDir := t.TempDir()
	mounts, err := mergeManagedContainerMCPMounts(runDir, nil,
		[]mcpproxy.WorkspaceTarget{{HostRoot: "/host/x", ContainerWS: "not-absolute"}},
		DefaultContainerManagedMCPTools)
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 0 {
		t.Fatalf("non-absolute target must be skipped, got mounts=%v", mounts)
	}
}

func TestMergeManagedContainerMCPMounts_DedupsAbsoluteTargets(t *testing.T) {
	runDir := t.TempDir()
	hostA := t.TempDir()
	mounts, err := mergeManagedContainerMCPMounts(runDir, nil,
		[]mcpproxy.WorkspaceTarget{
			{HostRoot: hostA, ContainerWS: "/workspace/app"},
			{HostRoot: hostA, ContainerWS: "/workspace/app"},
		},
		DefaultContainerManagedMCPTools)
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 {
		t.Fatalf("duplicate absolute targets must dedup to a single overlay, got %d: %v", len(mounts), mounts)
	}
}
