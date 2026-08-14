//go:build e2e && windows

package tty

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// terminal_windows_e2e_test.go is the Windows half of the local-terminal
// boundary: the observations unit-platform-hardening-spikes needs from a real
// console, written so they run wherever one exists.
//
// The Unix backstop allocates its own pty. Windows has no equivalent to open,
// but it does have the console the test process is already attached to, and
// CONIN$/CONOUT$ address it by name rather than relying on whatever the runner
// left on standard input — which under `go test` is a null device either way.
//
// It skips when the process has no console, which is the expected state in a
// service or a redirected CI step.

// openRealConsole opens the attached console and builds the production Terminal
// over it.
func openRealConsole(t *testing.T) (Terminal, windows.Handle) {
	t.Helper()
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no console attached to this process: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no console output handle: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })

	terminal, err := openTerminal(in, out)
	if err != nil {
		t.Fatalf("open the console as a terminal: %v", err)
	}
	t.Cleanup(func() { _ = terminal.Close() })
	return terminal, windows.Handle(in.Fd())
}

// TestE2E_WindowsRawModeChangesAndRestoresConsoleMode closes the raw-mode half
// of evidence-windows-console-behavior: that raw entry really changes the
// console mode flags and that restore really puts them back.
//
// The fake records that the calls happened. Only a console can show that they
// had an effect, which is the whole reason this boundary carries its own
// registry entry.
func TestE2E_WindowsRawModeChangesAndRestoresConsoleMode(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the console backstop needs a real console")
	}
	terminal, handle := openRealConsole(t)

	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		t.Fatalf("read the original console mode: %v", err)
	}

	if err := terminal.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	var raw uint32
	if err := windows.GetConsoleMode(handle, &raw); err != nil {
		t.Fatalf("read the raw console mode: %v", err)
	}
	if raw == original {
		t.Errorf("console mode is unchanged after MakeRaw (0x%08x); raw entry did nothing", raw)
	}
	if raw&windows.ENABLE_LINE_INPUT != 0 {
		t.Errorf("ENABLE_LINE_INPUT is still set in raw mode (0x%08x); "+
			"the console would hold keystrokes until Enter", raw)
	}
	if raw&windows.ENABLE_ECHO_INPUT != 0 {
		t.Errorf("ENABLE_ECHO_INPUT is still set in raw mode (0x%08x); "+
			"the console would echo what the remote session also echoes", raw)
	}

	if err := terminal.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// Twice on purpose: a signal exit runs both a handler and a deferred call.
	if err := terminal.Restore(); err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	var restored uint32
	if err := windows.GetConsoleMode(handle, &restored); err != nil {
		t.Fatalf("read the restored console mode: %v", err)
	}
	if restored != original {
		t.Errorf("console mode after restore = 0x%08x, want the original 0x%08x",
			restored, original)
	}
}

// TestE2E_WindowsDiscardInputQueueEmptiesTheConsoleBuffer closes the
// input-queue half of evidence-windows-console-behavior.
//
// The claim this replaces was that Windows has no queue to flush. It has one:
// the console input buffer, which is what holds keystrokes the process has not
// read, and what would otherwise be handed to the shell when cooked mode
// returns after a detach.
func TestE2E_WindowsDiscardInputQueueEmptiesTheConsoleBuffer(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the console backstop needs a real console")
	}
	terminal, handle := openRealConsole(t)

	if err := terminal.DiscardInputQueue(); err != nil {
		t.Fatalf("DiscardInputQueue on a real console: %v", err)
	}
	var pending uint32
	if err := windows.GetNumberOfConsoleInputEvents(handle, &pending); err != nil {
		t.Fatalf("count pending console input events: %v", err)
	}
	if pending != 0 {
		t.Errorf("%d console input events remain after DiscardInputQueue, want 0", pending)
	}
}

// TestE2E_WindowsSizeReadsTheConsole requires Size to report what the console
// reports, rather than a remembered value — the same fact the Unix backstop
// establishes by resizing a pty behind the terminal's back.
func TestE2E_WindowsSizeReadsTheConsole(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the console backstop needs a real console")
	}
	terminal, _ := openRealConsole(t)

	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no console output handle: %v", err)
	}
	defer func() { _ = out.Close() }()
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(out.Fd()), &info); err != nil {
		t.Fatalf("GetConsoleScreenBufferInfo: %v", err)
	}
	wantCols := int(info.Window.Right - info.Window.Left + 1)
	wantRows := int(info.Window.Bottom - info.Window.Top + 1)

	cols, rows, err := terminal.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if cols != wantCols || rows != wantRows {
		t.Errorf("Size reported %dx%d, console reports %dx%d", cols, rows, wantCols, wantRows)
	}
}

// TestE2E_WindowsSizeObserverStartsAndStops requires the polling observer to be
// installed and to shut down cleanly.
//
// What it does not establish is that a resize fires it: x/sys/windows exposes no
// call to resize a console, so the test cannot cause one. The decision the
// observer encodes — poll, compare, notify only on change, stop synchronously
// and idempotently — is covered by size_poll_test.go on every platform, and
// what is left here is that the geometry source works, which the test above
// establishes.
func TestE2E_WindowsSizeObserverStartsAndStops(t *testing.T) {
	if os.Getenv("AG_E2E_REAL_TTY") == "" {
		t.Skip("AG_E2E_REAL_TTY is unset; the console backstop needs a real console")
	}
	terminal, _ := openRealConsole(t)

	local, ok := terminal.(*localTerminal)
	if !ok {
		t.Fatalf("openRealConsole returned %T, not the production terminal", terminal)
	}
	if local.stopSize == nil {
		t.Fatal("no size observer was installed; SizeChanges could never yield")
	}
	if terminal.SizeChanges() == nil {
		t.Fatal("SizeChanges returned a nil channel")
	}
	local.stopSize()
	local.stopSize() // Close reaches this twice on a signal exit.
}
