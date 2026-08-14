//go:build linux

package tty

import "golang.org/x/sys/unix"

// flushTerminalInput discards the driver's input queue. TCIFLUSH is the input
// half only: output already on its way to the screen is not this program's to
// throw away.
func flushTerminalInput(descriptor uintptr) error {
	return unix.IoctlSetInt(int(descriptor), unix.TCFLSH, unix.TCIFLUSH)
}
