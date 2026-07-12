package runtime

import (
	"fmt"
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

// TestState exposes the runtime's state for test assertions. Must
// only be called from tests — production code accesses state
// exclusively through the event loop.
func (r *Runtime) TestState() state.State {
	return r.state
}

// TestPublishedState returns the latest state snapshot published by the event
// loop for cross-goroutine test assertions.
func (r *Runtime) TestPublishedState() state.State {
	if snapshot := r.published.Load(); snapshot != nil {
		return *snapshot
	}
	return state.New()
}

// TestEventQueueCapacity returns the public event queue size for saturation
// tests that need to drive the non-blocking drop branch deterministically.
func (r *Runtime) TestEventQueueCapacity() int {
	return cap(r.eventCh)
}

// TestEnqueue submits an external event with a bounded blocking send so tests
// can fail loudly instead of inheriting Runtime.Enqueue's best-effort drop
// semantics.
func (r *Runtime) TestEnqueue(ev state.Event, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r.eventCh <- ev:
		return nil
	case <-timer.C:
		return fmt.Errorf("runtime test enqueue timed out after %v for %s", timeout, eventTypeName(ev))
	}
}

// TestQuiesce blocks until both runtime queues are empty at a barrier point or
// returns an error after timeout.
func (r *Runtime) TestQuiesce(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("runtime test quiesce timed out after %v", timeout)
		}

		reply := make(chan bool, 1)
		barrier := internalBarrier{drained: reply}
		remaining := time.Until(deadline)
		timer := time.NewTimer(remaining)
		select {
		case r.internalChBulk <- barrier:
		case <-timer.C:
			return fmt.Errorf("runtime test quiesce enqueue timed out after %v", timeout)
		}
		if !timer.Stop() {
			<-timer.C
		}

		timer = time.NewTimer(time.Until(deadline))
		var drained bool
		select {
		case drained = <-reply:
		case <-timer.C:
			return fmt.Errorf("runtime test quiesce wait timed out after %v", timeout)
		}
		if !timer.Stop() {
			<-timer.C
		}
		if drained {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

func (r *Runtime) quiesced() bool {
	return len(r.eventCh) == 0 &&
		len(r.internalChInteractive) == 0 &&
		len(r.internalChBulk) == 0 &&
		len(r.pendingSpawns) == 0 &&
		(r.workers == nil || r.workers.Idle())
}
