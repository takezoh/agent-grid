package driver

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/takezoh/agent-grid/client/lib/claude/transcript"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/fishpath-go"
)

// view constructs a state.View snapshot from the cached ClaudeState.
// View building is pure: no I/O, no detection. Heavy work happens in
// Step before view is called.
//
// Card content:
//   - Title = transcript title, falling back to a single-line-collapsed
//     haiku-generated session summary (ADR-0079). LastPrompt
//     is never promoted to Title — it is only surfaced in the
//     INFO "Last Prompt" line.
//   - Tags  = [BranchTag?]
//
// StatusLine: cached from the transcript parse result.
func (d ClaudeDriver) view(cs ClaudeState) state.View {
	tags := CommonTags(cs.CommonState)

	var logTabs []state.LogTab
	if transcriptPath := d.resolveTranscriptPath(cs); transcriptPath != "" {
		rendererCfg, _ := json.Marshal(transcript.RendererConfig{
			SubagentDir:  subagentDir(transcriptPath),
			ShowThinking: d.showThinking,
		})
		logTabs = append(logTabs, state.LogTab{
			Label:       "TRANSCRIPT",
			Path:        transcriptPath,
			Kind:        transcript.KindTranscript,
			RendererCfg: rendererCfg,
		})
	}
	if tab := EventLogTab(cs.CommonState, d.eventLogDir); tab != nil {
		logTabs = append(logTabs, *tab)
	}

	return state.View{
		Card: state.Card{
			Title:       resolveCardTitle(cs.Title, cs.Summary),
			Tags:        tags,
			BorderTitle: CommandTag(ClaudeDriverName),
			BorderBadge: fishpath.Shorten(cs.StartDir, d.home),
		},
		DisplayName:     ClaudeDriverName,
		LogTabs:         logTabs,
		InfoExtras:      claudeInfoExtras(cs),
		StatusLine:      planStatusLine(cs),
		Model:           cs.Model,
		Effort:          cs.Effort,
		Status:          cs.Status,
		StatusChangedAt: cs.StatusChangedAt,
	}
}

func claudeInfoExtras(cs ClaudeState) []state.InfoLine {
	var lines []state.InfoLine
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, state.InfoLine{Label: label, Value: value})
		}
	}
	add("Title", cs.Title)
	add("Summary", cs.Summary)
	add("Model", cs.Model)
	add("Effort", cs.Effort)
	add("Last Prompt", cs.LastPrompt)
	add("Working Dir", cs.StartDir)
	if cs.BranchIsWorktree {
		add("Parent Branch", cs.BranchParentBranch)
	}
	add("Transcript", cs.TranscriptPath)
	return lines
}

// planStatusLine returns a status-line-formatted clickable "PLAN" label when the
// session has a detected plan file. The #[range=user|plan]…#[norange] markers
// register the region so the backend reports mouse_status_range="plan" on click.
func planStatusLine(cs ClaudeState) string {
	if cs.PlanFile != "" {
		return "#[range=user|plan]PLAN#[norange]"
	}
	return ""
}

func subagentDir(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}
	if !strings.HasSuffix(transcriptPath, ".jsonl") {
		return ""
	}
	base := strings.TrimSuffix(transcriptPath, ".jsonl")
	return base + string(os.PathSeparator) + "subagents"
}
