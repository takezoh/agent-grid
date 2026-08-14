//go:build !windows

package attach

import (
	"os"
	"syscall"
)

// isSuspend reports whether a signal asks this process to stop rather than to
// end. Suspend is the one catchable signal that must not end the attach: the
// shell takes the terminal back, the operator resumes, and the subscription was
// never in question.
func isSuspend(received os.Signal) bool {
	return received == syscall.SIGTSTP
}

// suspendProcess stops this process the way the shell expects, after the
// terminal has already been restored.
//
// SIGSTOP rather than SIGTSTP: the handler that brought us here has SIGTSTP
// intercepted, so re-raising it would be caught again instead of stopping.
// SIGSTOP cannot be caught, which is exactly what is wanted at this point —
// the terminal is already back in cooked mode.
func suspendProcess() {
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
}
