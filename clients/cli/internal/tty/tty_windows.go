//go:build windows

package tty

// tty_windows.go is deliberately incomplete.
//
// Raw entry and restore go through golang.org/x/term, which has a Windows
// implementation, so they are shared with the Unix path. The two operations
// below are the ones with no Windows equivalent at all: there is no SIGWINCH,
// and there is no terminal driver queue to flush — the console host owns the
// input buffer.
//
// Guessing at replacements is what unit-platform-hardening-spikes exists to
// avoid. A polling interval chosen without measuring loses resizes or wakes an
// idle process; a console-buffer flush chosen without measuring either does
// nothing or discards more than the contract allows. Both wait on
// evidence-windows-resize-source and evidence-windows-console-behavior, and
// until then they say so rather than pretending.

// watchSize installs no observer. SizeChanges therefore never yields on
// Windows, which means an attached session keeps the geometry it declared at
// subscribe time. That is a known, recorded gap rather than a silent one.
func (local *localTerminal) watchSize() {
	local.stopSize = func() {}
}

// DiscardInputQueue reports that it did not discard anything.
func (local *localTerminal) DiscardInputQueue() error {
	return ErrUnsupportedPlatform
}
