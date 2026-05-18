package runtime

import (
	"fmt"

	"github.com/takezoh/agent-roost/state"
)

type SubsystemRegistry struct{}

func newSubsystemRegistry() *SubsystemRegistry {
	return &SubsystemRegistry{}
}

func (r *SubsystemRegistry) Inject(kind state.LaunchSubsystem, plan state.LaunchPlan, stdin []byte) (state.LaunchPlan, map[string]string, error) {
	sub, err := resolveLaunchSubsystem(kind)
	if err != nil {
		return plan, nil, err
	}
	p, env := sub.RewritePlan(plan, stdin)
	return p, env, nil
}

type launchSubsystem interface {
	// RewritePlan rewrites plan.Command into the canonical helper invocation
	// sentinel and returns the extra env vars to be merged before WrapLaunch.
	// Launchers resolve the sentinel to a physical binary path.
	RewritePlan(plan state.LaunchPlan, stdin []byte) (state.LaunchPlan, map[string]string)
}

type cliSubsystem struct{}

func (cliSubsystem) RewritePlan(plan state.LaunchPlan, _ []byte) (state.LaunchPlan, map[string]string) {
	return plan, nil
}

type streamSubsystem struct{}

func (streamSubsystem) RewritePlan(plan state.LaunchPlan, _ []byte) (state.LaunchPlan, map[string]string) {
	return plan, nil
}

func resolveLaunchSubsystem(kind state.LaunchSubsystem) (launchSubsystem, error) {
	switch kind {
	case "", state.LaunchSubsystemCLI:
		return cliSubsystem{}, nil
	case state.LaunchSubsystemStream:
		return streamSubsystem{}, nil
	default:
		return nil, fmt.Errorf("runtime: unknown launch subsystem %q", kind)
	}
}

func cloneEnvMap(env map[string]string, extra int) map[string]string {
	if env == nil {
		return make(map[string]string, extra)
	}
	out := make(map[string]string, len(env)+extra)
	for k, v := range env {
		out[k] = v
	}
	return out
}

func mergeEnvMaps(base, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return cloneEnvMap(base, 0)
	}
	out := cloneEnvMap(base, len(extra))
	for k, v := range extra {
		out[k] = v
	}
	return out
}
