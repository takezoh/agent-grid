package tty

import (
	"sync"
	"time"
)

// DefaultSizePollInterval is how often a polling size observer asks the
// terminal for its geometry.
//
// Four wakeups a second is below what a person notices while dragging a window
// edge, and it is far less work than the cursor blink the same terminal is
// already doing. The Unix path does not poll at all — SIGWINCH is the kernel
// telling us — so this exists only for platforms that have no such signal.
const DefaultSizePollInterval = 250 * time.Millisecond

// pollSize watches a size source and calls notify each time the geometry
// changes. It returns a stop function.
//
// The comparison against the previous reading is the whole point: a poller that
// notified on every tick would turn an idle terminal into a resize storm, and
// the consumer re-queries the size on every notification.
//
// This is separated from the platform binding so the decision it encodes — poll,
// compare, notify only on change, stop cleanly — is testable on any machine.
// What remains platform-specific is one call that reads the geometry.
func pollSize(read func() (int, int, error), interval time.Duration, notify func()) func() {
	if interval <= 0 {
		interval = DefaultSizePollInterval
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		lastCols, lastRows, err := read()
		if err != nil {
			// An unreadable size at start-up is not fatal: the first successful
			// read becomes the baseline, and until then nothing is reported.
			lastCols, lastRows = -1, -1
		}
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cols, rows, err := read()
				if err != nil {
					continue
				}
				if cols == lastCols && rows == lastRows {
					continue
				}
				lastCols, lastRows = cols, rows
				notify()
			}
		}
	}()
	// Idempotent because Close is: localTerminal.Close calls stopSize and is
	// itself reachable from a deferred call and a signal handler on the same
	// exit. A stop that closed its channel twice would panic on the second
	// caller, turning an orderly shutdown into a crash.
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-stopped
	}
}
