package runtime

import (
	"context"

	rsubsystem "github.com/takezoh/agent-grid/services/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/services/host/state"
)

func (r *Runtime) enqueueLifecycle(ev state.Event) {
	select {
	case r.eventCh <- ev:
	case <-r.done:
	}
}

// startShutdownCleanup starts independent workers and waits only until the
// first transaction deadline. Non-cooperative cleanup is deliberately left
// behind for process exit; its late completion cannot enqueue another result.
func (r *Runtime) startShutdownCleanup(transactionID uint64) {
	deadline := r.shutdownDeadline()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	subs := make([]rsubsystem.Subsystem, 0, len(r.subsystems))
	for _, sub := range r.subsystems {
		subs = append(subs, sub)
	}
	go func() {
		defer cancel()
		completed := make(chan struct{}, len(subs))
		for _, sub := range subs {
			go func(sub rsubsystem.Subsystem) {
				sub.Stop(ctx, rsubsystem.StopCauseRuntimeShutdown)
				completed <- struct{}{}
			}(sub)
		}
		remaining := len(subs)
		for remaining > 0 {
			select {
			case <-completed:
				remaining--
			case <-ctx.Done():
				r.enqueueLifecycle(state.EvShutdownCleanupFinished{TransactionID: transactionID, Outcome: state.ShutdownCleanupDeadlineExceeded})
				return
			}
		}
		r.enqueueLifecycle(state.EvShutdownCleanupFinished{TransactionID: transactionID, Outcome: state.ShutdownCleanupCompleted})
	}()
}

func (r *Runtime) registerFrameToken(frameID state.FrameID, token string) {
	r.frameReg.Register(frameID, token)
}

// releaseFrameRegistration drops the frame's entry from the frame registry.
// Must be called from the event loop only.
func (r *Runtime) releaseFrameRegistration(frameID state.FrameID) {
	r.frameReg.Delete(frameID)
}
