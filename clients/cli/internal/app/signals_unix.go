//go:build !windows

package app

import (
	"os"
	"syscall"
)

// attachSignals are the signals an attach listens for.
//
// SIGINT is deliberately absent. In raw mode the terminal driver does not turn
// Ctrl-C into a signal at all — the byte is delivered and forwarded, which is
// what FR-005 requires — so a SIGINT here would come from somewhere else and is
// treated like any other terminating signal by the default disposition. What
// this list adds is the ability to restore the terminal before dying on the
// signals that a supervisor or a closing terminal emulator actually sends.
func attachSignals() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGHUP, syscall.SIGTSTP}
}
