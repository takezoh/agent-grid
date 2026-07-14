package runtime

import (
	"log/slog"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
)

func (r *Runtime) emitToolLogActivityEvents(line string) {
	if r.toolLogReader == nil {
		return
	}
	rec, ok := ParseToolLogRecord(line)
	if !ok {
		return
	}
	sessionID := rec.RoostSessionID
	events := r.toolLogReader.ProcessLine(sessionID, line)
	if len(events) == 0 {
		return
	}
	r.broadcastActivityEvents(sessionID, events)
}

func (r *Runtime) broadcastActivityEvents(sessionID string, events []ActivityEvent) {
	wireEvents := make([]proto.ActivityEventWire, 0, len(events))
	for _, ev := range events {
		switch e := ev.(type) {
		case MidTurnTouchEvent:
			wireEvents = append(wireEvents, proto.ActivityEventWire{
				Type:          string(ActivityEventMidTurnTouch),
				Sequence:      e.Sequence,
				SessionID:     e.SessionID,
				TurnID:        e.TurnID,
				Path:          e.Path,
				FileEventKind: string(e.FileEventKind),
				ToolUseID:     e.ToolUseID,
				ToolName:      e.ToolName,
				TS:            formatActivityTS(e.TS),
			})
		case TurnRowEvent:
			wireEvents = append(wireEvents, proto.ActivityEventWire{
				Type:          string(ActivityEventTurnRow),
				Sequence:      e.Sequence,
				SessionID:     e.SessionID,
				TurnID:        e.TurnID,
				Path:          e.Path,
				FileEventKind: "",
				Count:         e.Count,
				TurnFailure:   e.TurnFailure,
				Events:        drillDownToWire(e.Events),
				SubRows:       subRowsToWire(e.SubRows),
			})
		default:
			slog.Warn("runtime: unknown activity event type", "type", ev.ActivityType())
		}
	}
	if len(wireEvents) == 0 {
		return
	}
	ev := proto.EvtActivityEvents{
		SessionID: sessionID,
		Events:    wireEvents,
	}
	wire, err := proto.EncodeEvent(ev)
	if err != nil {
		slog.Error("runtime: encode activity events failed", "err", err)
		return
	}
	r.broadcastWire(wire, proto.EvtNameActivityEvents)
}

func drillDownToWire(in []TurnRowDrillDown) []proto.ActivityDrillDownWire {
	if len(in) == 0 {
		return nil
	}
	out := make([]proto.ActivityDrillDownWire, len(in))
	for i, d := range in {
		out[i] = proto.ActivityDrillDownWire{
			ToolUseID:     d.ToolUseID,
			ToolName:      d.ToolName,
			FileEventKind: string(d.FileEventKind),
			TS:            formatActivityTS(d.TS),
		}
	}
	return out
}

func subRowsToWire(in []TurnRowSubRow) []proto.ActivityTurnSubRowWire {
	if len(in) == 0 {
		return nil
	}
	out := make([]proto.ActivityTurnSubRowWire, len(in))
	for i, s := range in {
		out[i] = proto.ActivityTurnSubRowWire{
			TurnID: s.TurnID,
			Path:   s.Path,
			Count:  s.Count,
			Events: drillDownToWire(s.Events),
		}
	}
	return out
}

func formatActivityTS(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
