package agentlaunch

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-grid/platform/mcpoverlay"
	"github.com/takezoh/agent-grid/platform/mcpproxy"
	"github.com/takezoh/credproxy/container"
)

// mergeManagedContainerMCPMounts composes managed MCP alias overlays into the
// per-workspace .mcp.json mounts. External proxy overlays already present in
// mounts are consumed as merge bases (their alias entries survive when the
// managed catalog does not force an override); managed aliases are added to
// every absolute deduplicated workspace target.
//
// Empty tool sets are non-mutating: mounts is returned unchanged and no
// overlay is written. This is the same fail-closed contract as the host
// direct path (design integrated.md FR-004 / FR-006).
func mergeManagedContainerMCPMounts(runDir string, mounts []string, targets []mcpproxy.WorkspaceTarget, tools ManagedMCPTools) ([]string, error) {
	if tools.Empty() {
		return mounts, nil
	}
	aliases, err := RenderManagedMCPAliases(tools, ManagedMCPEndpoint{
		Executable: ContainerBinaryPath,
		Socket:     ContainerSockFilePath,
	})
	if err != nil {
		return nil, err
	}

	baseByTarget := map[string]string{}
	out := make([]string, 0, len(mounts)+len(targets))
	for _, mount := range mounts {
		target := mountField(mount, "target")
		source := mountField(mount, "source")
		if strings.HasSuffix(target, "/.mcp.json") && source != "" {
			baseByTarget[target] = source
			continue
		}
		out = append(out, mount)
	}

	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if !filepath.IsAbs(target.ContainerWS) {
			slog.Warn("agentlaunch: skip managed MCP overlay for non-absolute workspace target",
				"containerWS", target.ContainerWS, "hostRoot", target.HostRoot)
			continue
		}
		targetPath := target.ContainerWS + "/.mcp.json"
		if _, dup := seen[targetPath]; dup {
			continue
		}
		seen[targetPath] = struct{}{}
		basePath := filepath.Join(target.HostRoot, ".mcp.json")
		if source := baseByTarget[targetPath]; source != "" {
			basePath = source
		}
		hostOverlay := filepath.Join(runDir, managedMCPJSONFileName(target.ContainerWS))
		if err := mcpoverlay.WriteJSON(hostOverlay, basePath, aliases); err != nil {
			return nil, fmt.Errorf("agentlaunch: write managed MCP overlay: %w", err)
		}
		out = append(out, fmt.Sprintf("type=bind,source=%s,target=%s,readonly", hostOverlay, targetPath))
	}
	return out, nil
}

func managedMCPJSONFileName(containerWS string) string {
	return "managed-mcp-" + container.ProjectRunHash(containerWS) + ".json"
}

func mountField(mount, key string) string {
	prefix := key + "="
	for _, part := range strings.Split(mount, ",") {
		if value, ok := strings.CutPrefix(part, prefix); ok {
			return value
		}
	}
	return ""
}
