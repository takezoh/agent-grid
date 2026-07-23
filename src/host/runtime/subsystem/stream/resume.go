package stream

import (
	"errors"
	"fmt"
	"strings"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	"github.com/takezoh/agent-grid/platform/pathmap"
)

// resolveDialSock returns the host-side path the daemon dials for an app-server
// that binds listenSock. In container mode listenSock is container-absolute and
// the launch's bind mounts expose it at a host path; in host mode there are no
// mounts, so the dial path equals the listen path.
func resolveDialSock(listenSock string, wrapped agentlaunch.WrappedLaunch) string {
	if host, ok := wrapped.HostPath(listenSock); ok {
		return host
	}
	return listenSock
}

func toPathmapMounts(ms []agentlaunch.Mount) pathmap.Mounts {
	if len(ms) == 0 {
		return nil
	}
	out := make(pathmap.Mounts, len(ms))
	for i, m := range ms {
		out[i] = pathmap.Mount{Host: m.Host, Container: m.Container}
	}
	return out
}

type normalizedResumeTarget struct {
	rpc state.ResumeTarget
}

func normalizeResumeTarget(target state.ResumeTarget, mounts pathmap.Mounts) (normalizedResumeTarget, error) {
	target.ThreadID = strings.TrimSpace(target.ThreadID)
	target.RolloutPath = strings.TrimSpace(target.RolloutPath)
	if target.ThreadID == "" && target.RolloutPath == "" {
		return normalizedResumeTarget{}, nil
	}
	if target.RolloutPath == "" {
		return normalizedResumeTarget{rpc: state.ResumeTarget{ThreadID: target.ThreadID}}, nil
	}
	cliPath, _, err := translateRolloutPath(target.RolloutPath, mounts)
	if err != nil {
		return normalizedResumeTarget{}, err
	}
	return normalizedResumeTarget{
		rpc: state.ResumeTarget{ThreadID: target.ThreadID, RolloutPath: cliPath},
	}, nil
}

func translateRolloutPath(path string, mounts pathmap.Mounts) (cliPath, hostPath string, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", errors.New("stream backend: codex resume requires rollout path")
	}
	if len(mounts) == 0 {
		return path, path, nil
	}
	if container, ok := mounts.ToContainer(path); ok {
		return container, path, nil
	}
	if host, ok := mounts.ToHost(path); ok {
		return path, host, nil
	}
	return "", "", fmt.Errorf("stream backend: rollout path %q is not reachable from current sandbox mounts", path)
}
