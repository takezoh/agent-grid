package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-roost/runtime/subsystem/stream"
)

// resolveStreamSockPaths returns the host-side and container-side sock paths
// for the given project. The container path equals the host path when the
// project runs directly on the host.
func (r *Runtime) resolveStreamSockPaths(project string) (string, string, error) {
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	l := launcher(r.cfg)
	runDirKey := project
	if dl := devcontainerLauncherFor(l); dl != nil && l.IsContainer(project) {
		runDirKey = dl.RunDirKey(project)
	}
	runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), runDirKey)
	if err != nil {
		return "", "", fmt.Errorf("stream backend: run dir: %w", err)
	}
	hostSock := filepath.Join(runDir, cstream.SockName)
	containerSock := hostSock
	if l.IsContainer(project) {
		containerSock = ContainerRunDir + "/" + cstream.SockName
	}
	return hostSock, containerSock, nil
}

// ContainerExecConfig implements stream.RuntimeHook: returns docker exec
// parameters for the project's devcontainer, or nil for host projects.
func (r *Runtime) ContainerExecConfig(ctx context.Context, project string) (*cstream.ContainerExecConfig, error) {
	if !launcher(r.cfg).IsContainer(project) {
		return nil, nil
	}
	dl := devcontainerLauncherFor(launcher(r.cfg))
	if dl == nil {
		return nil, fmt.Errorf("runtime: unsupported container launcher for stream backend")
	}
	inst, err := dl.mgr.EnsureInstance(ctx, project, "", dl.StartOptionsFor(project))
	if err != nil {
		return nil, err
	}
	cs := inst.Internal
	return &cstream.ContainerExecConfig{
		ContainerID: cs.ContainerID(),
		User:        cs.EffectiveUser(),
		WorkDir:     cs.WorkspaceTarget(),
		PreExec:     cs.PreExec(),
	}, nil
}
