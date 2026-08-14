//go:build e2e

package tty

import (
	"os"
	"testing"

	"golang.org/x/term"
)

// terminal_e2e_test.go is the fidelity backstop for the local-terminal
// boundary.
//
// The fake records that raw mode was entered and restored; only a real terminal
// can show that the settings actually changed and actually came back. That is
// the whole gap, and it is why the local terminal needs its own registry entry
// rather than riding on the server-side pty exception: that exception covers a
// pty the daemon allocates, and this is the operator's own console.
//
// It skips unless AG_E2E_REAL_TTY is set and standard input really is a
// terminal, which is the expected state on a pull-request runner.

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
// answers from its own record; the real one is asked about its actual settings.
func observeRaw(terminal Terminal) bool {
	if fake, ok := terminal.(*Fake); ok {
		return fake.IsRaw()
	}
	state, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return false
	}
	// A raw terminal does not echo. Restoring and re-reading the state is the
	// only portable way to ask, so the comparison is against the state captured
	// when the terminal was opened.
	return state != nil && rawDiffersFromOriginal(terminal, state)
}

func rawDiffersFromOriginal(terminal Terminal, state *term.State) bool {
	local, ok := terminal.(*localTerminal)
	if !ok {
		return false
	}
	return local.original != nil && *state != *local.original
}

// TestE2E_FakeVsRealTerminalRestore requires the fake and a real terminal to
// agree that raw entry changes the terminal and that restore puts it back.
func TestE2E_FakeVsRealTerminalRestore(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the terminal fidelity backstop needs a real terminal")
	}
	if !IsTerminal(os.Stdin) {
		t.Skip("standard input is not a terminal")
	}

	fakeRaw, fakeRestored := terminalStateCycle(t, NewFake(120, 40))

	real, err := Open()
	if err != nil {
		t.Fatalf("open the real terminal: %v", err)
	}
	defer func() { _ = real.Close() }()
	realRaw, realRestored := terminalStateCycle(t, real)

	if fakeRaw != realRaw {
		t.Errorf("raw entry differs: fake %v, real %v; fix the fake", fakeRaw, realRaw)
	}
	if fakeRestored != realRestored {
		t.Errorf("restore differs: fake %v, real %v; fix the fake", fakeRestored, realRestored)
	}
}
