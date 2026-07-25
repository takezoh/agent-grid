package agentlaunch

import (
	"context"
	"fmt"

	"github.com/takezoh/agent-grid/platform/appid"
	"github.com/takezoh/agent-grid/platform/framelaunch"
)

// DirectDispatcher is the host-side Dispatcher. Every launch goes through
// `<SelfBin> frame-exec` with AG_FRAME_SPEC (same sequencing as the container
// bridge path). SockPath, when non-empty, is injected as AG_SOCKET so hook
// subprocesses can reach the daemon without relying on baked-in paths.
type DirectDispatcher struct {
	SockPath string
	SelfBin  string
}

func (d DirectDispatcher) Wrap(_ context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error) {
	merged := stripContainerOnlyEnv(plan.Env, plan.ManagedMCPTools.RequiresAuthority())
	if d.SockPath != "" {
		merged = cloneAndSet(merged, "AG_SOCKET", d.SockPath)
	}

	plan, err := NormalizePlanForFrameExec(plan)
	if err != nil {
		return WrappedLaunch{}, err
	}
	if !plan.ManagedMCPTools.Empty() {
		endpoint := ManagedMCPEndpoint{Executable: d.SelfBin, Socket: d.SockPath}
		plan.Argv, err = AppendManagedMCPClaudeConfig(plan.Argv, plan.ManagedMCPTools, endpoint)
		if err != nil {
			return WrappedLaunch{}, err
		}
	}
	specJSON, err := EncodeFrameSpec(plan)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("agentlaunch: encode FrameSpec: %w", err)
	}
	merged = cloneAndSet(merged, framelaunch.EnvVar, specJSON)
	selfBin := d.SelfBin
	if selfBin == "" {
		selfBin = appid.ClientBin
	}
	cmd := JoinArgs([]string{selfBin, "frame-exec"})
	return WrappedLaunch{
		Command:  cmd,
		Argv:     []string{selfBin, "frame-exec"},
		StartDir: plan.StartDir,
		Env:      merged,
	}, nil
}

func (DirectDispatcher) AdoptFrame(_ context.Context, _, _ string) (func(context.Context) error, []Mount, error) {
	return nil, nil, nil
}

func (DirectDispatcher) EnsureProject(_ context.Context, _ string) error { return nil }

func (DirectDispatcher) IsContainer(_ string) bool { return false }

func (DirectDispatcher) BeginColdStart() {}
func (DirectDispatcher) EndColdStart()   {}

// stripContainerOnlyEnv returns a copy of env with AG_SOCKET_TOKEN forced
// empty unless the launch explicitly needs host-side frame-messaging access.
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

func cloneEnvMap(src map[string]string, extra int) map[string]string {
	out := make(map[string]string, len(src)+extra)
	for k, v := range src {
		out[k] = v
	}
	return out
}
