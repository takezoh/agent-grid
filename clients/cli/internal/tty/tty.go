// Package tty is the only place in this module that touches the local
// terminal. Raw entry, restore, size query, size-change events, input-queue
// discard and terminal detection all pass through the Terminal interface, so
// the attach runtime can be driven to every one of its exit paths by a fake
// with no real terminal present.
//
// Confining the side effects is not tidiness: the terminal is process-global
// mutable state that outlives this program. A path that changes it and does not
// change it back leaves the operator with a shell that does not echo, and the
// only way to test every such path is to make every such path go through one
// object a test can supply.
package tty

import (
	"errors"
	"io"
	"os"
	"sync"

	"golang.org/x/term"
)

// ErrNotATerminal is returned by Open when standard input is not a terminal.
// It is returned *before* anything is changed, which is what lets attach fail
// on a pipe without having touched the invoking terminal at all.
var ErrNotATerminal = errors.New("standard input is not a terminal")

// ErrUnsupportedPlatform marks an operation this build has deliberately not
// implemented for the current platform, because the behaviour has not been
// observed on it yet. It is returned rather than guessed: a wrong guess about a
// console leaves the terminal in a state the operator has to fix by hand.
var ErrUnsupportedPlatform = errors.New("not implemented on this platform yet")

// Terminal is every terminal side effect this program performs.
type Terminal interface {
	// MakeRaw switches the terminal into raw mode. It is called only after a
	// subscription has reached a terminal accepted outcome
	// (adr-20260814-cli-raw-mode-critical-section), never at start-up.
	MakeRaw() error
	// Restore returns the terminal to the state it had when Open succeeded.
	// It is idempotent and safe to call from a deferred path and from a signal
	// handler, because on some exits both will run.
	Restore() error
	// Size reports the current geometry.
	Size() (cols, rows int, err error)
	// SizeChanges yields once per local size change. It never closes; the
	// caller stops selecting on it.
	SizeChanges() <-chan struct{}
	// DiscardInputQueue drops bytes the user has typed that the terminal
	// driver has not yet handed over. It runs before Restore on detach, so the
	// keystrokes after the detach key reach neither the remote session nor the
	// returning shell.
	DiscardInputQueue() error
	// Reader is the terminal input stream.
	Reader() io.Reader
	// Writer is the terminal output stream.
	Writer() io.Writer
	// Close restores and releases.
	Close() error
}

// Opener opens the controlling terminal. It is the seam the harness registry
// declares for the local-terminal boundary.
type Opener func() (Terminal, error)

// IsTerminal reports whether a file is a character device the user is looking
// at. It is the one terminal observation that happens before a Terminal is
// opened, which is why it is a free function.
func IsTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// Open opens the controlling terminal for reading and writing.
//
// It fails before changing anything when standard input is not a terminal. That
// ordering is the contract: FR-004 requires a non-TTY attach to leave the
// invoking terminal untouched, and a function that switched to raw mode and
// then discovered the problem would already have violated it.
func Open() (Terminal, error) {
	return openTerminal(os.Stdin, os.Stdout)
}

// openTerminal is Open with the two files named rather than taken from the
// process. The production path passes standard input and output; the fidelity
// backstop passes a terminal it allocated itself, so it can observe the real
// implementation against real terminal settings without depending on what the
// test runner happened to leave on standard input.
func openTerminal(in, out *os.File) (Terminal, error) {
	if !IsTerminal(in) {
		return nil, ErrNotATerminal
	}
	descriptor := int(in.Fd())
	state, err := term.GetState(descriptor)
	if err != nil {
		return nil, err
	}
	local := &localTerminal{
		in:         in,
		out:        out,
		descriptor: descriptor,
		original:   state,
		changes:    make(chan struct{}, 1),
	}
	local.watchSize()
	return local, nil
}

// localTerminal is the production Terminal.
type localTerminal struct {
	in         *os.File
	out        *os.File
	descriptor int
	original   *term.State

	mu       sync.Mutex
	raw      bool
	restored bool
	changes  chan struct{}
	stopSize func()
}

// MakeRaw enters raw mode. It also clears the restored flag, because suspend
// and resume legitimately go raw → cooked → raw within one attach, and a
// terminal that refused to restore a second time would hand the shell back in
// raw mode after a resume.
func (local *localTerminal) MakeRaw() error {
	local.mu.Lock()
	defer local.mu.Unlock()
	if local.raw {
		return nil
	}
	if _, err := term.MakeRaw(local.descriptor); err != nil {
		return err
	}
	local.raw = true
	local.restored = false
	return nil
}

// Restore is the single restore path.
//
// Idempotence is not a convenience here. On a signal exit both the handler and
// the deferred call run, and on a normal detach only the deferred one does; a
// restore that corrupted the terminal when called twice would make the signal
// path unsafe, and a program that therefore restored on only one path would
// leave the terminal broken on the other.
func (local *localTerminal) Restore() error {
	local.mu.Lock()
	defer local.mu.Unlock()
	if local.restored || !local.raw {
		local.restored = true
		return nil
	}
	err := term.Restore(local.descriptor, local.original)
	local.restored = true
	local.raw = false
	return err
}

func (local *localTerminal) Size() (int, int, error) {
	return term.GetSize(local.descriptor)
}

func (local *localTerminal) SizeChanges() <-chan struct{} { return local.changes }

func (local *localTerminal) Reader() io.Reader { return local.in }

func (local *localTerminal) Writer() io.Writer { return local.out }

func (local *localTerminal) Close() error {
	if local.stopSize != nil {
		local.stopSize()
	}
	return local.Restore()
}

// notifySize delivers a size-change edge without blocking. Coalescing here is
// deliberate: the consumer re-queries the size, so N pending edges and one
// pending edge lead to the same request, and dropping the extras is what stops
// a resize drag from queueing stale geometry behind the current one.
func (local *localTerminal) notifySize() {
	select {
	case local.changes <- struct{}{}:
	default:
	}
}
