package driver

import (
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-grid/client/state"
)

// CommonTags returns the shared UI tags (e.g. Git branch) for the driver.
func CommonTags(c CommonState) []state.Tag {
	var tags []state.Tag
	if t := BranchTag(c.BranchTag, c.BranchBG, c.BranchFG, c.BranchParentBranch); t.Text != "" {
		tags = append(tags, t)
	}
	return tags
}

// EventLogTab returns the "EVENTS" log tab if the session and log directory are known.
func EventLogTab(c CommonState, eventLogDir string) *state.LogTab {
	if c.RoostSessionID != "" && eventLogDir != "" {
		return &state.LogTab{
			Label: "EVENTS",
			Path:  filepath.Join(eventLogDir, c.RoostSessionID+".log"),
			Kind:  state.TabKindText,
		}
	}
	return nil
}

// firstNonEmpty returns the first string in candidates that is not empty.
func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}

// resolveCardTitle picks the session card Title from `aiTitle → summary → ""`.
// LastPrompt is never a Title candidate — ADR-0079 rejects promoting raw user
// prompts (un-summarised, often multi-line) into the title slot.
//
// Multi-line summaries used to drop to "" because the legacy Subtitle row
// would re-render them. That row no longer exists (Web Subtitle removed in
// ADR-0076, TUI removed in 5cb51eb, the peer/palette consumers ADR-0079
// §Decision 3 cited are gone too), so leaving multi-line summaries
// unrendered would lock affected sessions on the "New Session" placeholder
// forever. We collapse newlines into spaces so the Title row still gets the
// LLM's summary verbatim (modulo whitespace).
func resolveCardTitle(aiTitle, summary string) string {
	return firstNonEmpty(aiTitle, collapseToSingleLine(summary))
}

func resolveCardTitleWithDisplayFallback(aiTitle, summary, fallback string) string {
	return firstNonEmpty(aiTitle, collapseToSingleLine(summary), previewSummary(fallback))
}

func previewSummary(preview string) string {
	return previewText(collapseToSingleLine(preview))
}

// collapseToSingleLine folds CR/LF runs into single spaces and trims the
// result. Used to keep multi-line summaries renderable in the single-line
// card Title slot.
func collapseToSingleLine(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return strings.TrimSpace(s)
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// previewText truncates long text for display in info lines.
func previewText(text string) string {
	const max = 80
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
