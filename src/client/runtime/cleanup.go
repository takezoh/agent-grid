package runtime

import (
	"context"
	"log/slog"
	"sync"

	rsubsystem "github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/pathmap"
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
	fns := r.sandboxCleanups
	r.sandboxCleanups = map[state.FrameID]func() error{}

	go func() {
		defer cancel()
		completed := make(chan struct{}, len(subs)+len(fns))
		for _, sub := range subs {
			go func(sub rsubsystem.Subsystem) {
				sub.Stop(ctx, rsubsystem.StopCauseRuntimeShutdown)
				completed <- struct{}{}
			}(sub)
		}
		for frameID, fn := range fns {
			go func(frameID state.FrameID, fn func() error) {
				if err := fn(); err != nil {
					slog.Warn("runtime: sandbox cleanup (shutdown) failed", "frame", frameID, "err", err)
				}
				completed <- struct{}{}
			}(frameID, fn)
		}
		remaining := len(subs) + len(fns)
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

// storeFrameCleanup registers a sandbox cleanup callback for a frame.
// No-op when fn is nil. Must be called from the event loop or bootstrap
// (pre-Run) only — sandboxCleanups is a plain loop-owned map.
func (r *Runtime) storeFrameCleanup(frameID state.FrameID, fn func() error) {
	if fn == nil {
		return
	}
	r.sandboxCleanups[frameID] = fn
}

// registerContainerFrame atomically registers the container token and mounts,
// starts the endpoint if needed, and persists the warm-frame state. The atomic
// RegisterWithMounts closes the window where a container request could see the
// token before its mounts. Warm Save runs synchronously on the event loop
// (issues/029, F4) so it cannot race executeKillSessionWindow's synchronous
// Delete on the same frame's warm file — the prior async go-Save could win and
// leave a stale warm file behind a kill. The disk cost is a small JSON write
// to <dataDir>/warm/, sub-ms on local storage. Must be called from the event
// loop or bootstrap (pre-Run) only.
func (r *Runtime) registerContainerFrame(frameID state.FrameID, project, sockDir, token string, mounts pathmap.Mounts) {
	r.frameReg.RegisterWithMounts(frameID, token, mounts)
	r.startContainerEndpointIfNeeded(project, ContainerSockPath(sockDir))
	if r.warmFrames == nil {
		return
	}
	wf := WarmFrameState{FrameID: string(frameID), ContainerToken: token}
	if err := r.warmFrames.Save(wf); err != nil {
		slog.Warn("runtime: warm frame save failed", "frame", frameID, "err", err)
	}
}

func (r *Runtime) registerFrameToken(frameID state.FrameID, token string) {
	r.frameReg.Register(frameID, token)
}

// invokeFrameCleanup removes the frame's container registration, retrieves the
// registered sandbox cleanup, deletes it from the map, and runs it in a
// goroutine so the event loop is not blocked. Must be called from the event
// loop only.
func (r *Runtime) invokeFrameCleanup(frameID state.FrameID) {
	r.frameReg.Delete(frameID)
	fn := r.sandboxCleanups[frameID]
	delete(r.sandboxCleanups, frameID)
	if fn == nil {
		return
	}
	go func() {
		if err := fn(); err != nil {
			slog.Warn("runtime: sandbox cleanup failed", "frame", frameID, "err", err)
		}
	}()
}

// drainFrameCleanups invokes all pending sandbox cleanups concurrently and
// waits for them to finish. Called at daemon shutdown before the launcher
// itself is shut down. Must be called from the event loop only.
func (r *Runtime) drainFrameCleanups() {
	fns := r.sandboxCleanups
	r.sandboxCleanups = map[state.FrameID]func() error{}
	var wg sync.WaitGroup
	for frameID, fn := range fns {
		wg.Add(1)
		go func(frameID state.FrameID, fn func() error) {
			defer wg.Done()
			if err := fn(); err != nil {
				slog.Warn("runtime: sandbox cleanup (drain) failed", "frame", frameID, "err", err)
			}
		}(frameID, fn)
	}
	wg.Wait()
}
