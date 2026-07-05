package runtime

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/driver/vt"
	"github.com/takezoh/agent-reactor/client/state"
)

// fakeFrameTap records Start/Stop calls for assertions.
type fakeFrameTap struct {
	mu      sync.Mutex
	started []string
	stopped []string
}

func (f *fakeFrameTap) Start(_ context.Context, frameID string) (<-chan []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, frameID)
	ch := make(chan []byte)
	return ch, nil
}

func (f *fakeFrameTap) Stop(frameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, frameID)
	return nil
}

func (f *fakeFrameTap) startedSorted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.started...)
	sort.Strings(out)
	return out
}

func TestReadTapEmitsOscEvents(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	ch <- []byte("\x1b]9;hello world\x07")
	close(ch)

	var events []state.Event
	sink := eventSinkFunc(func(e state.Event) { events = append(events, e) })

	readTap(context.Background(), frameID, "%1", ch, sink)

	var gotOsc bool
	for _, ev := range events {
		if o, ok := ev.(state.EvFrameOsc); ok {
			gotOsc = true
			if o.Cmd != 9 {
				t.Errorf("Cmd = %d, want 9", o.Cmd)
			}
			if o.Title != "hello world" {
				t.Errorf("Title = %q, want %q", o.Title, "hello world")
			}
		}
	}
	if !gotOsc {
		t.Error("expected EvFrameOsc event")
	}
}

func TestReadTapEmitsRepeatedPromptEvents(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	ch <- []byte("\x1b]133;C\x07")
	ch <- []byte("\x1b]133;D;0\x07")
	ch <- []byte("\x1b]133;C\x07")
	ch <- []byte("\x1b]133;D;42\x07")
	close(ch)

	var events []state.Event
	sink := eventSinkFunc(func(e state.Event) { events = append(events, e) })

	readTap(context.Background(), frameID, "%1", ch, sink)

	var prompts []state.EvFramePrompt
	for _, ev := range events {
		if p, ok := ev.(state.EvFramePrompt); ok {
			prompts = append(prompts, p)
		}
	}
	if len(prompts) != 4 {
		t.Fatalf("prompt events = %d, want 4", len(prompts))
	}
	if prompts[0].Phase != state.PromptPhaseCommand {
		t.Errorf("prompts[0].Phase = %v, want Command", prompts[0].Phase)
	}
	if prompts[1].Phase != state.PromptPhaseComplete {
		t.Errorf("prompts[1].Phase = %v, want Complete", prompts[1].Phase)
	}
	if prompts[1].ExitCode == nil || *prompts[1].ExitCode != 0 {
		t.Errorf("prompts[1].ExitCode = %v, want 0", prompts[1].ExitCode)
	}
	if prompts[2].Phase != state.PromptPhaseCommand {
		t.Errorf("prompts[2].Phase = %v, want Command", prompts[2].Phase)
	}
	if prompts[3].Phase != state.PromptPhaseComplete {
		t.Errorf("prompts[3].Phase = %v, want Complete", prompts[3].Phase)
	}
	if prompts[3].ExitCode == nil || *prompts[3].ExitCode != 42 {
		t.Errorf("prompts[3].ExitCode = %v, want 42", prompts[3].ExitCode)
	}
}

func TestParseOscNotification_OSC9(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 9, Payload: "  hello  "})
	if title != "hello" || body != "" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestParseOscNotification_OSC777(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 777, Payload: "notify;My Title;My Body"})
	if title != "My Title" || body != "My Body" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestParseOscNotification_OSC99(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 99, Payload: "i=1:d=Alert:p=Something happened"})
	if title != "Alert" || body != "Something happened" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestReadTapCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan []byte)

	done := make(chan struct{})
	go func() {
		readTap(ctx, "f1", "%1", ch, eventSinkFunc(func(state.Event) {}))
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("readTap did not exit after context cancel")
	}
}

// Regression: the frame tap VT emulator runs at 1x1 dimensions and the
// upstream charmbracelet/x/vt library used to panic with "index out of
// range" when scroll/cursor sequences (e.g. CSI M / DECRC / ESC M) drove
// Buffer.InsertLineArea past the buffer bounds. The upstream bounds fixes
// eliminated this deterministic panic. feedSafe remains only as the outer
// daemon-crash guard
// for unknown emulator defects; known scroll sequences must not rely on it.
func TestFrameTapTerminal_ProcessesOscAfterScrollSequences(t *testing.T) {
	frameID := state.FrameID("f1")
	var events []state.Event
	sink := eventSinkFunc(func(e state.Event) { events = append(events, e) })
	term := newFrameTapTerminal(frameID, sink)
	// Chunk 1: ESC sequences that used to crash the 1x1 emulator.
	if err := term.Feed([]byte("\x1bM\x1bM\x1bM")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	// Chunk 2: a well-formed OSC notification must still come through.
	if err := term.Feed([]byte("\x1b]9;ping\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}

	var gotOSC bool
	for _, ev := range events {
		if o, ok := ev.(state.EvFrameOsc); ok && o.Cmd == 9 && o.Title == "ping" {
			gotOSC = true
		}
	}
	if !gotOSC {
		t.Fatalf("expected OSC 9 event, got events=%+v", events)
	}
}

// readTap must survive a chunk full of scroll sequences at 1x1 and still
// deliver the OSC payload from the following chunk. Before the upstream
// bounds fixes this was the daemon-killing path: the emulator panicked in
// the per-frame goroutine and took down the whole process.
func TestReadTap_SurvivesScrollSequences(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	// 1x1 emulator used to panic on these.
	ch <- []byte("\x1bM\x1bM\x1bM")
	// Followed by a legitimate OSC payload that must arrive.
	ch <- []byte("\x1b]9;after-panic\x07")
	close(ch)

	var events []state.Event
	sink := eventSinkFunc(func(e state.Event) { events = append(events, e) })
	done := make(chan struct{})
	go func() {
		readTap(context.Background(), frameID, "%1", ch, sink)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("readTap did not return after channel close (likely killed by panic instead of recovering)")
	}

	var gotOSC bool
	for _, ev := range events {
		if o, ok := ev.(state.EvFrameOsc); ok && o.Title == "after-panic" {
			gotOSC = true
		}
	}
	if !gotOSC {
		t.Fatalf("expected post-panic OSC event, got %+v", events)
	}
}

func TestStartRestoredTaps_StartsOnlyRootFrames(t *testing.T) {
	tap := &fakeFrameTap{}
	r := New(Config{
		TickInterval: 10 * time.Second,
		Tap:          tap,
	})
	// frame_a is root of session s1, frame_b is root of session s2.
	// frame_c is a non-root child frame and must NOT get a tap.
	r.state.Sessions[state.SessionID("s1")] = state.Session{
		ID: "s1",
		Frames: []state.SessionFrame{
			{ID: state.FrameID("frame_a")},
			{ID: state.FrameID("frame_c")},
		},
	}
	r.state.Sessions[state.SessionID("s2")] = state.Session{
		ID:     "s2",
		Frames: []state.SessionFrame{{ID: state.FrameID("frame_b")}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.taps = newTapManager(ctx, tap)

	r.startRestoredTaps()

	got := tap.startedSorted()
	want := []string{"frame_a", "frame_b"}
	if len(got) != len(want) {
		t.Fatalf("started frames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("started[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStartRestoredTaps_NoTapsWhenNilManager(t *testing.T) {
	tap := &fakeFrameTap{}
	r := New(Config{
		TickInterval: 10 * time.Second,
		Tap:          tap,
	})
	r.state.Sessions[state.SessionID("s1")] = state.Session{
		ID:     "s1",
		Frames: []state.SessionFrame{{ID: state.FrameID("frame_a")}},
	}
	// r.taps left nil (Run not started)

	r.startRestoredTaps() // must not panic

	if len(tap.started) != 0 {
		t.Errorf("tap unexpectedly started while taps manager was nil: %v", tap.started)
	}
}

func TestStartTapsForRestoredFrames_DispatchesViaEventLoop(t *testing.T) {
	tap := &fakeFrameTap{}
	r := New(Config{
		TickInterval: 10 * time.Millisecond,
		Tap:          tap,
		Backend:      noopBackend{},
	})
	r.state.Sessions[state.SessionID("s1")] = state.Session{
		ID:     "s1",
		Frames: []state.SessionFrame{{ID: state.FrameID("frame_a")}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = r.Run(ctx) }()

	r.StartTapsForRestoredFrames()

	deadline := time.Now().Add(1 * time.Second)
	for len(tap.startedSorted()) != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("tap never started; got %v", tap.startedSorted())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := tap.startedSorted(); got[0] != "frame_a" {
		t.Errorf("started = %v, want [frame_a]", got)
	}
}
