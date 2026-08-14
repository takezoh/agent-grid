package attach

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway/fakegateway"
)

// Contract: contract-attach-transport-topology, contract-output-passthrough,
// contract-input-passthrough, contract-attach-lease-progress,
// contract-geometry-propagation (unit-attach-runtime).

// TestSubscribeSnapshotWrittenBeforeLiveOutput is ver-first-payload-written-first.
//
// The session's current screen is simply the first payload that arrives. An
// implementation that buffered payloads to "sort" them, or that rendered a
// reconstruction of its own before writing what the server sent, would show the
// operator something the session never printed.
func TestSubscribeSnapshotWrittenBeforeLiveOutput(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, func(fake *fakegateway.Fake) {
		fake.Snapshot = []byte("SCREEN-AS-IT-WAS")
	})
	attached.waitEstablished()
	attached.socket.PushOutput(testSessionID, []byte("-then-live"))

	attached.waitFor("live output never arrived", func() bool {
		return strings.Contains(attached.out.String(), "-then-live")
	})
	if got := attached.out.String(); got != "SCREEN-AS-IT-WAS-then-live" {
		t.Errorf("output order is %q, want the snapshot first", got)
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestForeignSessionOutputNeverReachesStdout is ver-foreign-session-output-dropped.
//
// The lifecycle socket multiplexes every session this connection subscribed to,
// and the routing information is the fourth element of the output array. Skip
// that check and another session's output lands on this terminal — which is not
// a cosmetic problem when the other session is printing a secret.
func TestForeignSessionOutputNeverReachesStdout(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	attached.socket.PushOutput(foreignSession, []byte("NOT-MINE"))
	attached.socket.PushOutput(testSessionID, []byte("mine"))

	attached.waitFor("own output never arrived", func() bool {
		return strings.Contains(attached.out.String(), "mine")
	})
	if strings.Contains(attached.out.String(), "NOT-MINE") {
		t.Errorf("another session's output reached this terminal: %q", attached.out.String())
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestCtrlCForwardedNotTrapped is ver-ctrl-c-forwarded-as-0x03.
//
// Ctrl-C interrupts the agent's current operation, not this client. A CLI that
// took it as a local interrupt would exit at exactly the moment the operator
// wanted the remote command to stop, leaving the command running and the
// operator detached.
func TestCtrlCForwardedNotTrapped(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	if err := attached.terminal.Type([]byte{0x03}); err != nil {
		t.Fatalf("type: %v", err)
	}
	attached.waitFor("Ctrl-C was not forwarded", func() bool {
		for _, frame := range attached.framesOfKind("i") {
			if frame["d"] == "\x03" {
				return true
			}
		}
		return false
	})

	select {
	case result := <-attached.result:
		t.Fatalf("Ctrl-C ended the attach: %+v", result)
	default:
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestNonTerminalOutcomesAreNotEstablished is
// ver-attach-never-reports-accepted-as-established.
//
// `accepted` and `waiting` are progress reports the daemon marks non-terminal.
// Entering raw mode on either would take the terminal for a subscription that
// has not been applied, and a subsequent rejection would then have to hand it
// back — after the operator has already seen their shell stop echoing.
func TestNonTerminalOutcomesAreNotEstablished(t *testing.T) {
	t.Parallel()

	for _, status := range []string{gateway.OutcomeAccepted, gateway.OutcomeWaiting} {
		if gateway.IsTerminalOutcome(status) {
			t.Errorf("%q is treated as terminal", status)
		}
		if gateway.IsEstablishedOutcome(status) {
			t.Errorf("%q is treated as established", status)
		}
	}
	for _, status := range []string{
		gateway.OutcomeApplied, gateway.OutcomeRejected, gateway.OutcomeSuperseded,
		gateway.OutcomeReleased, gateway.OutcomeDegraded, gateway.OutcomeNoOutput,
	} {
		if !gateway.IsTerminalOutcome(status) {
			t.Errorf("%q is not treated as terminal", status)
		}
	}
	if !gateway.IsEstablishedOutcome(gateway.OutcomeApplied) {
		t.Error("applied is not treated as established")
	}

	// Driven end to end: the daemon reports accepted, then applied. Raw mode
	// must not arrive until the second.
	attached := newHarness(t, func(fake *fakegateway.Fake) {
		fake.SubscribeOutcomes = []string{gateway.OutcomeAccepted, gateway.OutcomeApplied}
	})
	attached.waitEstablished()
	if attached.terminal.RawCalls != 1 {
		t.Errorf("raw mode was entered %d times", attached.terminal.RawCalls)
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestCapacityRefusalExitsTransportWithThePublishedReason is
// ver-attach-capacity-refusal-is-transport-with-the-published-reason.
//
// Three wrong answers are all available: report it as not-found, report it as a
// clean detach, or write a sentence of the client's own. The reason has to be
// the one the plane published, and the terminal must never have been touched.
func TestCapacityRefusalExitsTransportWithThePublishedReason(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, func(fake *fakegateway.Fake) {
		fake.SubscribeCode = gateway.CodeResourceExhausted
	})
	err := attached.awaitFailure()

	var transport *gateway.TransportError
	if !errors.As(err, &transport) {
		t.Fatalf("refusal surfaced as %T: %v", err, err)
	}
	if transport.Reason != gateway.ReasonCapacity {
		t.Errorf("reason = %q, want %q", transport.Reason, gateway.ReasonCapacity)
	}
	if !errors.Is(transport, gateway.ErrCapacity) {
		t.Errorf("a capacity refusal is not a capacity failure: %v", transport.Kind)
	}
	if errors.Is(transport, gateway.ErrNotFound) {
		t.Error("a capacity refusal was reported as not-found")
	}
	if attached.terminal.RawCalls != 0 {
		t.Error("raw mode was entered before the refusal was known")
	}
}

// TestConcurrentAttachEachReceivesSnapshot is
// ver-attach-topology-snapshot-two-clients.
//
// The disqualified topology is the per-session route, whose shared empty
// subscriber identity denies the second client its snapshot and drops both on
// one detach. Two independent attaches must each get their own screen, and one
// leaving must not disturb the other.
func TestConcurrentAttachEachReceivesSnapshot(t *testing.T) {
	t.Parallel()

	first := newHarness(t, func(fake *fakegateway.Fake) { fake.Snapshot = []byte("FIRST-SCREEN") })
	second := newHarness(t, func(fake *fakegateway.Fake) { fake.Snapshot = []byte("SECOND-SCREEN") })

	first.waitEstablished()
	second.waitEstablished()

	first.waitFor("the first client got no snapshot", func() bool {
		return strings.Contains(first.out.String(), "FIRST-SCREEN")
	})
	second.waitFor("the second client got no snapshot", func() bool {
		return strings.Contains(second.out.String(), "SECOND-SCREEN")
	})

	// One detaches; the other keeps receiving.
	if err := first.terminal.Type([]byte{0x1d}); err != nil {
		t.Fatalf("type detach: %v", err)
	}
	if got := first.awaitResult(); got.Departure != DepartedByDetach {
		t.Fatalf("the first client departed as %v", got.Departure)
	}

	second.socket.PushOutput(testSessionID, []byte("-still-here"))
	second.waitFor("the surviving client stopped receiving", func() bool {
		return strings.Contains(second.out.String(), "-still-here")
	})

	second.socket.Depart(gateway.ReasonGoingAway)
	second.awaitResult()
}

// TestGeometryDeclaredAtSubscribe is ver-initial-geometry-is-first-frame.
//
// A subscription without a size is refused by the server, so geometry cannot be
// something the client sends afterwards.
func TestGeometryDeclaredAtSubscribe(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	frames := attached.outboundFrames()
	if len(frames) == 0 {
		t.Fatal("nothing was sent")
	}
	first := frames[0]
	if first["k"] != "ld" {
		t.Fatalf("the first frame is %v, want the desired publication", first["k"])
	}
	if first["cols"] != float64(120) || first["rows"] != float64(40) {
		t.Errorf("the subscription declared %v×%v, want 120×40", first["cols"], first["rows"])
	}
	if first["desired"] != true {
		t.Error("the opening publication did not ask for the subscription")
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestGeometryClampedBeforeSend is ver-geometry-clamped-into-range.
//
// The server discards an out-of-range hint with a log line and no reply, so an
// unclamped client gets silence and a session still rendering at the old size.
// The zero case matters as much as the huge one: exactly one zero is an
// asymmetric hint the server rejects outright.
func TestGeometryClampedBeforeSend(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]struct{ in, want [2]int }{
		"far too large":   {in: [2]int{99999, 99999}, want: [2]int{MaxDimension, MaxDimension}},
		"one zero":        {in: [2]int{100, 0}, want: [2]int{100, MinDimension}},
		"both zero":       {in: [2]int{0, 0}, want: [2]int{MinDimension, MinDimension}},
		"negative":        {in: [2]int{-5, -5}, want: [2]int{MinDimension, MinDimension}},
		"already in band": {in: [2]int{120, 40}, want: [2]int{120, 40}},
	} {
		cols, rows := clampGeometry(expected.in[0], expected.in[1])
		if cols != expected.want[0] || rows != expected.want[1] {
			t.Errorf("%s: clamped to %d×%d, want %d×%d",
				name, cols, rows, expected.want[0], expected.want[1])
		}
	}

	// Driven end to end, so the clamp is on the path the frame actually takes.
	attached := newHarnessOn(t, 99999, 0, nil)
	attached.waitEstablished()

	opening := attached.outboundFrames()[0]
	if opening["cols"] != float64(MaxDimension) || opening["rows"] != float64(MinDimension) {
		t.Errorf("the subscription declared %v×%v, want the clamped values",
			opening["cols"], opening["rows"])
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestResizeBurstCoalescesToLatestValue is ver-resize-burst-coalesces-to-latest.
//
// Dragging a window edge produces a change per pixel row. Queueing every one of
// them applies a stale size last, leaving the session rendering at a width the
// window no longer has — the failure is not the extra frames, it is which frame
// ends up being the final one.
func TestResizeBurstCoalescesToLatestValue(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	attached.terminal.Resize(100, 30)
	attached.terminal.Resize(110, 33)
	attached.terminal.Resize(160, 50)

	attached.waitFor("no resize reached the server", func() bool {
		frames := attached.framesOfKind("r")
		return len(frames) > 0 &&
			frames[len(frames)-1]["cols"] == float64(160) &&
			frames[len(frames)-1]["rows"] == float64(50)
	})

	frames := attached.framesOfKind("r")
	last := frames[len(frames)-1]
	if last["cols"] != float64(160) || last["rows"] != float64(50) {
		t.Errorf("the last resize carried %v×%v, want the latest size", last["cols"], last["rows"])
	}
	if len(frames) > 3 {
		t.Errorf("%d resize frames for three changes; the burst was not coalesced", len(frames))
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestLeaseRevisionIsMonotonic is ver-lease-revision-monotonic.
//
// The daemon admits a newer desired publication over an older one by comparing
// this value. A reused or decremented revision is a renewal the daemon ignores,
// and an ignored renewal is a lease that lapses twelve seconds later for no
// visible reason.
func TestLeaseRevisionIsMonotonic(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()
	attached.waitFor("no renewals were sent", func() bool {
		return len(attached.framesOfKind("ld")) >= 4
	})

	var previous float64 = -1
	for index, frame := range attached.framesOfKind("ld") {
		correlation, ok := frame["correlation"].(map[string]any)
		if !ok {
			t.Fatalf("publication %d carries no correlation", index)
		}
		revision, ok := correlation["clientRevision"].(float64)
		if !ok {
			t.Fatalf("publication %d carries no client revision", index)
		}
		if revision <= previous {
			t.Errorf("publication %d has revision %v after %v; revisions must strictly increase",
				index, revision, previous)
		}
		previous = revision
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestLeaseRenewsWhileStdoutBlocked is ver-lease-renews-under-stdout-backpressure.
//
// The counterexample is an implementation that renews inside the output loop:
// it stops renewing the moment the terminal blocks — a paused pager, a stopped
// reader — and the subscription is gone after the twelve-second lease. The
// operator loses the session because they scrolled.
func TestLeaseRenewsWhileStdoutBlocked(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	attached := newHarness(t, func(fake *fakegateway.Fake) {
		fake.Snapshot = []byte("first")
	})
	attached.waitEstablished()

	// Block the terminal, then keep the session talking.
	attached.out.mu.Lock()
	attached.out.release = release
	attached.out.mu.Unlock()
	for range 5 {
		attached.socket.PushOutput(testSessionID, []byte("more"))
	}

	before := len(attached.framesOfKind("ld"))
	attached.waitFor("renewal stopped while the terminal was blocked", func() bool {
		return len(attached.framesOfKind("ld")) >= before+3
	})

	close(release)
	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}

// TestUTF8SequencesSurviveTerminalReads pins FR-006 on the path a real
// keystroke takes: a multi-byte character typed one byte at a time must reach
// the session as one character.
func TestUTF8SequencesSurviveTerminalReads(t *testing.T) {
	t.Parallel()

	attached := newHarness(t, nil)
	attached.waitEstablished()

	character := []byte("あ")
	for _, single := range character {
		if err := attached.terminal.Type([]byte{single}); err != nil {
			t.Fatalf("type: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	attached.waitFor("the character never arrived", func() bool {
		var assembled strings.Builder
		for _, frame := range attached.framesOfKind("i") {
			if payload, ok := frame["d"].(string); ok {
				assembled.WriteString(payload)
			}
		}
		return assembled.String() == "あ"
	})
	for _, frame := range attached.framesOfKind("i") {
		payload, _ := frame["d"].(string)
		if strings.ContainsRune(payload, '�') {
			t.Errorf("a frame carried a replacement character: %q", payload)
		}
	}

	attached.socket.Depart(gateway.ReasonGoingAway)
	attached.awaitResult()
}
