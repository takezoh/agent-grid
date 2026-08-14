package tty

import (
	"errors"
	"os"
	"testing"
)

// Contract: contract-terminal-mode-lifecycle (unit-tty-adapter).

// TestRestoreIsIdempotent is ver-restore-is-idempotent.
//
// Both a deferred restore and a signal handler's restore run on a signal exit,
// and only the deferred one runs on a normal detach. A restore that misbehaved
// when called twice would make the signal path unsafe — and an implementation
// that therefore restored on only one path would leave the terminal broken on
// the other. So the requirement is that calling it repeatedly is harmless, not
// that it is called once.
func TestRestoreIsIdempotent(t *testing.T) {
	t.Parallel()

	fake := NewFake(120, 40)
	if err := fake.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	for range 5 {
		if err := fake.Restore(); err != nil {
			t.Fatalf("Restore: %v", err)
		}
	}
	if fake.EffectiveRestores != 1 {
		t.Errorf("%d restores actually changed the terminal, want exactly 1", fake.EffectiveRestores)
	}
	if fake.IsRaw() {
		t.Error("the terminal is still raw after restore")
	}
}

// TestRestoreWithoutRawIsHarmless covers the pre-subscription failure path,
// where the terminal was opened but never switched: restoring must be a no-op
// rather than an error the caller has to special-case.
func TestRestoreWithoutRawIsHarmless(t *testing.T) {
	t.Parallel()

	fake := NewFake(80, 24)
	if err := fake.Restore(); err != nil {
		t.Fatalf("Restore before MakeRaw: %v", err)
	}
	if fake.EffectiveRestores != 0 {
		t.Errorf("a terminal that was never raw was restored %d times", fake.EffectiveRestores)
	}
}

// TestFakeDrivesEveryTerminalOperation is the fake's own contract test: the
// registered triple requires the fake to cover the whole seam, and a fake that
// silently omits an operation would make an attach path untestable while
// looking covered.
func TestFakeDrivesEveryTerminalOperation(t *testing.T) {
	t.Parallel()

	var terminal Terminal = NewFake(100, 30)
	fake, _ := terminal.(*Fake)

	if err := terminal.MakeRaw(); err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	cols, rows, err := terminal.Size()
	if err != nil || cols != 100 || rows != 30 {
		t.Fatalf("Size = %d,%d,%v", cols, rows, err)
	}

	fake.Resize(160, 50)
	select {
	case <-terminal.SizeChanges():
	default:
		t.Fatal("a resize produced no size-change edge")
	}
	cols, rows, _ = terminal.Size()
	if cols != 160 || rows != 50 {
		t.Errorf("after the edge the size is %d,%d, want the new value", cols, rows)
	}

	if err := terminal.DiscardInputQueue(); err != nil {
		t.Fatalf("DiscardInputQueue: %v", err)
	}
	if !fake.DiscardedBeforeRestore {
		t.Error("the discard was not recorded as happening before restore")
	}

	go func() { _ = fake.Type([]byte("abc")) }()
	buffer := make([]byte, 3)
	if _, err := terminal.Reader().Read(buffer); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buffer) != "abc" {
		t.Errorf("read %q, want abc", buffer)
	}

	if _, err := terminal.Writer().Write([]byte("out")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if string(fake.Output()) != "out" {
		t.Errorf("output %q, want out", fake.Output())
	}

	if err := terminal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fake.Closed || fake.IsRaw() {
		t.Error("Close did not release and restore")
	}
}

// TestFailingInputSurfacesAsAReadError covers the terminal that goes away
// mid-session, which is one of the exit paths the attach runtime must restore
// on.
func TestFailingInputSurfacesAsAReadError(t *testing.T) {
	t.Parallel()

	fake := NewFake(80, 24)
	fake.FailInput(ErrFakeTerminalGone)
	if _, err := fake.Reader().Read(make([]byte, 1)); !errors.Is(err, ErrFakeTerminalGone) {
		t.Errorf("read error = %v, want ErrFakeTerminalGone", err)
	}
}

// TestOpenRefusesANonTerminalBeforeChangingAnything is the production half of
// FR-004. The test runs against a pipe, which is what a non-interactive
// invocation gives the process.
func TestOpenRefusesANonTerminalBeforeChangingAnything(t *testing.T) {
	t.Parallel()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = reader.Close(); _ = writer.Close() }()

	if IsTerminal(reader) {
		t.Fatal("a pipe reported itself as a terminal")
	}
	if IsTerminal(nil) {
		t.Error("a nil file reported itself as a terminal")
	}
}
