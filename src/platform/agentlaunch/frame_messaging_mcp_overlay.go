package agentlaunch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-grid/platform/mcpoverlay"
	"github.com/takezoh/agent-grid/platform/mcpproxy"
	"github.com/takezoh/credproxy/container"
)

const managedAgentFramesAlias = "agent_frames"

func mergeManagedAgentFramesMounts(runDir string, mounts []string, targets []mcpproxy.WorkspaceTarget) ([]string, error) {
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
	for _, target := range targets {
		if !filepath.IsAbs(target.ContainerWS) {
			slog.Warn("agentlaunch: skip managed agent_frames overlay for non-absolute workspace target",
				"containerWS", target.ContainerWS, "hostRoot", target.HostRoot)
			continue
		}
		targetPath := target.ContainerWS + "/.mcp.json"
		basePath := filepath.Join(target.HostRoot, ".mcp.json")
		if source := baseByTarget[targetPath]; source != "" {
			basePath = source
		}
		hostOverlay := filepath.Join(runDir, managedMCPJSONFileName(target.ContainerWS))
		if err := writeManagedAgentFramesMCPJSON(hostOverlay, basePath); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("type=bind,source=%s,target=%s,readonly", hostOverlay, targetPath))
	}
	return out, nil
}

func writeManagedAgentFramesMCPJSON(path, basePath string) error {
	entry, err := json.Marshal(map[string]any{
		"type":    "stdio",
		"command": ContainerBinaryPath,
		"args":    []string{"agent-frames-mcp", "--sock", ContainerSockFilePath},
	})
	if err != nil {
		return err
	}
	return mcpoverlay.WriteJSON(path, basePath, map[string]mcpoverlay.AliasEntry{
		managedAgentFramesAlias: {Value: entry},
	})
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
