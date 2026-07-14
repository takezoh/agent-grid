package driver

import (
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

func codexToolLogWorkspace(cs CodexState) string {
	if cs.Project != "" {
		return cs.Project
	}
	return cs.StartDir
}

func (d CodexDriver) emitToolLog(cs CodexState, ev codexToolEvent, now time.Time, kind string, effs []state.Effect) (CodexState, []state.Effect) {
	var (
		durationMs int64
		toolInput  map[string]any
	)

	if ev.ToolUseID == "" {
		toolInput = ev.ToolInput
	} else if entry, ok := cs.PendingTools[ev.ToolUseID]; ok {
		delete(cs.PendingTools, ev.ToolUseID)
		if !entry.StartedAt.IsZero() && !now.IsZero() {
			durationMs = now.Sub(entry.StartedAt).Milliseconds()
		}
		toolInput = entry.Input
	} else {
		if kind == "auto" {
			kind = "orphan"
		}
		toolInput = ev.ToolInput
	}

	project := cs.Project
	if project == "" {
		project = cs.StartDir
	}
	slug := resolveProjectSlug(project)
	if slug == "" || ev.ToolName == "" {
		return cs, effs
	}

	summarised := summariseToolInput(ev.ToolName, toolInput)
	fileKind, relPath := classifyCodexTool(ev.ToolName, toolInput, codexToolLogWorkspace(cs))

	line := buildToolLogLine(toolLogEntry{
		SchemaVersion:         toolLogSchemaVersion,
		TS:                    now,
		RoostSessionID:        cs.RoostSessionID,
		ToolUseID:             ev.ToolUseID,
		ToolName:              ev.ToolName,
		Kind:                  kind,
		DurationMs:            durationMs,
		ToolInput:             summarised,
		Error:                 ev.Error,
		TurnID:                cs.ToolLogTurnID,
		FileEventKind:         fileKind,
		WorkspaceRelativePath: relPath,
	})
	effs = append(effs, state.EffToolLogAppend{
		Namespace: CodexDriverName,
		Project:   slug,
		Line:      line,
	})
	return cs, effs
}

func (d CodexDriver) emitCodexTurnBoundary(cs CodexState, now time.Time, failure bool) (CodexState, []state.Effect) {
	if cs.ToolLogTurnID == "" {
		return cs, nil
	}
	slug := resolveProjectSlug(cs.Project)
	if slug == "" {
		project := cs.Project
		if project == "" {
			project = cs.StartDir
		}
		slug = resolveProjectSlug(project)
	}
	if slug == "" {
		return cs, nil
	}
	line := buildToolLogLine(toolLogEntry{
		SchemaVersion:  toolLogSchemaVersion,
		TS:             now,
		RoostSessionID: cs.RoostSessionID,
		TurnID:         cs.ToolLogTurnID,
		TurnComplete:   true,
		TurnFailure:    failure,
	})
	return cs, []state.Effect{state.EffToolLogAppend{Namespace: CodexDriverName, Project: slug, Line: line}}
}
