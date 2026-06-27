package runtime

import (
	"log/slog"
	"sync"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

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
