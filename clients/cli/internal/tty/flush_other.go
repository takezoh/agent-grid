//go:build !linux && !windows

package tty

// flushTerminalInput has no observed spelling on this platform.
//
// The BSD family spells it TIOCFLUSH with FREAD rather than Linux's TCFLSH with
// TCIFLUSH, and that spelling has not been run against a real terminal here.
// Returning the unsupported error rather than silently doing nothing keeps the
// detach boundary honest: a caller learns the queue was not discarded instead
// of believing it was. Implementing it means observing it first, exactly as the
// Windows console path does.
func flushTerminalInput(uintptr) error { return ErrUnsupportedPlatform }
