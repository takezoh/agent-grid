//go:build !linux && !windows && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package tty

// flushTerminalInput has no known spelling on this platform.
//
// Linux, the BSD family and Windows each have one, and each has its own file.
// What is left here is everything else — solaris, aix, plan9, js — where no
// ioctl or console call has been identified.
//
// Returning the unsupported error rather than silently doing nothing keeps the
// detach boundary honest: a caller learns the queue was not discarded instead
// of believing it was.
func flushTerminalInput(uintptr) error { return ErrUnsupportedPlatform }
