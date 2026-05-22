package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSnapshotCtx_Success verifies that SnapshotCtx returns the state normally
// when the lock is available and the context has not expired.
func TestSnapshotCtx_Success(t *testing.T) {
	s := NewState()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snap, err := s.SnapshotCtx(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Running == nil {
		t.Error("Running map should be non-nil")
	}
}

// TestSnapshotCtx_Timeout verifies that SnapshotCtx returns ErrSnapshotTimeout
// when the context deadline expires while the state lock is held by another goroutine.
func TestSnapshotCtx_Timeout(t *testing.T) {
	s := NewState()

	// Hold the lock to block SnapshotCtx.
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := s.SnapshotCtx(ctx)
	if !errors.Is(err, ErrSnapshotTimeout) {
		t.Errorf("want ErrSnapshotTimeout, got %v", err)
	}
}

// TestSnapshotCtx_Cancelled verifies that SnapshotCtx returns ErrOrchestratorUnavailable
// when the context is cancelled (not timed out) while the state lock is held.
func TestSnapshotCtx_Cancelled(t *testing.T) {
	s := NewState()

	// Hold the lock to block SnapshotCtx.
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately via a goroutine so the select in SnapshotCtx fires.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := s.SnapshotCtx(ctx)
		if !errors.Is(err, ErrOrchestratorUnavailable) {
			t.Errorf("want ErrOrchestratorUnavailable, got %v", err)
		}
	}()

	cancel()
	<-done
}

// TestScheduler_SnapshotCtx_Unavailable verifies that Scheduler.SnapshotCtx
// returns ErrOrchestratorUnavailable when the scheduler has not been started
// (available flag is false).
func TestScheduler_SnapshotCtx_Unavailable(t *testing.T) {
	s := New("", schedCfg(), minDeps(nil, nil, newFakeClock(time.Now())))
	// available is false by default (Run has not been called).

	ctx := context.Background()
	_, err := s.SnapshotCtx(ctx)
	if !errors.Is(err, ErrOrchestratorUnavailable) {
		t.Errorf("want ErrOrchestratorUnavailable before Run, got %v", err)
	}
}

// TestScheduler_SnapshotCtx_AvailableWhileRunning verifies that Scheduler.SnapshotCtx
// succeeds while the scheduler is running (available flag is true).
func TestScheduler_SnapshotCtx_AvailableWhileRunning(t *testing.T) {
	s := New("", schedCfg(), minDeps(nil, nil, newFakeClock(time.Now())))
	s.available.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snap, err := s.SnapshotCtx(ctx)
	if err != nil {
		t.Fatalf("want nil error while available, got %v", err)
	}
	if snap.Running == nil {
		t.Error("Running map should be non-nil")
	}
}
