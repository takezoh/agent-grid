package app

import (
	"bytes"
	"strings"
	"testing"
)

// Contract: contract-exit-classification, contract-module-boundary
// (adr-20260814-cli-failure-class-exit-codes).

// TestFailureClassesMapToTheFixedExitCodes is
// ver-exit-codes-match-the-fixed-assignment.
//
// The assignment is asserted value by value rather than by round-tripping the
// table through itself, which would pass for any assignment at all. The second
// half is what stops two classes sharing a number, which is the failure that
// actually hurts: automation branching on the code would then be unable to tell
// a usage error from a missing session.
func TestFailureClassesMapToTheFixedExitCodes(t *testing.T) {
	t.Parallel()

	for _, expected := range []struct {
		class FailureClass
		code  int
	}{
		{ClassSuccess, 0},
		{ClassInternal, 1},
		{ClassUsage, 2},
		{ClassNotFound, 3},
		{ClassAmbiguous, 4},
		{ClassAuth, 5},
		{ClassTransport, 6},
	} {
		if got := ExitCode(expected.class); got != expected.code {
			t.Errorf("ExitCode(%s) = %d, want %d", expected.class, got, expected.code)
		}
	}

	seen := make(map[int]FailureClass, len(exitCodes))
	for class, code := range exitCodes {
		if other, taken := seen[code]; taken {
			t.Errorf("%s and %s both exit %d; every class needs its own code",
				class, other, code)
		}
		seen[code] = class
	}
	if len(exitCodes) != 7 {
		t.Errorf("the assignment has %d classes; the contract fixes exactly 7", len(exitCodes))
	}
}

// TestUnknownClassIsReportedAsAnInternalFault pins the one case the table
// cannot answer. Falling through to 0 would report a state this program does
// not define as a success.
func TestUnknownClassIsReportedAsAnInternalFault(t *testing.T) {
	t.Parallel()

	if got := ExitCode(FailureClass(99)); got != 1 {
		t.Errorf("ExitCode(undefined class) = %d, want 1", got)
	}
}

// TestPositionalSelectorIsAUsageError is the module-skeleton acceptance that
// the first argument is always a verb (DP-ENTRY-DISPATCH-STYLE / UAC-035).
//
// A session identifier looks exactly like an unknown verb, and that is the
// point: an implementation that treats a non-verb first argument as an attach
// target would attach here instead of exiting 2.
func TestPositionalSelectorIsAUsageError(t *testing.T) {
	t.Parallel()

	streams, _, errOut := captureStreams()
	class := Run(streams, []string{"s_0a1b2c3d4e5f"})

	if class != ClassUsage {
		t.Errorf("class = %s, want %s", class, ClassUsage)
	}
	if !strings.Contains(errOut.String(), "the first argument is always a verb") {
		t.Errorf("diagnostic does not say why:\n%s", errOut)
	}
}

// TestInteractiveAndPipedInvocationsAgree is ver-tty-and-non-tty-identical.
//
// FR-025 forbids branching output content or exit code on terminal detection.
// The check drives every invocation twice with the only thing that differs
// being the stdout-is-a-terminal observation, and requires byte equality of
// both streams as well as the same class. An implementation that colours its
// output, truncates to the terminal width, or adds a confirmation prompt fails
// here rather than in a pipeline someone is debugging at the time.
func TestInteractiveAndPipedInvocationsAgree(t *testing.T) {
	t.Parallel()

	invocations := [][]string{
		{},
		{"help"},
		{"version"},
		{"s_0a1b2c3d4e5f"},
		{"attach"},
		{"attach", "--id", "a", "--prefix", "b"},
		{"new"},
		{"new", "--server", "one"},
		{"kill", "--nonexistent-flag"},
		{"ls", "extra-positional"},
	}

	for _, argv := range invocations {
		interactiveStreams, interactiveOut, interactiveErr := captureStreams()
		interactiveStreams.OutIsTerminal = true
		interactiveClass := Run(interactiveStreams, argv)

		pipedStreams, pipedOut, pipedErr := captureStreams()
		pipedStreams.OutIsTerminal = false
		pipedClass := Run(pipedStreams, argv)

		if interactiveClass != pipedClass {
			t.Errorf("%v: class differs by terminal detection: %s vs %s",
				argv, interactiveClass, pipedClass)
		}
		if interactiveOut.String() != pipedOut.String() {
			t.Errorf("%v: stdout differs by terminal detection:\n%q\n%q",
				argv, interactiveOut, pipedOut)
		}
		if interactiveErr.String() != pipedErr.String() {
			t.Errorf("%v: stderr differs by terminal detection:\n%q\n%q",
				argv, interactiveErr, pipedErr)
		}
	}
}

func captureStreams() (Streams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return Streams{In: strings.NewReader(""), Out: out, Err: errOut}, out, errOut
}
