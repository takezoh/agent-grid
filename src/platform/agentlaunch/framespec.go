package agentlaunch

import (
	"github.com/takezoh/agent-grid/platform/framelaunch"
	"github.com/takezoh/agent-grid/platform/sandbox"
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

// EncodeFrameSpecFromLaunchSpec is the sandbox.LaunchSpec-oriented variant used
// by callers that already hold a LaunchSpec. Same wire format as EncodeFrameSpec.
// Note: sandbox/devcontainer must not import agentlaunch (import cycle with
// DevcontainerLauncher); it builds framelaunch.FrameSpec directly.
func EncodeFrameSpecFromLaunchSpec(spec sandbox.LaunchSpec) (string, error) {
	fs := framelaunch.FrameSpec{
		PreExec:     spec.PreExec,
		LoginShell:  spec.LoginShell,
		PreCommands: spec.PreCommands,
		MainCommand: spec.Argv,
	}
	if spec.PreCommandTimeout > 0 {
		fs.PreCommandTimeout = spec.PreCommandTimeout.String()
	}
	return framelaunch.Encode(fs)
}
