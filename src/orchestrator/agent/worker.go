package agent

import (
	"context"
	"log/slog"
)

// Worker implements scheduler.Worker (SPEC §7.2 / §16.5).
// It holds the cancellation function and a done channel for the codex subprocess.
type Worker struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

// Kill stops the underlying codex subprocess by cancelling the worker context.
// It blocks until the subprocess exits.
func (w *Worker) Kill(reason string) error {
	slog.Info("agent: killing worker", "reason", reason)
	w.cancel()
	<-w.done
	return nil
}
