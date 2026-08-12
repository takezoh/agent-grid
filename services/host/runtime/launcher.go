package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/agentlaunch"
)

// WrappedLaunch is the resolved launch specification the launcher produced.
// The runtime passes Command/StartDir/Env directly to FrameBackend.SpawnFrame.
type WrappedLaunch struct {
	Command  string
	StartDir string
	Env      map[string]string
}

// AgentLauncher resolves a state.LaunchPlan into the concrete command the frame
// backend spawns. The runtime calls WrapLaunch once per spawn; DirectLauncher
// is used when no Launcher is configured.
type AgentLauncher interface {
	WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error)
}

// DirectLauncher is the no-op implementation: it passes the plan through
// unchanged so behaviour is identical to the pre-launcher code path.
// SockPath, when non-empty, is injected as AG_SOCKET so hook subprocesses
// can reach the daemon without relying on baked-in or fallback paths.
type DirectLauncher struct {
	SockPath string
	SelfBin  string
}

func (d DirectLauncher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	merged := stripContainerOnlyEnv(env, plan.ManagedMCPTools.RequiresAuthority())
	if d.SockPath != "" {
		merged = cloneAndSet(merged, "AG_SOCKET", d.SockPath)
	}

	// Every launch goes through `<SelfBin> frame-exec` (adr-20260711-0082).
	// Command strings from drivers are tokenized into Argv at this boundary.
	normalized, err := agentlaunch.NormalizePlanForFrameExec(agentlaunch.LaunchPlan{
		Command:           plan.Command,
		Argv:              plan.Argv,
		PreCommands:       plan.PreCommands,
		PreCommandTimeout: plan.PreCommandTimeout,
		ManagedMCPTools:   plan.ManagedMCPTools,
	})
	if err != nil {
		return WrappedLaunch{}, err
	}
	if !plan.ManagedMCPTools.Empty() {
		endpoint := agentlaunch.ManagedMCPEndpoint{Executable: d.SelfBin, Socket: d.SockPath}
		normalized.Argv, err = agentlaunch.AppendManagedMCPClaudeConfig(normalized.Argv, plan.ManagedMCPTools, endpoint)
		if err != nil {
			return WrappedLaunch{}, err
		}
	}
	specJSON, err := agentlaunch.EncodeFrameSpec(normalized)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("runtime: encode FrameSpec: %w", err)
	}
	merged = cloneAndSet(merged, "AG_FRAME_SPEC", specJSON)
	selfBin := d.SelfBin
	if selfBin == "" {
		selfBin = "server"
	}
	return WrappedLaunch{
		Command:  agentlaunch.JoinArgs([]string{selfBin, "frame-exec"}),
		StartDir: plan.StartDir,
		Env:      merged,
	}, nil
}

// stripContainerOnlyEnv returns a copy of env with AG_SOCKET_TOKEN forced
// empty unless the launch explicitly needs host-side frame-messaging access.
// This keeps host processes from inheriting an ambient credential while still
// allowing managed host Claude launches to use agent_frames.
func stripContainerOnlyEnv(env map[string]string, keepFrameMessagingToken bool) map[string]string {
	out := cloneEnvMap(env, 1)
	if !keepFrameMessagingToken {
		out["AG_SOCKET_TOKEN"] = ""
	}
	return out
}

func cloneAndSet(env map[string]string, key, value string) map[string]string {
	out := cloneEnvMap(env, 1)
	out[key] = value
	return out
}

// dispatcherAdapter translates between AgentLauncher (client state types) and
// agentlaunch.Dispatcher (pure platform types).
type dispatcherAdapter struct {
	d agentlaunch.Dispatcher
}

func (a dispatcherAdapter) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	pp := agentlaunch.LaunchPlan{
		Command:           plan.Command,
		Argv:              plan.Argv,
		PreCommands:       plan.PreCommands,
		PreCommandTimeout: plan.PreCommandTimeout,
		Env:               env,
		StartDir:          plan.StartDir,
		Project:           plan.Project,
		ManagedMCPTools:   plan.ManagedMCPTools,
	}
	w, err := a.d.Wrap(context.Background(), string(frameID), pp)
	if err != nil {
		return WrappedLaunch{}, err
	}
	return WrappedLaunch{
		Command:  w.Command,
		StartDir: w.StartDir,
		Env:      w.Env,
	}, nil
}

// NewDispatcherAdapter wraps an agentlaunch.Dispatcher for use as AgentLauncher.
func NewDispatcherAdapter(d agentlaunch.Dispatcher) AgentLauncher {
	return dispatcherAdapter{d: d}
}

// launcher returns cfg.Launcher if set, otherwise a zero-cost DirectLauncher.
func launcher(cfg Config) AgentLauncher {
	if cfg.Launcher != nil {
		return cfg.Launcher
	}
	return DirectLauncher{}
}

// wrapLaunchResult holds the output of wrapLaunchForSpawn.
type wrapLaunchResult struct {
	wrapped WrappedLaunch
	// token is non-empty only when the frame exposes frame-messaging
	// authority. The token string is generated here so it can be baked into
	// the spawn env. Registration (token↔frame) happens on the event loop via
	// internalSpawnComplete, so no runtime state is mutated from the spawn
	// goroutine.
	token string
}

// wrapLaunchForSpawn calls WrapLaunch and generates a bearer token when the
// launched process needs frame-messaging authority — a host Claude launch with
// managed agent_frames exposure, whose token terminates at the host IPC
// socket. It has no side effects on runtime state — token registration happens
// after spawn completes on the event loop.
func wrapLaunchForSpawn(l AgentLauncher, frameID state.FrameID, project string, plan state.LaunchPlan, baseEnv map[string]string) (wrapLaunchResult, error) {
	_ = project
	if !plan.ManagedMCPTools.RequiresAuthority() {
		wrapped, err := l.WrapLaunch(frameID, plan, baseEnv)
		return wrapLaunchResult{wrapped: wrapped}, err
	}

	token, err := generateToken()
	if err != nil {
		return wrapLaunchResult{}, fmt.Errorf("token generate: %w", err)
	}
	env := cloneAndSet(baseEnv, "AG_SOCKET_TOKEN", token)

	wrapped, err := l.WrapLaunch(frameID, plan, env)
	if err != nil {
		return wrapLaunchResult{}, fmt.Errorf("launcher wrap: %w", err)
	}
	return wrapLaunchResult{wrapped: wrapped, token: token}, nil
}

// generateToken returns a random 32-byte hex-encoded bearer token.
// Pure computation; safe to call from any goroutine.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
