package termvt

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitTimeout bounds how long the event-waiting helpers block before failing.
const waitTimeout = 3 * time.Second

// waitFor reads events until pred matches or waitTimeout elapses.
func waitFor(t *testing.T, ch <-chan Event, pred func(Event) bool) {
	t.Helper()
	deadline := time.After(waitTimeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before match")
			}
			if pred(ev) {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for event")
		}
	}
}

func outputContains(ev Event, sub string) bool {
	return ev.Kind == EventOutput && strings.Contains(string(ev.Data), sub)
}

func controlMatch(ev Event, kind string, code int, dataSub string) bool {
	return ev.Kind == EventControl && ev.Ctl.Kind == kind &&
		ev.Ctl.Code == code && strings.Contains(ev.Ctl.Data, dataSub)
}

func TestSessionEchoesInput(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	if err := s.WriteInput([]byte("ping-123\n")); err != nil {
		t.Fatalf("WriteInput: %v", err)
	}
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "ping-123") })
}

func TestSessionCapturesOSC9(t *testing.T) {
	// printf emits an OSC 9 desktop-notification sequence; the session must
	// surface it as a Control event rather than raw bytes. The leading sleep
	// gives the test time to Subscribe before the sequence fires — mainLoop
	// only fans Control events out to subscribers live at the time of the
	// write, it does not replay them for late subscribers, so without this
	// margin a slow-scheduled test goroutine (e.g. under -race) can lose the
	// race against the unthrottled child process and miss the event entirely.
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `sleep 0.2; printf '\033]9;hello-notif\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "osc", 9, "hello-notif") })
}

func TestSessionCapturesOSC133Prompt(t *testing.T) {
	// See TestSessionCapturesOSC9 for why the leading sleep is required.
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `sleep 0.2; printf '\033]133;A\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "prompt", 133, "A") })
}

// TestSessionHonorsSpecDir pins the contract that Spec.Dir becomes the
// spawned child's initial cwd. Regression pin against the historic
// TODO(B1) gap: SpawnFrame → termvt.Spec dropped startDir silently because
// Spec had no Dir field, so host-launched agents inherited daemon cwd
// (typically /home/<user>) instead of the driver-resolved project dir.
func TestSessionHonorsSpecDir(t *testing.T) {
	dir := t.TempDir()
	// resolve any /var → /private/var (macOS) symlinks so the reported
	// cwd from pwd matches t.TempDir()'s canonical form.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	// bash pwd on the pty; sleep gives the subscriber time to attach.
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c", `sleep 0.2; pwd; sleep 0.3`},
		Dir:  dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, dir) })
}

func TestSessionCapturesTitle(t *testing.T) {
	// See TestSessionCapturesOSC9 for why the leading sleep is required.
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `sleep 0.2; printf '\033]0;my-title\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "title", 0, "my-title") })
}

func TestSessionReattachSnapshotFirst(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	select {
	case ev := <-ch:
		if ev.Kind != EventOutput {
			t.Fatalf("first event is not an output snapshot: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot event")
	}
}

// TestAttachAtGeometryContract_ReflowsRetainedSoftWrap is the wired x/vt
// contract behind the fake-emulator actor tests. Output is first parsed at a
// narrow geometry, including a row that moves into history, then attached at
// a wider geometry. The first event must be one semantic snapshot without the
// old physical-row newline.
func TestAttachAtGeometryContract_ReflowsRetainedSoftWrap(t *testing.T) {
	em := emulatorFor(5, 2)
	pty := newFakePTY()
	s := NewSessionWithDeps(em, pty, nil, 5, 2)
	defer func() { _ = s.Close() }()

	pty.in <- []byte("abcdefghijk")
	deadline := time.Now().Add(time.Second)
	for {
		physical := strings.ReplaceAll(string(s.Snapshot()), "\n", "")
		if strings.Contains(physical, "fghijk") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("narrow emulator never observed output: %q", s.Snapshot())
		}
		time.Sleep(time.Millisecond)
	}

	_, ch, err := s.AttachAtGeometry(10, 2)
	if err != nil {
		t.Fatalf("AttachAtGeometry: %v", err)
	}
	seed := waitNext(t, ch, time.Second)
	if seed.Kind != EventOutput {
		t.Fatalf("first attach event = %+v, want EventOutput", seed)
	}
	if got := string(seed.Data); !strings.Contains(got, "abcdefghijk") {
		t.Fatalf("reattach snapshot = %q, want reflowed logical text", got)
	}
}

func TestSessionResize(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if cols, rows := s.Size(); cols != 100 || rows != 30 {
		t.Fatalf("resize not applied: got %dx%d", cols, rows)
	}
}

func TestSessionEmitsExitOnClose(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "5"}})
	if err != nil {
		t.Fatal(err)
	}
	_, ch := s.SubscribeCurrent()
	_ = s.Close()
	waitFor(t, ch, func(ev Event) bool { return ev.Kind == EventExit })
}

func TestSessionDefaultSize(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}}) // no Cols/Rows → defaults
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if cols, rows := s.Size(); cols != 80 || rows != 24 {
		t.Fatalf("default size = %dx%d, want 80x24", cols, rows)
	}
}

func TestNewSessionEmptyArgv(t *testing.T) {
	if _, err := NewSession(Spec{}); err == nil {
		t.Fatal("expected error for empty argv")
	}
}

// TestNormalizeSizeClamp pins the dimension guard: non-positive sizes floor to
// the defaults and oversized ones are capped at MaxDim — so a client cannot
// overflow the uint16 pty winsize (65536 → 0) or drive the VT grid toward OOM.
func TestNormalizeSizeClamp(t *testing.T) {
	cases := []struct{ inC, inR, wantC, wantR int }{
		{0, 0, 80, 24},
		{-5, -1, 80, 24},
		{100, 30, 100, 30},
		{100000, 100000, MaxDim, MaxDim}, // OOM-sized grid → clamped
		{65536, 1, MaxDim, 1},            // would wrap to 0 cols without the cap
	}
	for _, c := range cases {
		if gotC, gotR := normalizeSize(c.inC, c.inR); gotC != c.wantC || gotR != c.wantR {
			t.Errorf("normalizeSize(%d,%d) = %dx%d, want %dx%d",
				c.inC, c.inR, gotC, gotR, c.wantC, c.wantR)
		}
	}
}

// TestClampDim pins the per-dimension clamp helper directly, exercising the def
// parameter (which normalizeSize hard-codes to 80/24) and the exact MaxDim edge.
func TestClampDim(t *testing.T) {
	cases := []struct {
		name string
		d    int
		def  int
		want int
	}{
		{"zero floors to def", 0, 80, 80},
		{"negative floors to def", -10, 24, 24},
		{"in range passes through", 100, 80, 100},
		{"exactly MaxDim passes through", MaxDim, 80, MaxDim},
		{"above MaxDim caps", MaxDim + 1, 80, MaxDim},
		{"far above MaxDim caps", 100000, 24, MaxDim},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampDim(c.d, c.def); got != c.want {
				t.Errorf("clampDim(%d, %d) = %d, want %d", c.d, c.def, got, c.want)
			}
		})
	}
}

// TestSessionExitCodeNeverBlocksDuringCSIReportMode reproduces a deadlock:
// the shell emits CSI Report Mode (DECRQM, "\033[?1$p"), the VT emulator's
// handleRequestMode writes the reply to its internal io.Pipe synchronously,
// and nothing drains the read end — em.Write blocks forever, holding s.mu,
// and every ExitCode call (which the runtime dispatch loop fires every tick
// via aliveness probe) hangs in turn. Bug surfaces as the whole daemon's IPC
// freezing under any tty client that ever queries terminal modes.
//
// Skipped pre-fix; the responseLoop drain in step 3 makes it pass. Step 4's
// atomic ExitCode makes ExitCode robust even if mainLoop is busy.
func TestSessionExitCodeNeverBlocksDuringCSIReportMode(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c",
		`printf '\033[?1$p'; sleep 0.2; exit 0`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			_, _ = s.ExitCode()
			time.Sleep(10 * time.Millisecond)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExitCode blocked — readLoop deadlock on CSI Report Mode")
	}
}

// TestSessionScrollbackSurvivesLateSubscribe pins the late-join contract:
// a fresh subscriber that attaches after the visible grid has scrolled past
// the first lines must receive those lines in the opaque reattach snapshot.
// Without server-side scrollback the late subscriber would only see the
// trailing 24 rows (visible grid) and the early "line 1" would be lost —
// exactly the regression this feature exists to prevent.
func TestSessionScrollbackSurvivesLateSubscribe(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`for i in $(seq 1 200); do printf "scrollback-row-%d\n" $i; done; sleep 2`},
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	// First subscriber drives the timing: drain until we see the last row,
	// which proves the emulator has processed all 200 lines and the early
	// ones have necessarily scrolled past the 24-row visible grid into
	// scrollback.
	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "scrollback-row-200") })

	// Now a late subscriber attaches. Its single seed carries semantic history
	// plus the current visible grid.
	_, late := s.SubscribeCurrent()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	if !strings.Contains(string(first.Data), "scrollback-row-1\r\n") {
		t.Fatalf("late subscriber snapshot missing row 1; got first 200 bytes: %q",
			truncate(string(first.Data), 200))
	}
}

// TestSessionScrollbackLinesCapHonored verifies that Spec.ScrollbackLines
// bounds the buffer: with cap=5 the single snapshot contains at most five
// scrolled-off rows plus the 24-row visible grid.
func TestSessionScrollbackLinesCapHonored(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`for i in $(seq 1 200); do printf "row-%d\n" $i; done; sleep 2`},
		Cols: 80, Rows: 24,
		ScrollbackLines: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "row-200") })

	_, late := s.SubscribeCurrent()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	payload := string(first.Data)
	if got := strings.Count(payload, "\n"); got > 28 {
		t.Fatalf("snapshot has %d row separators, want at most 28 for cap=5 + 24-row grid; payload: %q",
			got, truncate(payload, 400))
	}
	// The cap-drops-old invariant: row-1 is the very first emitted line and
	// must have fallen off the scrollback long ago. If it's present the cap
	// is not being applied.
	if strings.Contains(payload, "row-1\r\n") {
		t.Fatalf("scrollback cap=5 leaked row-1 (oldest emitted line, cap should have dropped it);"+
			" payload: %q", truncate(payload, 400))
	}
}

// TestSessionScrollbackOmittedInAltScreen pins the alt-screen contract:
// when the program is on the alternate screen (DECSET 1049), nothing has
// spilled to scrollback yet — primary screen scrollback is empty — and the
// seed must elide the scrollback frame entirely. The single seed frame the
// late subscriber receives carries the current alt-screen render.
func TestSessionScrollbackOmittedInAltScreen(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`printf '\033[?1049h'; printf 'alt-marker\n'; sleep 2`},
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "alt-marker") })

	_, late := s.SubscribeCurrent()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	// In alt-screen mode the scrollback buffer is empty, so the very first
	// seed frame must already be the screen render (it contains alt-marker).
	if !strings.Contains(string(first.Data), "alt-marker") {
		t.Fatalf("first frame should be screen render with alt-marker; got: %q",
			truncate(string(first.Data), 400))
	}
}

// truncate returns up to n bytes of s, with an ellipsis when longer. Used by
// scrollback-test failure messages to keep error logs bounded when a multi-KB
// frame is involved.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// TestSessionDisconnectsSlowSubscriber verifies that a subscriber which does not
// drain its channel is disconnected (channel closed) once its buffer overflows,
// rather than having events silently dropped.
//
// This drives the actor through fake deps (see session_actor_test.go) instead
// of a real pty and a 20MB /dev/zero flood: the disconnect logic lives in
// fanout() and only cares about channel occupancy, not where chunks come
// from, and a real subprocess made the overflow depend on wall-clock pty
// throughput — on a CPU-starved CI runner the flood could take far longer
// than the test's fixed sleep/deadline to fill the buffer, so the test
// wrongly reported the subscriber as never disconnected.
func TestSessionDisconnectsSlowSubscriber(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.SubscribeCurrent()

	// Feed far more chunks than subBuffer can hold while nothing drains ch;
	// each chunk becomes one Output event in fanout.
	for i := 0; i < subBuffer+50; i++ {
		pty.in <- []byte("x")
	}

	deadline := time.After(waitTimeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // disconnected as expected
			}
		case <-deadline:
			t.Fatal("slow subscriber was not disconnected on overflow")
		}
	}
}
