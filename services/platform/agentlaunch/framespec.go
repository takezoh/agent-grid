package agentlaunch

import (
	"github.com/takezoh/agent-grid/services/platform/framelaunch"
)

// EncodeFrameSpec builds an AG_FRAME_SPEC JSON payload from a LaunchPlan.
// LoginShell is intentionally left empty by most callers — the framelaunch.Run
// consumer resolves it from /etc/passwd at runtime (see OQ-1 resolution).
func EncodeFrameSpec(plan LaunchPlan) (string, error) {
	spec := framelaunch.FrameSpec{
		PreExec:     plan.PreExec,
		LoginShell:  plan.LoginShell, // usually empty; caller override only
		PreCommands: plan.PreCommands,
		MainCommand: plan.Argv,
	}
	if plan.PreCommandTimeout > 0 {
		spec.PreCommandTimeout = plan.PreCommandTimeout.String()
	}
	return framelaunch.Encode(spec)
}
