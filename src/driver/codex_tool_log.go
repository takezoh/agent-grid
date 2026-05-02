package driver

import (
	"time"

	"github.com/takezoh/agent-roost/state"
)

func (d CodexDriver) handleToolLog(cs CodexState, hp codexHookPayload, now time.Time, effs []state.Effect) (CodexState, []state.Effect) {
	switch hp.HookEventName {
	case "PreToolUse":
		if hp.ToolUseID == "" || hp.ToolName == "" {
			return cs, effs
		}
		if cs.PendingTools == nil {
			cs.PendingTools = make(map[string]codexPendingTool)
		}
		cs.PendingTools[hp.ToolUseID] = codexPendingTool{
			Name:      hp.ToolName,
			Input:     hp.ToolInput,
			StartedAt: now,
		}
		return cs, effs
	case "PostToolUse":
		return d.emitToolLog(cs, hp, now, "auto", effs)
	case "PostToolUseFailure":
		kind := "failed"
		if hp.IsInterrupt {
			kind = "denied"
		}
		return d.emitToolLog(cs, hp, now, kind, effs)
	default:
		return cs, effs
	}
}

func (d CodexDriver) emitToolLog(cs CodexState, hp codexHookPayload, now time.Time, kind string, effs []state.Effect) (CodexState, []state.Effect) {
	var (
		durationMs int64
		toolInput  map[string]any
	)

	if hp.ToolUseID == "" {
		toolInput = hp.ToolInput
	} else if entry, ok := cs.PendingTools[hp.ToolUseID]; ok {
		delete(cs.PendingTools, hp.ToolUseID)
		if !entry.StartedAt.IsZero() && !now.IsZero() {
			durationMs = now.Sub(entry.StartedAt).Milliseconds()
		}
		toolInput = entry.Input
	} else {
		if kind == "auto" {
			kind = "orphan"
		}
		toolInput = hp.ToolInput
	}

	project := cs.Project
	if project == "" {
		project = cs.StartDir
	}
	slug := resolveProjectSlug(project)
	if slug == "" || hp.ToolName == "" {
		return cs, effs
	}

	line := buildToolLogLine(toolLogEntry{
		TS:             now,
		RoostSessionID: cs.RoostSessionID,
		ToolUseID:      hp.ToolUseID,
		ToolName:       hp.ToolName,
		Kind:           kind,
		PermissionMode: hp.PermissionMode,
		DurationMs:     durationMs,
		ToolInput:      summariseToolInput(hp.ToolName, toolInput),
		Error:          hp.Error,
	})
	effs = append(effs, state.EffToolLogAppend{
		Namespace: CodexDriverName,
		Project:   slug,
		Line:      line,
	})
	return cs, effs
}
