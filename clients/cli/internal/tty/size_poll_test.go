package tty

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// size_poll_test.go covers the decision the polling observer encodes. It runs
// everywhere, which is the point: the platform that needs polling is the one
// this repository cannot execute, so the logic is kept out of the platform file
// and only the geometry read is left there.

// scriptedSize serves a sequence of readings, repeating the last one.
type scriptedSize struct {
	mu       sync.Mutex
	readings [][2]int
	err      error
	index    int
	calls    int
}

func (s *scriptedSize) read() (int, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return 0, 0, s.err
	}
	reading := s.readings[s.index]
	if s.index < len(s.readings)-1 {
		s.index++
	}
	return reading[0], reading[1], nil
}

func (s *scriptedSize) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) inc() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

func (c *counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestPollSizeNotifiesOnlyWhenGeometryChanges(t *testing.T) {
	source := &scriptedSize{readings: [][2]int{
		{80, 24}, // baseline
		{80, 24}, // unchanged — must not notify
		{80, 24},
		{120, 40}, // changed — must notify once
		{120, 40}, // unchanged again
	}}
	notifications := &counter{}

	stop := pollSize(source.read, time.Millisecond, notifications.inc)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && source.callCount() < len(source.readings)+2 {
		time.Sleep(time.Millisecond)
	}
	stop()

	if got := notifications.get(); got != 1 {
		t.Fatalf("notifications = %d, want exactly 1 (only the 80x24 -> 120x40 step)", got)
	}
}

func TestPollSizeSurvivesAnUnreadableSize(t *testing.T) {
	source := &scriptedSize{readings: [][2]int{{80, 24}}, err: errors.New("no size")}
	notifications := &counter{}

	stop := pollSize(source.read, time.Millisecond, notifications.inc)
	time.Sleep(50 * time.Millisecond)
	stop()

	if got := notifications.get(); got != 0 {
		t.Fatalf("notifications = %d, want 0 when the size can never be read", got)
	}
	if source.callCount() == 0 {
		t.Fatal("the poller gave up after the first error instead of retrying")
	}
}

// TestPollSizeStopIsSynchronous pins that stop does not return while the
// goroutine is still running. A stop that returned early would let Close race
// a notification onto a channel the caller believes is finished with.
func TestPollSizeStopIsSynchronous(t *testing.T) {
	source := &scriptedSize{readings: [][2]int{{80, 24}, {81, 24}, {82, 24}}}
	notified := make(chan struct{}, 64)

	stop := pollSize(source.read, time.Millisecond, func() { notified <- struct{}{} })
	time.Sleep(20 * time.Millisecond)
	stop()

	drained := len(notified)
	time.Sleep(20 * time.Millisecond)
	if len(notified) != drained {
		t.Fatal("a notification arrived after stop returned")
	}
}

// TestPollSizeStopIsIdempotent guards the Close path. localTerminal.Close calls
// stopSize and is itself reachable twice on one exit — a deferred call and a
// signal handler — so a stop that closed its channel twice would panic during
// shutdown. The first version of pollSize did exactly that and this caught it.
func TestPollSizeStopIsIdempotent(t *testing.T) {
	source := &scriptedSize{readings: [][2]int{{80, 24}}}
	stop := pollSize(source.read, time.Millisecond, func() {})
	stop()
	stop()

	// The second stop must also mean the goroutine is gone, not merely that it
	// did not panic: a stop that returned early would leave the poller reading.
	settled := source.callCount()
	time.Sleep(20 * time.Millisecond)
	if got := source.callCount(); got != settled {
		t.Fatalf("read calls went %d -> %d after stop; the poller is still running", settled, got)
	}
}

func TestPollSizeDefaultsAnInvalidInterval(t *testing.T) {
	source := &scriptedSize{readings: [][2]int{{80, 24}}}
	stop := pollSize(source.read, 0, func() {})
	defer stop()
	// Long enough for the goroutine's baseline read, far short of the 250ms
	// default tick. Exactly one call is what proves 0 was replaced by the
	// default rather than taken literally as "poll continuously".
	time.Sleep(50 * time.Millisecond)
	if got := source.callCount(); got != 1 {
		t.Fatalf("read calls = %d, want 1 baseline read with the default interval", got)
	}
}
