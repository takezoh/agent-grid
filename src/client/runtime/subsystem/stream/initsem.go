package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

// errInitAdoptExpired is surfaced to Runtime via failFrame when reapExpiredSlots
// discards a slot whose CLI never emitted thread/started within initAdoptDeadline.
// Without a failure event the frame's driver would sit at Idle forever, which is
// the same class of stuck-badge bug ADR-0081 addresses on the routing side.
var errInitAdoptExpired = errors.New("stream backend: CLI did not emit thread/started before adopt deadline")

// pendingSlot is the payload of Backend.initState.slot. deadline lets the
// reaper reclaim slots whose owning frame never received a thread/started.
type pendingSlot struct {
	frameID  state.FrameID
	deadline time.Time
}

// initState is the mutex-guarded single-slot reservation used to serialize
// fresh (non-resume) frame init across a Backend. It replaces the earlier
// capacity-1 chan pendingSlot, whose drain-check-put-back non-atomicity was
// the root cause of a chain of races: reaper drops legitimate thread/started,
// adopt cross-talks with a concurrent reaper, releaseOwnSlot deadlocks or
// silently displaces the wrong frame. With mutex-guarded state every op
// (acquire / adopt / release / reap) transitions the field atomically under
// initMu; blocked acquires wait on a per-generation `free` channel that gets
// closed at the moment of transition-to-empty, so the wait wakeup is race-free.
type initState struct {
	mu   sync.Mutex
	slot *pendingSlot
	// free is closed when slot transitions from non-nil to nil, waking every
	// blocked acquirer. On each transition the field is replaced with a
	// fresh channel for the next generation, so no acquirer can miss a wake.
	free chan struct{}
}

func newInitState() *initState {
	return &initState{free: make(chan struct{})}
}

// clearLocked drains the current slot and broadcasts a wakeup to any acquirers.
// Caller must hold s.mu.
func (s *initState) clearLocked() {
	s.slot = nil
	close(s.free)
	s.free = make(chan struct{})
}

// acquire blocks until the slot is free, then reserves it for frameID.
// Times out after initAcquireTimeout or ctx cancellation. The wait wakeup
// path is race-free: even if a release fires between our unlock and select,
// the closed channel signals immediately on select entry.
func (s *initState) acquire(ctx context.Context, frameID state.FrameID) error {
	acquireCtx, cancel := context.WithTimeout(ctx, initAcquireTimeout)
	defer cancel()
	for {
		s.mu.Lock()
		if s.slot == nil {
			s.slot = &pendingSlot{
				frameID:  frameID,
				deadline: time.Now().Add(initAdoptDeadline),
			}
			s.mu.Unlock()
			return nil
		}
		waitCh := s.free
		s.mu.Unlock()
		select {
		case <-waitCh:
		case <-acquireCtx.Done():
			return fmt.Errorf("stream backend: previous frame init still pending after %s", initAcquireTimeout)
		}
	}
}

// takeIfOwned clears the slot if it belongs to frameID and returns true.
// Used by ReleaseFrame; a bound frame release must not touch a slot owned
// by a different pending frame.
func (s *initState) takeIfOwned(frameID state.FrameID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.slot == nil || s.slot.frameID != frameID {
		return false
	}
	s.clearLocked()
	return true
}

// takeAny clears the slot (any owner) and returns it, or nil if empty.
// Used by adoptPendingFrame to consume the reservation on incoming
// thread/started.
func (s *initState) takeAny() *pendingSlot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.slot == nil {
		return nil
	}
	slot := *s.slot
	s.clearLocked()
	return &slot
}

// takeIfExpired clears the slot iff its deadline has passed. Returns the
// expired slot for cleanup, or nil if the slot is empty or still valid.
// Non-blocking on the caller side and atomic under s.mu — no drain-then-put-back
// window that a concurrent acquire/adopt could race with.
func (s *initState) takeIfExpired(now time.Time) *pendingSlot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.slot == nil || now.Before(s.slot.deadline) {
		return nil
	}
	slot := *s.slot
	s.clearLocked()
	return &slot
}

// acquirePendingSlot is BindFrame's public entry point. See initState.acquire.
func (b *Backend) acquirePendingSlot(ctx context.Context, frameID state.FrameID) error {
	return b.initState.acquire(ctx, frameID)
}

// releaseOwnSlot clears the reservation if this frame is the current owner.
// Callers gate on a pending-check under b.mu, so this call is a no-op for
// bound frames — no chance of touching a foreign slot.
func (b *Backend) releaseOwnSlot(frameID state.FrameID) {
	b.initState.takeIfOwned(frameID)
}

// reapExpiredSlots runs while Backend.ctx is live and reclaims slots whose
// deadline has passed. Safety net for CLIs that crash between spawn and
// thread/start — with no adopt or ReleaseFrame path to fire, the reaper is
// the only way the slot ever gets returned to Runtime as a failed frame.
func (b *Backend) reapExpiredSlots() {
	ticker := time.NewTicker(reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.reapOnce(time.Now())
		}
	}
}

// reapOnce runs one iteration of the reap loop against the given clock.
// Extracted so tests can exercise the takeIfExpired → failFrame →
// ReleaseFrame chain without depending on the ticker cadence or the
// production goroutine. A revert of the shared-cleanup wiring surfaces
// as an assertion failure here.
func (b *Backend) reapOnce(now time.Time) {
	expired := b.initState.takeIfExpired(now)
	if expired == nil {
		return
	}
	// Notify Runtime BEFORE dropping the binding — failFrame reads
	// b.frames[frameID] under b.mu, so the emit must happen first or it
	// short-circuits on nil binding.
	b.failFrame(expired.frameID, errInitAdoptExpired)
	// Reuse ReleaseFrame for the binding delete + worktree cleanup.
	// ReleaseFrame's releaseOwnSlot is a no-op here because takeIfExpired
	// already consumed the slot (takeIfOwned returns false when
	// initState.slot is nil). This keeps the two cleanup paths — reap and
	// Runtime-driven kill — sharing one code path.
	b.ReleaseFrame(expired.frameID)
	slog.Warn("stream backend: reaping expired pending frame slot",
		"subsystem", b.subsystemID, "frame", expired.frameID,
		"deadline_ago", time.Since(expired.deadline))
}
