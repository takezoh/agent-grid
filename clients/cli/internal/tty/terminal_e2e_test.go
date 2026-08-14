//go:build e2e

package tty

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// terminal_e2e_test.go is the fidelity backstop for the local-terminal
// boundary.
//
// The fake records that raw mode was entered and restored; only a real terminal
// can show that the settings actually changed and actually came back. That is
// the whole gap, and it is why the local terminal needs its own registry entry
// rather than riding on the server-side pty exception: that exception covers a
// pty the daemon allocates for an agent, and this is the terminal this program
// puts into raw mode.
//
// The terminal is allocated here rather than inherited. An earlier version
// required standard input to be a terminal, which no invocation of `go test`
// can satisfy — the go command hands the test binary a null device — so the
// backstop could only ever skip, including on a machine with a real console.
// A freshly allocated pty is a real terminal as far as termios is concerned,
// which is exactly the fact under test, and it belongs to this test rather
// than to the operator, so raw mode is never entered on a console someone is
// looking at.
//
// A Windows console has no termios and no pty to allocate, so it is observed
// by its own file, terminal_windows_e2e_test.go, through the console API.
//
// It skips unless AG_E2E_REAL_TTY is set, which is the expected state on a
// pull-request runner.

// terminalStateCycle is the shared input: enter raw, observe, restore, observe.
// The fake and the real terminal are driven by identical calls.
func terminalStateCycle(t *testing.T, terminal Terminal) (rawEntered, restored bool) {
	t.Helper()
	if err := terminal.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	rawEntered = observeRaw(terminal)
	if err := terminal.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// Restore is called twice on purpose: a signal exit runs both a handler and
	// a deferred call, and the second must be harmless.
	if err := terminal.Restore(); err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	restored = !observeRaw(terminal)
	return rawEntered, restored
}

// observeRaw reports whether the terminal is currently in raw mode. The fake
// answers from its own record; the real one is asked about its actual settings,
// on its own descriptor rather than on the process's standard input.
func observeRaw(terminal Terminal) bool {
	if fake, ok := terminal.(*Fake); ok {
		return fake.IsRaw()
	}
	local, ok := terminal.(*localTerminal)
	if !ok {
		return false
	}
	state, err := term.GetState(local.descriptor)
	if err != nil {
		return false
	}
	// There is no "is it raw" query. The comparison is against the state
	// captured when the terminal was opened, which is the only portable way to
	// ask whether the settings moved and came back.
	return state != nil && local.original != nil && *state != *local.original
}

// openRealTerminal returns the production Terminal over a fresh pty slave, and
// the master. Writing to the master is how a test plays the part of someone
// typing, which is what the input-queue assertions need.
func openRealTerminal(t *testing.T) (Terminal, *os.File) {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		if errors.Is(err, pty.ErrUnsupported) {
			t.Skipf("this platform cannot allocate a pty: %v; "+
				"its console is covered by terminal_windows_e2e_test.go", err)
		}
		t.Fatalf("allocate a pty: %v", err)
	}
	t.Cleanup(func() {
		_ = slave.Close()
		_ = master.Close()
	})
	terminal, err := openTerminal(slave, slave)
	if err != nil {
		t.Fatalf("open the real terminal: %v", err)
	}
	t.Cleanup(func() { _ = terminal.Close() })
	return terminal, master
}

// readAvailable returns whatever the terminal hands over within the window.
// The read runs on its own goroutine because a pty does not honour read
// deadlines here; the descriptors are closed by cleanup, which releases it.
func readAvailable(terminal Terminal, window time.Duration) []byte {
	got := make(chan []byte, 1)
	go func() {
		buffer := make([]byte, 256)
		n, _ := terminal.Reader().Read(buffer)
		got <- buffer[:n]
	}()
	select {
	case data := <-got:
		return data
	case <-time.After(window):
		return nil
	}
}

// TestE2E_FakeVsRealTerminalRestore requires the fake and a real terminal to
// agree that raw entry changes the terminal and that restore puts it back.
func TestE2E_FakeVsRealTerminalRestore(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the terminal fidelity backstop needs a real terminal")
	}

	fakeRaw, fakeRestored := terminalStateCycle(t, NewFake(120, 40))
	real, _ := openRealTerminal(t)
	realRaw, realRestored := terminalStateCycle(t, real)

	if fakeRaw != realRaw {
		t.Errorf("raw entry differs: fake %v, real %v; fix the fake", fakeRaw, realRaw)
	}
	if fakeRestored != realRestored {
		t.Errorf("restore differs: fake %v, real %v; fix the fake", fakeRestored, realRestored)
	}
}

// TestE2E_RealTerminalGeometryIsReadBack requires the real terminal to report
// the geometry the kernel holds, not a remembered value. The fake answers from
// its own record, so nothing else in the suite can tell a Size() that queries
// from one that returns a constant.
func TestE2E_RealTerminalGeometryIsReadBack(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the terminal fidelity backstop needs a real terminal")
	}

	terminal, _ := openRealTerminal(t)
	local, ok := terminal.(*localTerminal)
	if !ok {
		t.Fatalf("openRealTerminal returned %T, not the production terminal", terminal)
	}
	if err := pty.Setsize(local.in, &pty.Winsize{Cols: 132, Rows: 43}); err != nil {
		t.Fatalf("set the pty size: %v", err)
	}

	cols, rows, err := terminal.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if cols != 132 || rows != 43 {
		t.Errorf("Size reported %dx%d, want 132x43", cols, rows)
	}
}

// TestE2E_DiscardInputQueueEmptiesTheDriverQueue observes the effect the fake
// can only record: that bytes already sitting in the terminal driver's input
// queue are gone afterwards.
//
// This is the assertion the detach boundary rests on. Keystrokes typed after
// the detach key live in two places — the ones this process already read, which
// the input pump drops itself, and the ones still in the driver, which would be
// handed to the shell the moment cooked mode returns and would execute whatever
// the user typed while they believed they were still talking to the agent.
//
// The control case is what makes it an assertion rather than a hope: it proves
// that bytes written to the master do reach a reader when nothing discards
// them, so the silence in the second case is the discard and not a broken
// probe. Without it, a flushTerminalInput that returned nil and did nothing
// would pass — and on the platforms this repository cannot run, that is exactly
// the failure to guard against.
func TestE2E_DiscardInputQueueEmptiesTheDriverQueue(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the terminal fidelity backstop needs a real terminal")
	}

	typed := []byte("rm -rf /\n")

	t.Run("control: without a discard the bytes arrive", func(t *testing.T) {
		terminal, master := openRealTerminal(t)
		if err := terminal.MakeRaw(); err != nil {
			t.Fatalf("MakeRaw: %v", err)
		}
		if _, err := master.Write(typed); err != nil {
			t.Fatalf("write to the master: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		if got := readAvailable(terminal, time.Second); len(got) == 0 {
			t.Fatal("nothing arrived even without a discard; " +
				"this probe cannot tell a flushed queue from an unread one")
		}
	})

	t.Run("after a discard nothing arrives", func(t *testing.T) {
		terminal, master := openRealTerminal(t)
		if err := terminal.MakeRaw(); err != nil {
			t.Fatalf("MakeRaw: %v", err)
		}
		if _, err := master.Write(typed); err != nil {
			t.Fatalf("write to the master: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		if err := terminal.DiscardInputQueue(); err != nil {
			t.Fatalf("DiscardInputQueue: %v", err)
		}
		if got := readAvailable(terminal, time.Second); len(got) != 0 {
			t.Errorf("%d bytes survived the discard (%q); "+
				"they would reach the shell when cooked mode returns", len(got), got)
		}
	})
}
