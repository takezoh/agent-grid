//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package tty

import "golang.org/x/sys/unix"

// fread is the BSD sys/file.h flag naming the read side of a descriptor
// (FREAD 0x0001, FWRITE 0x0002). golang.org/x/sys/unix exports TIOCFLUSH but
// not this constant, so it is written out here rather than reached for.
//
// Passing only the read flag is what makes this the counterpart of Linux's
// TCIFLUSH: output already on its way to the screen is not this program's to
// discard, and a flush that took both would erase what the session had just
// drawn.
const fread = 0x0001

// flushTerminalInput discards the driver's input queue.
//
// TIOCFLUSH takes a pointer to the flag word, which is why this uses
// IoctlSetPointerInt rather than the IoctlSetInt the Linux path uses with
// TCFLSH; the two families spell the same operation differently and passing the
// value by copy here would flush whatever happened to be at that address.
//
// This has not been run on a BSD machine from this repository. What backs it is
// the ioctl number x/sys exports for these platforms, the documented flag, a
// clean cross-compile, and TestE2E_DiscardInputQueueEmptiesTheDriverQueue,
// which observes the effect on a real pty wherever one can be allocated — and
// which is proven to be able to fail by the control case beside it.
func flushTerminalInput(descriptor uintptr) error {
	return unix.IoctlSetPointerInt(int(descriptor), unix.TIOCFLUSH, fread)
}
