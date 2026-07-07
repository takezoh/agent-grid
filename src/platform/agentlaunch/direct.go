package agentlaunch

import "context"

// DirectDispatcher is the no-op Dispatcher: it passes the plan through unchanged.
// SockPath, when non-empty, is injected as AG_SOCKET so hook subprocesses
// can reach the daemon without relying on baked-in or fallback paths.
type DirectDispatcher struct {
	SockPath string
	SelfBin  string
	DataDir  string
}

func (d DirectDispatcher) Wrap(_ context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error) {
	merged := stripContainerOnlyEnv(plan.Env, plan.ManagedFrameMessaging)
	var cleanup func(context.Context) error
	if d.SockPath != "" {
		merged = cloneAndSet(merged, "AG_SOCKET", d.SockPath)
	}
	if plan.ManagedFrameMessaging {
		var err error
		merged, cleanup, err = PrepareManagedClaudeHome(frameID, d.SelfBin, d.SockPath, d.DataDir, merged)
		if err != nil {
			return WrappedLaunch{}, err
		}
	}
	return WrappedLaunch{
		Command:  plan.Command,
		Argv:     plan.Argv,
		StartDir: plan.StartDir,
		Env:      merged,
		Cleanup:  cleanup,
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
