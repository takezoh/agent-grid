package vt

import "testing"

// These regression tests pin the reason tap_manager must not depend on
// feedSafe for known VT streams: a 1×1 tap terminal used to panic inside the
// upstream emulator on scroll/cursor sequences that any full-screen TUI emits
// at startup (issues/2026-07-02-vt-emulator-insertlinearea-panic.md). With the
// upstream bounds fixes the raw Feed must survive them all.

func TestFeed1x1SurvivesScrollAndCursorSequences(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"reverse index at top", "\x1bM\x1bM\x1bM"},
		{"delete line", "\x1b[2M"},
		{"insert line", "\x1b[2L"},
		{"restore cursor without save", "\x1b8"},
		{"save then restore cursor", "\x1b7\x1b8"},
		{"stale DECSTBM then reverse index", "\x1b[1;24r\x1b[1;1H\x1bM\x1bM\x1bM\x1b[r\x1b[1;10r"},
		{"stale DECSLRM", "\x1b[?69h\x1b[1;80s\x1bM"},
		{"scroll down/up", "\x1b[3S\x1b[3T"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			term := New(1, 1)
			if err := term.Feed([]byte(tc.input)); err != nil {
				t.Fatalf("Feed: %v", err)
			}
		})
	}
}

// TestFeed1x1SurvivesCodexStartupSequence replays the shape of the escape
// stream a codex TUI emits at startup (captured in the issue's Phase 2
// probe): full-height DECSTBM, cursor home, a burst of reverse indexes, then
// margin resets. This is the exact stream feedSafe used to swallow chunks
// (and their OSC 133 / notification payloads) on.
func TestFeed1x1SurvivesCodexStartupSequence(t *testing.T) {
	term := New(1, 1)
	var fired bool
	term.OnPromptEvent = func(PromptEvent) { fired = true }

	input := "\x1b[1;24r\x1b[1;1H"
	for range 13 {
		input += "\x1bM"
	}
	input += "\x1b[r\x1b[1;13r"
	// The OSC arriving in the same chunk must survive and fire its callback.
	input += "\x1b]133;A\x07"

	if err := term.Feed([]byte(input)); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if !fired {
		t.Fatal("OSC 133 callback did not fire — chunk content was lost")
	}
}
