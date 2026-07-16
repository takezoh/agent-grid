package termvt

import (
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// fakeEmulator drives session_actor_test without a real vt grid. Its Write
// captures bytes; Read serves whatever Replies returns; snapshot methods
// return canned state; OSC handlers and Callbacks are stored so a test can
// invoke them directly (e.g. firing OSC 9 during a controlled chunk).
//
// All hooks default to harmless behaviour, so tests only set the ones they
// care about.
type fakeEmulator struct {
	mu sync.Mutex

	WriteHook    func(p []byte) // called inside Write while no lock is held
	ResizeHook   func()
	ReattachHook func()
	RenderOut    string

	ReattachOut []byte
	ReattachErr error
	resizeCalls [][2]int

	written []byte
	closed  atomic.Bool

	// Replies bytes Read should emit. Drained as Read is called. EOF when
	// empty AND Close has been called.
	replies      chan []byte
	closeReplies sync.Once

	callbacks vt.Callbacks
	osc       map[int]vt.OscHandler
}

func newFakeEmulator() *fakeEmulator {
	return &fakeEmulator{
		replies: make(chan []byte, 8),
		osc:     map[int]vt.OscHandler{},
	}
}

func (e *fakeEmulator) Write(p []byte) (int, error) {
	if e.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	hook := e.WriteHook
	if hook != nil {
		hook(p)
	}
	e.mu.Lock()
	e.written = append(e.written, p...)
	e.mu.Unlock()
	return len(p), nil
}

func (e *fakeEmulator) Read(p []byte) (int, error) {
	b, ok := <-e.replies
	if !ok {
		return 0, io.EOF
	}
	return copy(p, b), nil
}

func (e *fakeEmulator) Render() string { return e.RenderOut }
func (e *fakeEmulator) Resize(cols, rows int) {
	if e.ResizeHook != nil {
		e.ResizeHook()
	}
	e.mu.Lock()
	e.resizeCalls = append(e.resizeCalls, [2]int{cols, rows})
	e.mu.Unlock()
}
func (e *fakeEmulator) SetCallbacks(cb vt.Callbacks)              { e.callbacks = cb }
func (e *fakeEmulator) RegisterOscHandler(c int, h vt.OscHandler) { e.osc[c] = h }
func (e *fakeEmulator) SetScrollbackSize(_ int)                   {}
func (e *fakeEmulator) ReattachSnapshot() ([]byte, error) {
	if e.ReattachHook != nil {
		e.ReattachHook()
	}
	return e.ReattachOut, e.ReattachErr
}
func (e *fakeEmulator) CloseInputPipe() error {
	if e.closed.Swap(true) {
		return nil
	}
	e.closeReplies.Do(func() { close(e.replies) })
	return nil
}

// fakePTY is a pty stand-in: Read draws chunks the test feeds via `in`,
// Write discards (no test currently asserts pty writes), Close unblocks a
// pending Read with io.EOF.
type fakePTY struct {
	in chan []byte
	mu sync.Mutex

	setSizeCalls [][2]int
	setSizeErr   error
	setSizeHook  func()

	closeOnce sync.Once
	closed    chan struct{}
}

func newFakePTY() *fakePTY {
	return &fakePTY{
		in:     make(chan []byte, 8),
		closed: make(chan struct{}),
	}
}

func (p *fakePTY) Read(buf []byte) (int, error) {
	select {
	case b, ok := <-p.in:
		if !ok {
			return 0, io.EOF
		}
		return copy(buf, b), nil
	case <-p.closed:
		return 0, io.EOF
	}
}

func (p *fakePTY) Write(b []byte) (int, error) {
	select {
	case <-p.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	return len(b), nil
}

func (p *fakePTY) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func (p *fakePTY) SetSize(cols, rows int) error {
	if p.setSizeHook != nil {
		p.setSizeHook()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setSizeCalls = append(p.setSizeCalls, [2]int{cols, rows})
	return p.setSizeErr
}

// helper: build a session against fakes, ready for use in a test.
func newFakeSession(em Emulator, pty PTY) *Session {
	return NewSessionWithDeps(em, pty, nil, 80, 24)
}

// TestActor_SubscribeReceivesSnapshotThenChunk pins the
// snapshot-before-live-chunk ordering inside the actor: Subscribe runs
// between chunks (cmdCh is processed serially with chunkCh), so the very
// first event must be the reattach snapshot and the next must be the live
// output from a chunk fed AFTER Subscribe returned.
func TestActor_SubscribeReceivesSnapshotThenChunk(t *testing.T) {
	em := newFakeEmulator()
	em.ReattachOut = []byte("SNAPSHOT\x1b[1;1H\x1b[K")
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	pty.in <- []byte("CHUNK")

	// Seed: Render() bytes + trailing CUP pinning the cursor to (0,0) +
	// EL to wipe stale cells past the cursor on the input row.
	first := waitNext(t, ch, time.Second)
	if first.Kind != EventOutput || string(first.Data) != "SNAPSHOT\x1b[1;1H\x1b[K" {
		t.Fatalf("first event = %+v, want EventOutput SNAPSHOT\\x1b[1;1H\\x1b[K", first)
	}
	second := waitNext(t, ch, time.Second)
	if second.Kind != EventOutput || string(second.Data) != "CHUNK" {
		t.Fatalf("second event = %+v, want EventOutput CHUNK", second)
	}
}

func TestActor_AttachAtGeometryCommitsSizeBeforeSnapshot(t *testing.T) {
	em := newFakeEmulator()
	em.ReattachOut = []byte("REFLOWED-SNAPSHOT")
	pty := newFakePTY()
	var order []string
	pty.setSizeHook = func() { order = append(order, "pty") }
	em.ResizeHook = func() { order = append(order, "emulator") }
	em.ReattachHook = func() { order = append(order, "snapshot") }
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	id, ch, err := s.AttachAtGeometry(120, 40)
	if err != nil {
		t.Fatalf("AttachAtGeometry: %v", err)
	}
	if id == 0 {
		t.Fatal("AttachAtGeometry returned shutdown sentinel")
	}
	if got := waitNext(t, ch, time.Second); got.Kind != EventOutput || string(got.Data) != "REFLOWED-SNAPSHOT" {
		t.Fatalf("seed = %+v, want opaque reattach snapshot", got)
	}
	if got := pty.setSizeCalls; len(got) != 1 || got[0] != [2]int{120, 40} {
		t.Fatalf("PTY SetSize calls = %v, want [[120 40]]", got)
	}
	if got := em.resizeCalls; len(got) != 1 || got[0] != [2]int{120, 40} {
		t.Fatalf("emulator Resize calls = %v, want [[120 40]]", got)
	}
	if got := strings.Join(order, ","); got != "pty,emulator,snapshot" {
		t.Fatalf("attach order = %q, want pty,emulator,snapshot", got)
	}
	if cols, rows := s.Size(); cols != 120 || rows != 40 {
		t.Fatalf("session size = %dx%d, want 120x40", cols, rows)
	}
}

func TestActor_AttachPTYFailureLeavesStateUnchanged(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	pty.setSizeErr = io.ErrClosedPipe
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	id, ch, err := s.AttachAtGeometry(120, 40)
	if err == nil {
		t.Fatal("AttachAtGeometry error = nil, want PTY SetSize failure")
	}
	if id != 0 {
		t.Fatalf("AttachAtGeometry id = %d, want 0", id)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("failed attach delivered an event")
		}
	case <-time.After(time.Second):
		t.Fatal("failed attach channel did not close")
	}
	if len(em.resizeCalls) != 0 {
		t.Fatalf("emulator resized after PTY failure: %v", em.resizeCalls)
	}
	if cols, rows := s.Size(); cols != 80 || rows != 24 {
		t.Fatalf("session size after failed attach = %dx%d, want 80x24", cols, rows)
	}
}

func TestActor_SnapshotFailurePublishesNothingAndTerminatesSession(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)

	wantErr := errors.New("invalid semantic state")
	em.ReattachErr = &vt.SnapshotError{Cause: wantErr}
	id, ch, err := s.AttachAtGeometry(120, 40)
	if !errors.Is(err, wantErr) {
		t.Fatalf("AttachAtGeometry error = %v, want %v", err, wantErr)
	}
	if id != 0 {
		t.Fatalf("AttachAtGeometry id = %d, want 0", id)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("failed snapshot published a seed event")
		}
	case <-time.After(time.Second):
		t.Fatal("failed snapshot channel did not close")
	}
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("snapshot failure did not terminate the unusable session")
	}
	lateID, late := s.SubscribeCurrent()
	if lateID != 0 {
		t.Fatalf("late subscriber id = %d, want shutdown sentinel", lateID)
	}
	if _, ok := <-late; ok {
		t.Fatal("late subscriber channel remained open after snapshot failure")
	}
}

func TestActor_SubscribeCurrentDoesNotResize(t *testing.T) {
	em := newFakeEmulator()
	em.ReattachOut = []byte("CURRENT")
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	if got := waitNext(t, ch, time.Second); string(got.Data) != "CURRENT" {
		t.Fatalf("seed = %q, want CURRENT", got.Data)
	}
	if len(pty.setSizeCalls) != 0 || len(em.resizeCalls) != 0 {
		t.Fatalf("SubscribeCurrent changed size: pty=%v emulator=%v", pty.setSizeCalls, em.resizeCalls)
	}
}

// TestActor_SubscribeForwardsOneOpaqueSnapshot pins the ownership boundary:
// termvt does not compose physical rows or cursor escapes. The VT owner emits
// one semantic ANSI snapshot and the actor forwards it unchanged.
func TestActor_SubscribeForwardsOneOpaqueSnapshot(t *testing.T) {
	em := newFakeEmulator()
	em.ReattachOut = []byte("old1old2SCREEN\x1b[1;1H\x1b[K")
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()

	first := waitNext(t, ch, time.Second)
	if first.Kind != EventOutput || string(first.Data) != "old1old2SCREEN\x1b[1;1H\x1b[K" {
		t.Fatalf("seed event = %+v, want opaque snapshot unchanged", first)
	}
}

// TestActor_SubscribeSeedPinsCursorWithCUP asserts that the second seed
// frame ends with a CUP escape that places xterm.js's cursor at the same
// (x, y) the emulator holds, immediately followed by EL (\x1b[K) which
// clears any stale cells past the cursor on the input row. Without CUP,
// Render() bytes leave the browser-side cursor at the bottom-right of the
// rendered grid and any subsequent shell echo gets painted at the wrong
// screen cell (the "session-switch input position is broken" regression).
// Without EL, the emulator's Render() faithfully re-paints any residue a
// claude-code-style "\r❯ "-only redraw left in the buffer (see seed-shape
// comment on subscribeCmd.run). CUP is 1-based; emulator coords are 0-based.
func TestActor_SubscribeSeedPinsCursorWithCUP(t *testing.T) {
	em := newFakeEmulator()
	em.ReattachOut = []byte("SCREEN\x1b[5;13H\x1b[K")
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()

	ev := waitNext(t, ch, time.Second)
	want := "SCREEN\x1b[5;13H\x1b[K"
	if ev.Kind != EventOutput || string(ev.Data) != want {
		t.Fatalf("seed event = %+v, want EventOutput %q", ev, want)
	}
}

// TestActor_SubscribeSeedClearsStaleInputTail is the end-to-end regression
// for the Web UI bug: when an agent CLI (e.g. claude code's prompt component)
// redraws the input row by writing only "\r❯ " — moving the cursor to col 0
// then overwriting cols 0..1 without an explicit EL — the emulator's
// underlying buffer keeps the previous prompt's tail at the cells past
// col 2. Render() at Subscribe faithfully re-emits that residue; without the
// trailing EL the user sees stale text on the input row immediately after a
// session switch, until the next repaint masks it.
//
// We drive a Session with the real *vt.Emulator (no fake mock) so we exercise
// the actual cell-tracking behavior, then replay the seed bytes into a
// freshly-spawned *vt.Emulator — exactly what xterm.js does client-side on
// the keyed remount. The fresh emulator's Render() must not contain the
// stale tail.
func TestActor_SubscribeSeedClearsStaleInputTail(t *testing.T) {
	em := emulatorFor(80, 24)
	pty := newFakePTY()
	s := NewSessionWithDeps(em, pty, nil, 80, 24)
	defer func() { _ = s.Close() }()

	const stale = "業確認"
	// One chunk so the emulator processes typing + incomplete clear atomically:
	// "❯ 業確認" (cursor advances past col 7) then "\r❯ " (cursor moves back
	// to col 0, rewrites the prompt indicator, ends at col 2). Cells 2..7
	// remain populated with the wide-char glyphs.
	pty.in <- []byte("❯ 業確認\r❯ ")
	// Wait until the actor has applied the chunk: the snapshot must contain
	// the stale tail. If not, the test's premise (incomplete clear leaves
	// residue in the buffer) is broken and the regression cannot trigger.
	deadline := time.Now().Add(time.Second)
	var snap string
	for time.Now().Before(deadline) {
		snap = string(s.Snapshot())
		if strings.Contains(snap, stale) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(snap, stale) {
		t.Fatalf("setup precondition not met: stale tail %q not in source snapshot %q", stale, snap)
	}

	_, ch := s.SubscribeCurrent()
	ev := waitNext(t, ch, time.Second)
	if ev.Kind != EventOutput {
		t.Fatalf("seed event kind = %v, want EventOutput", ev.Kind)
	}

	// Replay the seed bytes into a fresh emulator, mirroring xterm.js on the
	// client (keyed remount on session-id change).
	fresh := vt.NewEmulator(80, 24)
	if _, err := fresh.Write(ev.Data); err != nil {
		t.Fatalf("fresh emulator write: %v", err)
	}
	replayed := fresh.Render()
	if strings.Contains(replayed, stale) {
		t.Fatalf("EL did not wipe stale input tail; replayed Render() = %q", replayed)
	}
	// Sanity check: the prompt indicator survives (only cells past the
	// cursor are EL-cleared). The trailing space after "❯" is collapsed
	// by Render()'s trailing-whitespace trim (renderLine accumulates
	// EmptyCell into `pending` and never flushes it at EOL), so we don't
	// assert on it here.
	if !strings.Contains(replayed, "❯") {
		t.Fatalf("prompt indicator clobbered; replayed Render() = %q", replayed)
	}
}

// TestActor_ExitCodeNeverGoesThroughMainLoop hammers ExitCode while mainLoop
// is parked in a deliberately slow em.Write (the fake's WriteHook sleeps).
// Every ExitCode must return within milliseconds — it lives on atomics and
// must not be routed through cmdCh. This is the structural invariant that
// keeps the runtime's dispatch goroutine responsive under any backend
// latency.
func TestActor_ExitCodeNeverGoesThroughMainLoop(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	// Buffered so the hook's send always lands even if mainLoop reaches
	// em.Write before this goroutine reaches the receive below — an
	// unbuffered channel here raced mainLoop's scheduling and could drop
	// the signal, deadlocking the test (see incident: race-job timeout).
	chunkArrived := make(chan struct{}, 1)
	em.WriteHook = func(_ []byte) {
		select {
		case chunkArrived <- struct{}{}:
		default:
		}
		time.Sleep(300 * time.Millisecond) // park mainLoop here
	}
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	pty.in <- []byte("slow")
	<-chunkArrived // mainLoop is now inside em.Write

	// 200 concurrent ExitCode calls, each must finish under 10ms.
	const N = 200
	var maxNs atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			_, _ = s.ExitCode()
			d := time.Since(start).Nanoseconds()
			for {
				cur := maxNs.Load()
				if d <= cur || maxNs.CompareAndSwap(cur, d) {
					break
				}
			}
		}()
	}
	wg.Wait()
	if maxNs.Load() > int64(10*time.Millisecond) {
		t.Fatalf("ExitCode max latency = %dns (> 10ms) — looks like it routed through mainLoop",
			maxNs.Load())
	}
}

// TestActor_SubscribeIDsAreUniqueAndNonZero pins the sentinel contract:
// mainLoop must allocate ids strictly greater than zero so the post-shutdown
// sentinel 0 is unambiguous. A regression here would make callers like
// TerminalRelay (which trusts the id) silently break re-subscribe when a
// real id collides with the shutdown value.
func TestActor_SubscribeIDsAreUniqueAndNonZero(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	seen := map[int]bool{}
	for i := 0; i < 5; i++ {
		id, _ := s.SubscribeCurrent()
		if id == 0 {
			t.Fatalf("live Subscribe #%d returned id 0 — collides with shutdown sentinel", i)
		}
		if seen[id] {
			t.Fatalf("Subscribe #%d returned duplicate id %d", i, id)
		}
		seen[id] = true
	}
}

// TestActor_SubscribeAfterShutdownReturnsClosedChannel pins the actor's
// post-exit contract: Subscribe (and any other RPC) must not deadlock if
// mainLoop has already exited. The pre-actor implementation leaked a
// goroutine waiting on events that would never come; the actor returns a
// closed channel so the caller's select sees EOF immediately.
func TestActor_SubscribeAfterShutdownReturnsClosedChannel(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)

	_ = s.Close()
	// Wait for mainLoop to actually exit (Close just nudges shutdown). Loop
	// up to a generous deadline; ExitCode flips when handleExit runs.
	deadline := time.After(2 * time.Second)
	for {
		if _, exited := s.ExitCode(); exited {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Session did not finish shutdown within 2s")
		case <-time.After(5 * time.Millisecond):
		}
	}

	id, ch := s.SubscribeCurrent()
	if id != 0 {
		t.Errorf("post-shutdown Subscribe id = %d, want 0 sentinel", id)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("post-shutdown Subscribe channel delivered an event")
		}
	case <-time.After(time.Second):
		t.Fatal("post-shutdown Subscribe channel did not close")
	}
}

// waitNext returns the next event on ch or fails the test.
func waitNext(t *testing.T, ch <-chan Event, d time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed unexpectedly")
		}
		return ev
	case <-time.After(d):
		t.Fatal("timeout waiting for event")
		return Event{}
	}
}
