package scheduler

import (
	"context"
	"errors"
)

// ErrSnapshotTimeout is returned by SnapshotCtx when the state lock cannot be
// acquired before the context deadline expires (SPEC §13.3 RECOMMENDED).
var ErrSnapshotTimeout = errors.New("snapshot timeout")

// ErrOrchestratorUnavailable is returned by SnapshotCtx when the orchestrator
// is not available to serve a snapshot (not yet started or shutting down)
// (SPEC §13.3 RECOMMENDED).
var ErrOrchestratorUnavailable = errors.New("orchestrator unavailable")

// SnapshotCtx returns a read-only copy of the current state, or an error if
// the context expires before the state lock can be acquired (SPEC §13.3).
//
// The goroutine that acquires the lock always completes and releases it even
// when the context is cancelled first; the buffered channel (cap 1) ensures
// the goroutine never blocks on the send, so there is no goroutine leak.
//
// Error mapping:
//   - context.DeadlineExceeded → ErrSnapshotTimeout
//   - context.Canceled         → ErrOrchestratorUnavailable
func (s *State) SnapshotCtx(ctx context.Context) (StateSnapshot, error) {
	ch := make(chan StateSnapshot, 1)
	go func() {
		ch <- s.Snapshot()
	}()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return StateSnapshot{}, ErrSnapshotTimeout
		}
		return StateSnapshot{}, ErrOrchestratorUnavailable
	case snap := <-ch:
		return snap, nil
	}
}
