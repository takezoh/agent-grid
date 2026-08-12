package agent

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// Worker implements scheduler.Worker (SPEC §7.2 / §16.5).
// It holds the cancellation function and a done channel for the codex subprocess.
type Worker struct {
	cancel          context.CancelFunc
	done            <-chan struct{}
	issueID         string
	issueIdentifier string
	graceful        atomic.Bool // true when kill is orchestrator-initiated due to agent handoff/completion
}

// Kill stops the underlying codex subprocess by cancelling the worker context.
// It blocks until the subprocess exits.
// Reasons "terminal" and "non-active" are agent-initiated state transitions (handoff/Done);
// the graceful flag suppresses turn_failed for those cases.
func (w *Worker) Kill(reason string) error {
	slog.Info("agent: killing worker",
		"reason", reason,
		"issue_id", w.issueID,
		"issue_identifier", w.issueIdentifier,
	)
	if reason == "terminal" || reason == "non-active" {
		w.graceful.Store(true)
	}
	w.cancel()
	<-w.done
	return nil
}

// WasKilledGracefully reports whether Kill was called with a reason that represents
// an agent-initiated state transition (handoff to Human Review or terminal completion).
// Used by awaitTurn to suppress turn_failed for orchestrator-managed stops.
func (w *Worker) WasKilledGracefully() bool {
	return w.graceful.Load()
}
