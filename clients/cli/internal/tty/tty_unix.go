//go:build !windows

package tty

import (
	"os"
	"os/signal"
	"syscall"
)

// tty_unix.go implements the two operations that have no portable spelling:
// observing a size change, and discarding what the terminal driver has buffered.

// watchSize subscribes to SIGWINCH, which is the kernel's own notification that
// the window changed. Polling would be the alternative and is worse in both
// directions: it misses changes between polls and it wakes a process that is
// otherwise idle for hours.
func (local *localTerminal) watchSize() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	done := make(chan struct{})
	local.stopSize = func() {
		signal.Stop(signals)
		close(done)
	}
	go func() {
		for {
			select {
			case <-done:
				return
			case <-signals:
				local.notifySize()
			}
		}
	}()
}

// DiscardInputQueue drops keystrokes the terminal driver holds but has not
// handed over.
//
// This is what makes the detach boundary real. Bytes typed after the detach key
// live in two places: the ones this process already read, which the input pump
// drops itself, and the ones still in the driver — and those would be handed to
// the shell the moment cooked mode returns, executing whatever the user typed
// while they thought they were still talking to the agent.
func (local *localTerminal) DiscardInputQueue() error {
	return flushTerminalInput(uintptr(local.descriptor))
}
