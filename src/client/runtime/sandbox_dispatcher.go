package runtime

import (
	"context"
	"fmt"

	"github.com/takezoh/agent-roost/client/config"
	"github.com/takezoh/agent-roost/platform/pathmap"
	"github.com/takezoh/agent-roost/client/state"
)

// SandboxDispatcher implements AgentLauncher by selecting the correct backend
// (direct or devcontainer) based on the effective sandbox mode for each project.
// The mode is resolved per call via a SandboxResolver so project-scope
// overrides are applied without restarting the daemon.
type SandboxDispatcher struct {
	Resolver     *config.SandboxResolver
	Direct       AgentLauncher
	Devcontainer *DevcontainerLauncher // nil when devcontainer backend is not configured
}

// WrapLaunch resolves the effective sandbox mode for plan.Project and
// delegates to the appropriate backend launcher.
func (d *SandboxDispatcher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	if plan.Sandbox == state.SandboxOverrideHost {
		return d.Direct.WrapLaunch(frameID, plan, env)
	}
	mode := d.Resolver.Resolve(plan.Project).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: devcontainer mode for %q but devcontainer backend unavailable", plan.Project)
		}
		return d.Devcontainer.WrapLaunch(frameID, plan, env)
	case "", "direct":
		return d.Direct.WrapLaunch(frameID, plan, env)
	default:
		return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, plan.Project)
	}
}

// EnsureProject resolves the effective sandbox mode for projectPath and delegates
// to the appropriate backend to warm up the container without allocating a frame.
func (d *SandboxDispatcher) EnsureProject(ctx context.Context, projectPath string) error {
	mode := d.Resolver.Resolve(projectPath).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return nil
		}
		return d.Devcontainer.EnsureProject(ctx, projectPath)
	case "", "direct":
		return d.Direct.EnsureProject(ctx, projectPath)
	default:
		return fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, projectPath)
	}
}

// IsContainer reports whether projectPath will be run inside a container.
func (d *SandboxDispatcher) IsContainer(projectPath string) bool {
	if d.Devcontainer == nil {
		return false
	}
	mode := d.Resolver.Resolve(projectPath).Mode
	return mode == "devcontainer"
}

// BeginColdStart / EndColdStart forward the coordinator's cold-start window
// to every backend that supports it (currently only the devcontainer
// launcher; Direct mode has no persistent sandbox to discard).
func (d *SandboxDispatcher) BeginColdStart() {
	if d.Devcontainer != nil {
		d.Devcontainer.BeginColdStart()
	}
	if cs, ok := d.Direct.(ColdStartAware); ok {
		cs.BeginColdStart()
	}
}

func (d *SandboxDispatcher) EndColdStart() {
	if d.Devcontainer != nil {
		d.Devcontainer.EndColdStart()
	}
	if cs, ok := d.Direct.(ColdStartAware); ok {
		cs.EndColdStart()
	}
}

// devcontainerLauncherFor extracts the *DevcontainerLauncher from l, handling
// both a bare *DevcontainerLauncher and a *SandboxDispatcher wrapper.
// Returns nil if l has no devcontainer backend.
func devcontainerLauncherFor(l AgentLauncher) *DevcontainerLauncher {
	switch v := l.(type) {
	case *DevcontainerLauncher:
		return v
	case *SandboxDispatcher:
		return v.Devcontainer
	}
	return nil
}

// AdoptFrame resolves the effective sandbox mode for projectPath and delegates
// to the appropriate backend to reclaim the pre-running sandbox frame.
func (d *SandboxDispatcher) AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, pathmap.Mounts, error) {
	mode := d.Resolver.Resolve(projectPath).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return nil, nil, nil
		}
		return d.Devcontainer.AdoptFrame(ctx, frameID, projectPath)
	case "", "direct":
		return d.Direct.AdoptFrame(ctx, frameID, projectPath)
	default:
		return nil, nil, fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, projectPath)
	}
}
