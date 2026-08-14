//go:build windows

package tty

import (
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// tty_windows.go implements the two operations that have no portable spelling.
//
// An earlier version left both unimplemented and justified it by saying Windows
// has "no terminal driver queue to flush — the console host owns the input
// buffer". The first half of that is wrong: the console input buffer is exactly
// what FlushConsoleInputBuffer discards, and discarding it is the same act the
// Unix path performs with TCIFLUSH. The second operation had no signal to hang
// off, which was true, and remains the reason it polls.
//
// Neither has been observed on a real console from this repository. What backs
// them is the documented contract of two console APIs, a cross-compile, unit
// coverage of the polling decision in size_poll_test.go, and
// terminal_windows_e2e_test.go, which asserts both against a real console
// wherever one is available. That is a different position from guessing, and a
// different position from leaving the operations unimplemented forever.

// watchSize polls the console for its geometry.
//
// Windows has no SIGWINCH. Reading WINDOW_BUFFER_SIZE_EVENT from the console
// input buffer would be event-driven, but it consumes from the same buffer the
// input pump reads keystrokes out of, so the two would race for the user's
// typing — a resize observer that swallows input is worse than a slightly late
// resize. Polling reads a separate API and touches nothing the pump owns.
//
// The interval and the notify-only-on-change rule live in size_poll.go, where
// they are covered by tests that run on every platform.
func (local *localTerminal) watchSize() {
	local.stopSize = pollSize(
		func() (int, int, error) { return term.GetSize(local.descriptor) },
		DefaultSizePollInterval,
		local.notifySize,
	)
}

// DiscardInputQueue drops console input the user has typed that this process
// has not yet read.
//
// This is what makes the detach boundary real on Windows for the same reason it
// does on Unix: bytes typed after the detach key would otherwise be handed to
// the shell the moment the terminal returns to cooked mode, executing whatever
// the user typed while they believed they were still talking to the agent.
func (local *localTerminal) DiscardInputQueue() error {
	return windows.FlushConsoleInputBuffer(windows.Handle(local.descriptor))
}
