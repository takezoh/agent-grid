package termvt

import (
	"strings"
	"testing"
	"time"
)

// TestSession_StaleScrollRegionAfterResizeDoesNotPanic is the end-to-end
// regression for the production crash in
// the child TUI emits a DECSTBM computed from a stale size while a browser re-fit shrinks the
// session, then a reverse index at the top of the (now out-of-bounds) scroll
// region drove ScrollDown → InsertLine → Buffer.InsertLineArea into an index
// out of range, killing the whole server process.
//
// We drive a Session with the real *vt.Emulator so the chunk flows through
// the same processChunk path that crashed. The actor serializes chunks and
// Resize on one goroutine, so feeding the stale-margin chunk after Resize
// reproduces the exact interleaving. Without the upstream bounds fixes this
// panics (the actor has no recover, by design); with them the session keeps
// serving snapshots.
func TestSession_StaleScrollRegionAfterResizeDoesNotPanic(t *testing.T) {
	em := emulatorFor(80, 64)
	pty := newFakePTY()
	s := NewSessionWithDeps(em, pty, nil, 80, 64)
	defer func() { _ = s.Close() }()

	// Child sets a full-height scroll region while the session is 80x64.
	pty.in <- []byte("\x1b[1;64r")
	awaitApplied(t, s, pty, "A")

	// Browser re-fit shrinks the session to 80x63.
	if err := s.Resize(80, 63); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	// Child still believes the screen is 64 rows tall: stale DECSTBM, then
	// reverse indexes at the top of the region — the crash sequence.
	pty.in <- []byte("\x1b[1;64r\x1b[1;1H\x1bM\x1bM\x1bM")
	awaitApplied(t, s, pty, "B")
}

// awaitApplied feeds a marker through the pty and waits until it shows up in
// a snapshot, proving the actor survived and applied all chunks queued
// before it.
func awaitApplied(t *testing.T, s *Session, pty *fakePTY, marker string) {
	t.Helper()
	pty.in <- []byte("\x1b[1;1H" + marker)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(string(s.Snapshot()), marker) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("marker %q never appeared in snapshot — actor likely dead", marker)
}
