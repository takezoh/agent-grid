package attach

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway/fakegateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

// Contract: contract-terminal-mode-lifecycle, contract-detach-boundary,
// contract-exit-classification, contract-transport-close-taxonomy
// (unit-attach-runtime).

// exitPaths is every way an established attach can end. Each entry drives the
// runtime to that ending and says what it should report.
//
// Enumerating them in one table is the point: the failure mode this whole
// contract exists to prevent is an implementation that restores the terminal on
// the paths its author thought of and leaves it raw on the one they did not.
var exitPaths = []struct {
	name      string
	drive     func(attached *harness)
	departure Departure
	reason    gateway.CloseReason
}{
	{
		name:      "the operator detaches",
		drive:     func(attached *harness) { _ = attached.terminal.Type([]byte{0x1d}) },
		departure: DepartedByDetach,
	},
	{
		name:      "terminal input ends",
		drive:     func(attached *harness) { attached.terminal.EndInput() },
		departure: DepartedByInputEnd,
	},
	{
		name:      "the terminal goes away",
		drive:     func(attached *harness) { attached.terminal.FailInput(tty.ErrFakeTerminalGone) },
		departure: DepartedByInputEnd,
	},
	{
		name:      "a catchable terminating signal arrives",
		drive:     func(attached *harness) { attached.signals <- os.Interrupt },
		departure: DepartedBySignal,
	},
	{
		name: "the session ends server-side",
		drive: func(attached *harness) {
			attached.socket.PushViewUpdate([]gateway.SessionInfo{endedSession(testSessionID)})
		},
		departure: DepartedBySessionEnd,
	},
	{
		name:      "the server drains",
		drive:     func(attached *harness) { attached.socket.Depart(gateway.ReasonGoingAway) },
		departure: DepartedByTransportLoss,
		reason:    gateway.ReasonGoingAway,
	},
	{
		name:      "the backend disappears",
		drive:     func(attached *harness) { attached.socket.Depart(gateway.ReasonBackendUnavailable) },
		departure: DepartedByTransportLoss,
		reason:    gateway.ReasonBackendUnavailable,
	},
	{
		name:      "the connection drops carrying nothing",
		drive:     func(attached *harness) { attached.socket.DropAbruptly() },
		departure: DepartedByTransportLoss,
		reason:    gateway.ReasonUnattributed,
	},
}

// TestTerminalRestoredOnEveryExitPath is ver-restore-on-every-exit-path, and
// TestFakeTTYDrivesEveryExitPath is ver-tty-seam-drives-all-exit-paths — the
// same table proves both, because the table is only writable at all because
// every one of these endings is reachable through the fake.
func TestTerminalRestoredOnEveryExitPath(t *testing.T) {
	t.Parallel()

	for _, path := range exitPaths {
		t.Run(path.name, func(t *testing.T) {
			t.Parallel()
			attached := newHarness(t, nil)
			attached.waitEstablished()

			path.drive(attached)
			result := attached.awaitResult()

			if result.Departure != path.departure {
				t.Errorf("departed as %v, want %v", result.Departure, path.departure)
			}
			if attached.terminal.IsRaw() {
				t.Error("the terminal was left in raw mode")
			}
			if attached.terminal.EffectiveRestores != 1 {
				t.Errorf("%d effective restores, want exactly 1",
					attached.terminal.EffectiveRestores)
			}
		})
	}
}

func TestFakeTTYDrivesEveryExitPath(t *testing.T) {
	t.Parallel()

	if len(exitPaths) < 8 {
		t.Fatalf("only %d exit paths are driven; the enumeration has shrunk", len(exitPaths))
	}
	seen := make(map[Departure]bool, len(exitPaths))
	for _, path := range exitPaths {
		seen[path.departure] = true
	}
	for _, required := range []Departure{
		DepartedByDetach, DepartedByInputEnd, DepartedBySignal,
		DepartedBySessionEnd, DepartedByTransportLoss,
	} {
		if !seen[required] {
			t.Errorf("departure %v is not reachable through the fake terminal", required)
		}
	}
}

// TestDepartureReasonsFollowThePublishedCloseReason is
// ver-departure-classes-follow-the-published-reason.
//
// Three things must hold at once: the reason is the plane's word rather than a
// sentence written here, two departure classes never share one reason text, and
// a loss that published nothing is presented as unattributed rather than
// dressed up as one of the named causes.
func TestDepartureReasonsFollowThePublishedCloseReason(t *testing.T) {
	t.Parallel()

	seen := make(map[string][]string)
	for _, path := range exitPaths {
		t.Run(path.name, func(t *testing.T) {
			t.Parallel()
			attached := newHarness(t, nil)
			attached.waitEstablished()
			path.drive(attached)
			result := attached.awaitResult()

			if result.Reason != path.reason {
				t.Errorf("reason = %q, want %q", result.Reason, path.reason)
			}
			if path.departure == DepartedByTransportLoss {
				if result.Failure() == nil {
					t.Error("a transport loss reported no failure")
				}
			} else if result.Failure() != nil {
				t.Errorf("an ordinary ending reported a failure: %v", result.Failure())
			}
		})
		seen[string(path.reason)] = append(seen[string(path.reason)], path.name)
	}

	// The empty reason belongs to the endings that are not transport losses;
	// every published reason names exactly one of them.
	for reason, paths := range seen {
		if reason == "" {
			continue
		}
		if len(paths) > 1 {
			t.Errorf("reason %q is shared by %v", reason, paths)
		}
	}
}

// TestEachPublishedCloseReasonMapsToItsExitClass is
// ver-cli-maps-each-close-reason-to-its-exit-class.
//
// The two failures this rejects are collapsing reasons the contract separates —
// a capacity refusal and a rejected credential lead to different next actions —
// and guessing a class for a departure that published nothing.
func TestEachPublishedCloseReasonMapsToItsExitClass(t *testing.T) {
	t.Parallel()

	classes := map[gateway.CloseReason]error{}
	for _, reason := range gateway.PublishedCloseReasons {
		result := Result{Departure: DepartedByTransportLoss, Reason: reason}
		failure := result.Failure()
		if failure == nil {
			t.Fatalf("%q produced no failure", reason)
		}
		var transport *gateway.TransportError
		if !errors.As(failure, &transport) {
			t.Fatalf("%q produced %T", reason, failure)
		}
		if transport.Reason != reason {
			t.Errorf("%q lost its reason on the way to a failure: %q", reason, transport.Reason)
		}
		classes[reason] = transport.Kind
	}

	// Reasons the contract separates must not share a kind.
	for _, pair := range [][2]gateway.CloseReason{
		{gateway.ReasonCapacity, gateway.ReasonUnauthenticated},
		{gateway.ReasonCapacity, gateway.ReasonBackendUnavailable},
		{gateway.ReasonUnauthenticated, gateway.ReasonBackendUnavailable},
		{gateway.ReasonIncompatibleProtocol, gateway.ReasonBackendUnavailable},
		{gateway.ReasonInvalidRequest, gateway.ReasonInternalError},
	} {
		if classes[pair[0]] == classes[pair[1]] {
			t.Errorf("%q and %q collapse to the same class %v", pair[0], pair[1], classes[pair[0]])
		}
	}

	// A departure that published nothing keeps saying so.
	unattributed := Result{Departure: DepartedByTransportLoss, Reason: gateway.ReasonUnattributed}
	var transport *gateway.TransportError
	if !errors.As(unattributed.Failure(), &transport) {
		t.Fatal("an unattributed loss produced no transport failure")
	}
	if transport.Reason != gateway.ReasonUnattributed {
		t.Errorf("an unattributed loss was reported as %q", transport.Reason)
	}
	if transport.Reason.IsPublished() {
		t.Error("the unattributed sentinel was reported as a published reason")
	}
}

// TestPreSubscribeFailureNeverTouchesTerminal is
// ver-pre-subscribe-failure-leaves-terminal-untouched.
//
// The tempting implementation goes raw at start-up and relies on a deferred
// restore. It is wrong even though the restore runs: between the two, a
// refusal's diagnostic is printed to a terminal in raw mode, so the message
// explaining the failure arrives without line breaks.
func TestPreSubscribeFailureNeverTouchesTerminal(t *testing.T) {
	t.Parallel()

	for name, configure := range map[string]func(*fakegateway.Fake){
		"the subscription is refused": func(fake *fakegateway.Fake) {
			fake.SubscribeCode = gateway.CodeResourceExhausted
		},
		"the session is gone": func(fake *fakegateway.Fake) {
			fake.SubscribeCode = gateway.CodeNotFound
		},
		"the daemon rejects the publication": func(fake *fakegateway.Fake) {
			fake.SubscribeOutcomes = []string{gateway.OutcomeRejected}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attached := newHarness(t, configure)
			if err := attached.awaitFailure(); err == nil {
				t.Fatal("the attach succeeded")
			}
			if attached.terminal.RawCalls != 0 {
				t.Error("the terminal was switched to raw mode before the failure was known")
			}
			if attached.terminal.EffectiveRestores != 0 {
				t.Error("a terminal that was never changed was restored")
			}
		})
	}
}

// TestNonTTYStdinFailsWithoutModeChange is ver-non-tty-stdin-fails-without-mode-change.
//
// The ordering is the contract: the refusal happens before any mode change and
// before a subscription exists, so a piped invocation leaves the invoking
// terminal untouched and the target session unaffected.
func TestNonTTYStdinFailsWithoutModeChange(t *testing.T) {
	t.Parallel()

	var opened bool
	terminal, err := OpenTerminal(func() (tty.Terminal, error) {
		opened = true
		return nil, tty.ErrNotATerminal
	})
	if !errors.Is(err, tty.ErrNotATerminal) {
		t.Fatalf("OpenTerminal on a pipe = %v, want ErrNotATerminal", err)
	}
	if terminal != nil {
		t.Error("a terminal was returned for a non-terminal input")
	}
	if !opened {
		t.Error("the opener was never consulted")
	}

	if _, err := OpenTerminal(nil); !errors.Is(err, ErrTerminalRequired) {
		t.Errorf("OpenTerminal(nil) = %v, want ErrTerminalRequired", err)
	}
}

// TestBytesAfterDetachKeySurfaceNowhere is ver-post-detach-bytes-surface-nowhere.
//
// "Nowhere" is two places: the remote session, and the shell the operator is
// returning to. The first is this pump's job; the second is the terminal
// driver's queue, which is why the discard happens before cooked mode returns.
func TestBytesAfterDetachKeySurfaceNowhere(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	if err := attached.terminal.Type([]byte("before\x1drm -rf /\n")); err != nil {
		t.Fatalf("type: %v", err)
	}
	result := attached.awaitResult()
	if result.Departure != DepartedByDetach {
		t.Fatalf("departed as %v, want a detach", result.Departure)
	}

	var forwarded strings.Builder
	for _, frame := range attached.framesOfKind("i") {
		if payload, ok := frame["d"].(string); ok {
			forwarded.WriteString(payload)
		}
	}
	if forwarded.String() != "before" {
		t.Errorf("forwarded %q; everything after the detach key must be dropped", forwarded.String())
	}
	if strings.Contains(forwarded.String(), "\x1d") {
		t.Error("the detach byte itself was forwarded")
	}
}

// TestInputQueueFlushedBeforeRestore is ver-input-queue-flushed-before-restore.
//
// Stopping the reader is not enough. What the driver still holds is handed to
// the shell the moment cooked mode returns, so a half-typed command the
// operator abandoned becomes an executed one.
func TestInputQueueFlushedBeforeRestore(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()
	if err := attached.terminal.Type([]byte{0x1d}); err != nil {
		t.Fatalf("type: %v", err)
	}
	attached.awaitResult()

	if attached.terminal.Discards == 0 {
		t.Fatal("the terminal input queue was never discarded")
	}
	if !attached.terminal.DiscardedBeforeRestore {
		t.Error("the queue was discarded after cooked mode returned, which is too late")
	}
}

// TestOnlyConfiguredDetachByteTriggersDetach is ver-configured-detach-byte-only.
//
// Once the byte is configurable, the default stops being special: an
// implementation that keeps 0x1D as a trigger alongside the configured one
// detaches on a keystroke the operator has deliberately reassigned to the
// remote agent.
func TestOnlyConfiguredDetachByteTriggersDetach(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]struct {
		configured byte
		typed      []byte
		forward    string
		detaches   bool
	}{
		"the default detaches when it is the trigger": {
			configured: 0x1d, typed: []byte("ab\x1d"), forward: "ab", detaches: true,
		},
		"the default is forwarded when it is not the trigger": {
			configured: 0x02, typed: []byte("a\x1db"), forward: "a\x1db", detaches: false,
		},
		"the reconfigured byte detaches": {
			configured: 0x02, typed: []byte("a\x02b"), forward: "a", detaches: true,
		},
	} {
		forward, detached := splitAtDetachByte(expected.typed, expected.configured)
		if detached != expected.detaches {
			t.Errorf("%s: detached = %v, want %v", name, detached, expected.detaches)
		}
		if string(forward) != expected.forward {
			t.Errorf("%s: forwarded %q, want %q", name, forward, expected.forward)
		}
	}

	// Driven end to end with the default reassigned: 0x1D must reach the
	// session rather than ending the attach.
	attached := newHarness(t, nil, func(options *Options) { options.DetachByte = 0x02 })
	attached.waitEstablished()
	if err := attached.terminal.Type([]byte{0x1d}); err != nil {
		t.Fatalf("type: %v", err)
	}
	attached.waitFor("the reassigned default was not forwarded", func() bool {
		for _, frame := range attached.framesOfKind("i") {
			if frame["d"] == "\x1d" {
				return true
			}
		}
		return false
	})
	select {
	case result := <-attached.result:
		t.Fatalf("0x1D detached after being reassigned: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestNoTerminateRequestOnAnyAttachShutdownPath is ver-detach-sends-no-terminate.
//
// FR-001 lists the paths one by one because each is a place someone has
// reflexively added a cleanup call: detach, Ctrl-C, terminal close, input end,
// signal, WebSocket close. Every one of them here must leave the session
// running.
func TestNoTerminateRequestOnAnyAttachShutdownPath(t *testing.T) {
	t.Parallel()

	for _, path := range exitPaths {
		t.Run(path.name, func(t *testing.T) {
			t.Parallel()
			attached := newHarness(t, nil)
			attached.waitEstablished()
			path.drive(attached)
			attached.awaitResult()

			if len(attached.fake.Deletes) != 0 {
				t.Errorf("a termination was emitted on this path: %v", attached.fake.Deletes)
			}
			for _, frame := range attached.outboundFrames() {
				if frame["desired"] == true {
					continue
				}
				if kind, _ := frame["k"].(string); kind != "ld" && kind != "i" && kind != "r" {
					t.Errorf("an unexpected frame kind %q was sent on a shutdown path", kind)
				}
			}
		})
	}
}
