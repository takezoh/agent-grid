package driver

import "testing"

// TestCardTitleChain locks the Card.Title chain implemented by
// resolveCardTitle and applied across every driver's view().
//
//	Title = aiTitle → collapseToSingleLine(summary) → ""
//
// LastPrompt is never a Title candidate (ADR-0079). Multi-line summaries
// used to drop to "" — they now collapse newlines into spaces because the
// Web Subtitle row that ADR-0076 originally fell back to has been retired
// (and the non-rendering Card.Subtitle consumers ADR-0079 §Decision 3
// cited — state/reduce_peer.go, tools/builtin.go — are gone as well).
func TestCardTitleChain(t *testing.T) {
	tests := []struct {
		name             string
		aiTitle, summary string
		want             string
	}{
		{"AI title wins over summary", "ai", "sum", "ai"},
		{"AI title alone", "ai", "", "ai"},
		{"summary promotes when AI title empty", "", "sum", "sum"},
		{"all empty → web client owns New Session placeholder", "", "", ""},
		{"multi-line summary collapses to a single-line Title", "", "line1\nline2", "line1 line2"},
		{"multi-line summary with CR/LF mix", "", "line1\r\nline2\r\nline3", "line1 line2 line3"},
		{"multi-line summary with blank lines is squeezed", "", "\nline1\n\n\nline2\n", "line1 line2"},
		{"AI title precedence holds even for multi-line summary", "ai", "line1\nline2", "ai"},
		{"summary whitespace is trimmed", "", "  spaced  ", "spaced"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCardTitle(tc.aiTitle, tc.summary)
			if got != tc.want {
				t.Errorf("Title = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestClaudeViewWiresTitleChain spot-checks that the Claude driver's view()
// actually routes through resolveCardTitle (regression guard for the
// 5 driver wirings — one driver-level test stands in for the family, the
// per-case matrix lives on resolveCardTitle directly).
func TestClaudeViewWiresTitleChain(t *testing.T) {
	d, cs, _ := newClaude(t)
	cs.Summary = "the summary"
	cs.LastPrompt = "the prompt"
	v := d.view(cs)
	if v.Card.Title != "the summary" {
		t.Errorf("Title = %q, want the summary (Summary should promote when AI title empty)", v.Card.Title)
	}
}
