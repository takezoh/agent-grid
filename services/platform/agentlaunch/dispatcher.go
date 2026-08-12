package agentlaunch

import "context"

// Dispatcher resolves a LaunchPlan into the concrete launch spec the caller's
// spawn layer executes. Implementations must be safe for concurrent use.
type Dispatcher interface {
	// Wrap resolves plan into the launch spec for frameID.
	Wrap(ctx context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error)

	// EnsureProject prepares the environment for a project without allocating
	// a frame.
	EnsureProject(ctx context.Context, projectPath string) error
}
