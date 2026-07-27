package driver

import (
	"strings"
	"testing"
	"time"
)

// TestLastUserPromptViewWiring locks the per-driver View.LastUserPrompt
// contract consumed by the web terminal-header preview: claude / codex /
// gemini surface CommonState.LastPrompt, while drivers without a prompt
// concept (grok, generic, shell) never populate the field — even when the
// embedded CommonState happens to carry a stale LastPrompt value.
func TestLastUserPromptViewWiring(t *testing.T) {
	const prompt = "fix the flaky test in pkg/foo"

	t.Run("claude surfaces LastPrompt", func(t *testing.T) {
		d, cs, _ := newClaude(t)
		cs.LastPrompt = prompt
		if got := d.view(cs).LastUserPrompt; got != prompt {
			t.Errorf("LastUserPrompt = %q, want %q", got, prompt)
		}
	})

	t.Run("codex surfaces LastPrompt", func(t *testing.T) {
		d, cs, _ := newCodex(t)
		cs.LastPrompt = prompt
		if got := d.view(cs).LastUserPrompt; got != prompt {
			t.Errorf("LastUserPrompt = %q, want %q", got, prompt)
		}
	})

	t.Run("gemini surfaces LastPrompt", func(t *testing.T) {
		d, gs, _ := newGemini(t)
		gs.LastPrompt = prompt
		if got := d.view(gs).LastUserPrompt; got != prompt {
			t.Errorf("LastUserPrompt = %q, want %q", got, prompt)
		}
	})

	t.Run("grok never surfaces a prompt", func(t *testing.T) {
		d := NewGrokDriver("")
		gs := d.NewState(time.Now()).(GrokState)
		gs.LastPrompt = prompt
		if got := d.View(gs).LastUserPrompt; got != "" {
			t.Errorf("LastUserPrompt = %q, want empty", got)
		}
	})

	t.Run("generic never surfaces a prompt", func(t *testing.T) {
		d := NewGenericDriver("bash", "bash", time.Second)
		gs := d.NewState(time.Now()).(GenericState)
		gs.LastPrompt = prompt
		if got := d.View(gs).LastUserPrompt; got != "" {
			t.Errorf("LastUserPrompt = %q, want empty", got)
		}
	})

	t.Run("shell never surfaces a prompt", func(t *testing.T) {
		d := NewShellDriver("zsh", "zsh", time.Second)
		ss := d.NewState(time.Now()).(ShellState)
		ss.LastPrompt = prompt
		if got := d.View(ss).LastUserPrompt; got != "" {
			t.Errorf("LastUserPrompt = %q, want empty", got)
		}
	})
}

// TestLastUserPromptPreview locks the rune-safe bounding applied before the
// prompt is embedded in view broadcasts.
func TestLastUserPromptPreview(t *testing.T) {
	t.Run("short text passes through trimmed", func(t *testing.T) {
		if got := lastUserPromptPreview("  hello\n"); got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("long text is capped at 500 runes with ellipsis", func(t *testing.T) {
		in := strings.Repeat("a", 600)
		got := lastUserPromptPreview(in)
		want := strings.Repeat("a", 500) + "…"
		if got != want {
			t.Errorf("len(got) = %d, want 500 runes + ellipsis", len([]rune(got)))
		}
	})

	t.Run("multibyte text is not split mid-rune", func(t *testing.T) {
		in := strings.Repeat("あ", 600)
		got := lastUserPromptPreview(in)
		want := strings.Repeat("あ", 500) + "…"
		if got != want {
			t.Errorf("multibyte truncation mismatch: got %d runes", len([]rune(got)))
		}
	})

	t.Run("internal newlines are preserved for multi-line preview", func(t *testing.T) {
		if got := lastUserPromptPreview("line1\nline2"); got != "line1\nline2" {
			t.Errorf("got %q, want newlines preserved", got)
		}
	})
}
