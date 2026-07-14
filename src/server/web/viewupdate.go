package web

import (
	"encoding/json"
	"log/slog"

	"github.com/takezoh/agent-grid/client/proto"
)

// viewActivityDrillDown is the browser-facing drill-down entry on a turn_row.
// Proto uses file_event_kind + tool_use_id; the web store expects path/kind/
// tool_call_id (ADR-20260714-wsviewer-live-transport).
type viewActivityDrillDown struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// viewActivityEvent is the browser-facing activity_events element.
type viewActivityEvent struct {
	Type       string                  `json:"type"`
	Sequence   uint64                  `json:"sequence"`
	SessionID  string                  `json:"session_id"`
	TurnID     string                  `json:"turn_id,omitempty"`
	Path       string                  `json:"path,omitempty"`
	Kind       string                  `json:"kind,omitempty"`
	Count      int                     `json:"count,omitempty"`
	ToolCallID string                  `json:"tool_call_id,omitempty"`
	Actor      string                  `json:"actor,omitempty"`
	Events     []viewActivityDrillDown `json:"events,omitempty"`
}

var fileEventKindPriority = map[string]int{
	"read":   1,
	"delete": 2,
	"create": 3,
	"edit":   4,
}

// encodeFromActivityEvents encodes EvtActivityEvents as a partial view-update
// frame {"k":"v","activity_session_id":…,"activity_events":[…]} without a
// sessions array. Omitting sessions preserves adr-20260705 sessions-only
// scoping — activity-only pushes must not wipe the browser session list.
func encodeFromActivityEvents(ev proto.EvtActivityEvents) []byte {
	if len(ev.Events) == 0 {
		return nil
	}
	events := make([]viewActivityEvent, 0, len(ev.Events))
	for _, e := range ev.Events {
		events = append(events, protoActivityToView(e))
	}
	f := viewUpdateFrame{
		K:                 "v",
		ActivitySessionID: ev.SessionID,
		ActivityEvents:    events,
	}
	b, err := json.Marshal(f)
	if err != nil {
		slog.Error("wire: failed to encode activity view-update frame", "err", err)
		return nil
	}
	return b
}

func protoActivityToView(e proto.ActivityEventWire) viewActivityEvent {
	out := viewActivityEvent{
		Type:      e.Type,
		Sequence:  e.Sequence,
		SessionID: e.SessionID,
		TurnID:    e.TurnID,
		Path:      e.Path,
		Count:     e.Count,
	}
	switch e.Type {
	case "mid_turn_touch":
		out.ToolCallID = e.ToolUseID
		out.Kind = e.FileEventKind
		out.Actor = e.Actor
	case "turn_row":
		out.Events = drillDownToView(e.Path, e.Events)
		out.Kind = dominantFileEventKind(out.Events, e.FileEventKind)
	default:
		out.Kind = e.FileEventKind
	}
	return out
}

func drillDownToView(rowPath string, in []proto.ActivityDrillDownWire) []viewActivityDrillDown {
	if len(in) == 0 {
		return nil
	}
	out := make([]viewActivityDrillDown, len(in))
	for i, d := range in {
		out[i] = viewActivityDrillDown{
			Path:       rowPath,
			Kind:       d.FileEventKind,
			ToolCallID: d.ToolUseID,
		}
	}
	return out
}

func dominantFileEventKind(events []viewActivityDrillDown, fallback string) string {
	best := fallback
	bestP := fileEventKindPriority[fallback]
	for _, e := range events {
		if p := fileEventKindPriority[e.Kind]; p > bestP {
			bestP = p
			best = e.Kind
		}
	}
	if best == "" && len(events) > 0 {
		return events[0].Kind
	}
	return best
}
