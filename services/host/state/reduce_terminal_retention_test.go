package state

import (
	"testing"
	"time"
)

// Contract: contract-session-terminal-state-retention
// (adr-20260814-session-terminal-state-retention).
//
// A session that ends — however it ends — stays observable with a terminal
// marker instead of being deleted, so every client can render the `stopped`
// lifecycle the protocol already declares. Before this change a terminal row
// was reachable on exactly one of four paths (an abnormal self-exit, and only
// as a side effect of the crash path); the other three deleted the record and
// left clients unable to tell "ended" from "never existed".
//
// The named tests below are the contract's verifications:
//   - ver-explicit-terminate-leaves-a-terminal-row
//   - ver-self-exit-paths-leave-a-terminal-row

// TestStopSessionRetainsATerminalRow is ver-explicit-terminate-leaves-a-terminal-row.
//
// Distinct from TestStopSessionRetainsATerminalRow's sibling assertions in
// reduce_session_test.go: this one pins the *observable* consequence — the row
// survives the reducer and reports a terminal marker — rather than the effect
// list. An implementation that emitted every teardown effect but still called
// delete() would pass the effect assertions and fail here.
func TestStopSessionLeavesATerminalRowObservable(t *testing.T) {
	s := New()
	// State.Now advances on tick; production always has one behind it by the
	// time a terminate arrives, so the terminal marker carries a real instant.
	s.Now = time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	id := SessionID("terminated")
	s.Sessions[id] = newExitSession(id)

	next, _ := Reduce(s, EvEvent{
		ConnID: 1, ReqID: "r", Event: "stop-session",
		Payload: mustPayload(map[string]string{"session_id": string(id)}),
	})

	sess, ok := next.Sessions[id]
	if !ok {
		t.Fatal("explicit termination deleted the session; a terminal row must remain")
	}
	if !sess.Ended() {
		t.Fatal("retained session carries no terminal marker")
	}
	if sess.EndedAt.IsZero() {
		t.Error("terminal marker has no timestamp")
	}
	if len(sess.Frames) != 0 || sess.HeadFrameID != "" {
		t.Errorf("terminal row still holds frames: frames=%d head=%q",
			len(sess.Frames), sess.HeadFrameID)
	}
}

// TestIntentionalExitAndFrameVanishRetainATerminalRow is
// ver-self-exit-paths-leave-a-terminal-row.
//
// Covers the two remaining deletion paths together because they share one
// reducer helper: a clean or signalled self-exit, and a backend frame that
// disappeared. Both must land in the same terminal state as explicit
// termination — a client cannot be asked to distinguish them by whether a row
// exists.
func TestIntentionalExitAndFrameVanishRetainATerminalRow(t *testing.T) {
	for _, code := range []int{0, 129, 130, 137, 143} {
		t.Run(exitCodeName(code), func(t *testing.T) {
			s := New()
			id := SessionID("self-exit")
			s.Sessions[id] = newExitSession(id)

			next, _ := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: code})

			sess, ok := next.Sessions[id]
			if !ok {
				t.Fatalf("self-exit %d deleted the session; a terminal row must remain", code)
			}
			if !sess.Ended() {
				t.Fatalf("self-exit %d left no terminal marker", code)
			}
			if len(sess.Frames) != 0 {
				t.Errorf("self-exit %d kept %d frame(s)", code, len(sess.Frames))
			}
		})
	}

	t.Run("frame_vanished", func(t *testing.T) {
		s := New()
		id := SessionID("vanished")
		s.Sessions[id] = newExitSession(id)

		next, _ := Reduce(s, EvFrameVanished{FrameID: FrameID(id)})

		sess, ok := next.Sessions[id]
		if !ok {
			t.Fatal("a vanished backend frame deleted the session; a terminal row must remain")
		}
		if !sess.Ended() {
			t.Fatal("a vanished backend frame left no terminal marker")
		}
		if len(sess.Frames) != 0 {
			t.Errorf("vanished path kept %d frame(s)", len(sess.Frames))
		}
	})
}

// TestSpawnFailureLeavesNoRow guards the boundary the retention contract does
// not cross. A session whose first spawn failed never reached a running state,
// so presenting it as `stopped` would invent a terminal state it never had.
// This is the counterexample to the retention rule, not an exception to it.
func TestSpawnFailureLeavesNoRow(t *testing.T) {
	s := New()
	id := SessionID("never-started")
	s.Sessions[id] = newExitSession(id)

	next, _ := Reduce(s, EvSpawnFailed{FrameID: FrameID(id), Err: "exec format error"})

	if _, ok := next.Sessions[id]; ok {
		t.Error("a session that never started must leave no row, terminal or otherwise")
	}
}
