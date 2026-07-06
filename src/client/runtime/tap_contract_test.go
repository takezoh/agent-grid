package runtime

import (
	"context"
	"testing"

	"github.com/takezoh/agent-grid/client/driver/vt"
	"github.com/takezoh/agent-grid/client/state"
)

func TestPtyTapContract_RoutesOscAndPromptPerFrame(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	targetA := spawnFrame(t, backend, `sleep 0.5; printf '\033]0;ALPHA-TITLE\a'; printf '\033]133;B\a'; printf '\033]9;ALPHA-NOTE\a'; printf '\033]99;i=1:d=ALPHA-99:p=ALPHA-BODY\a'; sleep 1`)
	targetB := spawnFrame(t, backend, `sleep 0.5; printf '\033]2;BETA-TITLE\a'; printf '\033]133;D;7\a'; printf '\033]777;notify;BETA-777;BETA-BODY\a'; sleep 1`)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := newTapManager(ctx, NewPtyFrameTap(backend))
	t.Cleanup(mgr.stopAll)

	sink := &eventSink{}
	mgr.start(state.FrameID("frame-a"), targetA, sink)
	mgr.start(state.FrameID("frame-b"), targetB, sink)

	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-a" && osc.Cmd == 0 && osc.Title == "ALPHA-TITLE"
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		prompt, ok := ev.(state.EvFramePrompt)
		return ok && prompt.FrameID == "frame-a" && prompt.Phase == state.PromptPhaseInput
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-a" && osc.Cmd == 9 && osc.Title == "ALPHA-NOTE"
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-a" && osc.Cmd == 99 &&
			osc.Title == "ALPHA-99" && osc.Body == "ALPHA-BODY"
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-b" && osc.Cmd == 2 && osc.Title == "BETA-TITLE"
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		prompt, ok := ev.(state.EvFramePrompt)
		return ok && prompt.FrameID == "frame-b" &&
			prompt.Phase == state.PromptPhaseComplete &&
			prompt.ExitCode != nil && *prompt.ExitCode == 7
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-b" && osc.Cmd == 777 &&
			osc.Title == "BETA-777" && osc.Body == "BETA-BODY"
	})

	assertNoOscCrossTalk(t, sink.snapshot(), "frame-a", 0, "ALPHA-TITLE", "")
	assertNoOscCrossTalk(t, sink.snapshot(), "frame-a", 9, "ALPHA-NOTE", "")
	assertNoOscCrossTalk(t, sink.snapshot(), "frame-a", 99, "ALPHA-99", "ALPHA-BODY")
	assertNoOscCrossTalk(t, sink.snapshot(), "frame-b", 2, "BETA-TITLE", "")
	assertNoOscCrossTalk(t, sink.snapshot(), "frame-b", 777, "BETA-777", "BETA-BODY")
}

func TestPtyTapContract_RecoversAfterInvalidSequence(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	target := spawnFrame(t, backend, `sleep 0.5; printf '\033[1;24r\033[1;1H\033M\033M\033M\033M\033M\033M\033M\033M\033M\033M\033M\033M\033M\033[r\033[1;13r'; sleep 0.1; printf '\033]9;AFTER-INVALID\a'; printf '\033]133;B\a'; sleep 1`)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := newTapManager(ctx, NewPtyFrameTap(backend))
	t.Cleanup(mgr.stopAll)

	sink := &eventSink{}
	mgr.start(state.FrameID("frame-invalid"), target, sink)

	waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.FrameID == "frame-invalid" && osc.Cmd == 9 && osc.Title == "AFTER-INVALID"
	})
	waitForEvent(t, sink, func(ev state.Event) bool {
		prompt, ok := ev.(state.EvFramePrompt)
		return ok && prompt.FrameID == "frame-invalid" && prompt.Phase == state.PromptPhaseInput
	})
}

func TestFeedSafe_RecoversAndResetsTerminal(t *testing.T) {
	term := &panickingTapTerminal{}

	if err := feedSafe(term, "frame-test", "%1", []byte("bad")); err != nil {
		t.Fatalf("feedSafe first call: %v", err)
	}
	if term.resets != 1 {
		t.Fatalf("Reset count = %d, want 1", term.resets)
	}

	if err := feedSafe(term, "frame-test", "%1", []byte("good")); err != nil {
		t.Fatalf("feedSafe second call: %v", err)
	}
	if term.feeds != 2 {
		t.Fatalf("Feed count = %d, want 2", term.feeds)
	}
}

func FuzzParseOsc(f *testing.F) {
	f.Add(9, "  hello  ", uint8(vt.PromptPhaseStart))
	f.Add(99, "i=1:d=Alert:p=Body", uint8(vt.PromptPhaseInput))
	f.Add(777, "notify;Title;Body", uint8(vt.PromptPhaseCommand))
	f.Add(1234, "", uint8(vt.PromptPhaseComplete))

	f.Fuzz(func(t *testing.T, cmd int, payload string, phaseRaw uint8) {
		_, _ = parseOscNotification(vt.OscNotification{Cmd: cmd, Payload: payload})

		phase := vt.PromptPhase(phaseRaw)
		got := vtPromptPhase(phase)
		want := state.PromptPhaseNone
		switch phase {
		case vt.PromptPhaseNone:
			want = state.PromptPhaseNone
		case vt.PromptPhaseStart:
			want = state.PromptPhaseStart
		case vt.PromptPhaseInput:
			want = state.PromptPhaseInput
		case vt.PromptPhaseCommand:
			want = state.PromptPhaseCommand
		case vt.PromptPhaseComplete:
			want = state.PromptPhaseComplete
		}
		if got != want {
			t.Fatalf("vtPromptPhase(%v) = %v, want %v", phase, got, want)
		}
	})
}

type panickingTapTerminal struct {
	feeds  int
	resets int
}

func (t *panickingTapTerminal) Feed([]byte) error {
	t.feeds++
	if t.feeds == 1 {
		panic("synthetic panic")
	}
	return nil
}

func (t *panickingTapTerminal) Reset() { t.resets++ }

func assertNoOscCrossTalk(
	t *testing.T,
	events []state.Event,
	wantFrame state.FrameID,
	wantCmd int,
	wantTitle, wantBody string,
) {
	t.Helper()
	found := false
	for _, ev := range events {
		osc, ok := ev.(state.EvFrameOsc)
		if !ok {
			continue
		}
		if osc.Cmd != wantCmd || osc.Title != wantTitle || osc.Body != wantBody {
			continue
		}
		found = true
		if osc.FrameID != wantFrame {
			t.Fatalf("cross-talk: cmd=%d title=%q body=%q routed to %q, want %q",
				wantCmd, wantTitle, wantBody, osc.FrameID, wantFrame)
		}
	}
	if !found {
		t.Fatalf("expected OSC cmd=%d title=%q body=%q was not observed", wantCmd, wantTitle, wantBody)
	}
}
