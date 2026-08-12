package main

import (
	"context"
	"fmt"

	"github.com/takezoh/agent-grid/services/platform/agentlaunch"
)

// buildDispatcher constructs the Dispatcher for the orchestrator. Agents run
// directly on the host, so the dispatcher needs no per-project resolution.
// Returns the dispatcher and a cleanup func that stops any background services.
func buildDispatcher(_ context.Context, _ string) (agentlaunch.Dispatcher, func(), error) {
	return agentlaunch.DirectDispatcher{}, func() {}, nil
}

// ensureProject warms up the dispatcher for a project path.
func ensureProject(ctx context.Context, dispatcher agentlaunch.Dispatcher, projectPath string) error {
	if err := dispatcher.EnsureProject(ctx, projectPath); err != nil {
		return fmt.Errorf("orchestrator: ensure project: %w", err)
	}
	return nil
}
