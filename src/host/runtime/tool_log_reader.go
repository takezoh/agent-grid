package runtime

import (
	"sort"
	"strings"
	"time"
)

// ActivityEventType discriminates workspace activity events emitted by
// ToolLogReader.
type ActivityEventType string

const (
	ActivityEventTurnRow      ActivityEventType = "turn_row"
	ActivityEventMidTurnTouch ActivityEventType = "mid_turn_touch"
)

// ActivityEvent is the closed sum of turn_row and mid_turn_touch events.
type ActivityEvent interface {
	ActivityType() ActivityEventType
}

// MidTurnTouchEvent is emitted immediately for each classified PostToolUse
// line (schema_version=2, non-turn_complete).
type MidTurnTouchEvent struct {
	SessionID     string
	Sequence      uint64
	TurnID        string
	Path          string
	FileEventKind FileEventKind
	ToolUseID     string
	ToolName      string
	Actor         string
	TS            time.Time
}

func (MidTurnTouchEvent) ActivityType() ActivityEventType { return ActivityEventMidTurnTouch }

// TurnRowDrillDown is one classified tool call within an aggregated row.
type TurnRowDrillDown struct {
	ToolUseID     string
	ToolName      string
	FileEventKind FileEventKind
	TS            time.Time
}

// TurnRowSubRow aggregates same-path calls from a nested sub-agent turn.
type TurnRowSubRow struct {
	TurnID string
	Path   string
	Count  int
	Events []TurnRowDrillDown
}

// TurnRowEvent is emitted on turn_complete with per-path counts and
// drill-down detail. SubRows carry nested sub-agent activity for the
// completing parent turn.
type TurnRowEvent struct {
	SessionID   string
	Sequence    uint64
	TurnID      string
	Path        string
	Count       int
	TurnFailure bool
	Events      []TurnRowDrillDown
	SubRows     []TurnRowSubRow
}

func (TurnRowEvent) ActivityType() ActivityEventType { return ActivityEventTurnRow }

// ToolLogReader classifies schema_version=2 JSONL tool-log lines and emits
// activity events. Legacy lines are skipped with a counter; unclassified v2
// lines increment a separate counter.
type ToolLogReader struct {
	legacySkipped       int
	unclassifiedSkipped int
	sequence            uint64
	turnBuf             map[string]map[string][]ToolLogRecord
}

// NewToolLogReader returns a reader with empty turn buffers.
func NewToolLogReader() *ToolLogReader {
	return &ToolLogReader{
		turnBuf: make(map[string]map[string][]ToolLogRecord),
	}
}

// LegacySkipped returns the number of legacy (pre-v2) lines skipped.
func (r *ToolLogReader) LegacySkipped() int { return r.legacySkipped }

// UnclassifiedSkipped returns v2 lines skipped for lacking a structured path.
func (r *ToolLogReader) UnclassifiedSkipped() int { return r.unclassifiedSkipped }

// ProcessLine parses one JSONL record and returns zero or more activity
// events. sessionID is stamped on emitted events; when empty, roost_session_id
// from the record is used.
func (r *ToolLogReader) ProcessLine(sessionID, line string) []ActivityEvent {
	rec, ok := ParseToolLogRecord(line)
	if !ok {
		return nil
	}
	if rec.IsLegacy() {
		r.legacySkipped++
		return nil
	}
	sid := sessionID
	if sid == "" {
		sid = rec.RoostSessionID
	}
	if rec.TurnComplete {
		return r.completeTurn(sid, rec)
	}
	if !rec.IsClassified() {
		r.unclassifiedSkipped++
		return nil
	}
	r.bufferTouch(rec)
	r.sequence++
	actor := rec.Actor
	if actor == "" {
		actor = ToolLogActorAgent
	}
	return []ActivityEvent{MidTurnTouchEvent{
		SessionID:     sid,
		Sequence:      r.sequence,
		TurnID:        rec.TurnID,
		Path:          rec.WorkspaceRelativePath,
		FileEventKind: rec.FileEventKind,
		ToolUseID:     rec.ToolUseID,
		ToolName:      rec.ToolName,
		Actor:         actor,
		TS:            rec.TS,
	}}
}

func (r *ToolLogReader) bufferTouch(rec ToolLogRecord) {
	turnID := rec.TurnID
	path := rec.WorkspaceRelativePath
	if r.turnBuf[turnID] == nil {
		r.turnBuf[turnID] = make(map[string][]ToolLogRecord)
	}
	r.turnBuf[turnID][path] = append(r.turnBuf[turnID][path], rec)
}

func (r *ToolLogReader) completeTurn(sessionID string, rec ToolLogRecord) []ActivityEvent {
	turnID := strings.TrimSpace(rec.TurnID)
	if turnID == "" {
		return nil
	}
	subRows := r.collectSubRows(turnID)
	paths := r.turnBuf[turnID]
	if len(paths) == 0 && len(subRows) == 0 {
		r.clearTurnFamily(turnID)
		return nil
	}
	var out []ActivityEvent
	for _, path := range sortedKeys(paths) {
		touches := paths[path]
		r.sequence++
		out = append(out, TurnRowEvent{
			SessionID:   sessionID,
			Sequence:    r.sequence,
			TurnID:      turnID,
			Path:        path,
			Count:       len(touches),
			TurnFailure: rec.TurnFailure,
			Events:      drillDownFromRecords(touches),
			SubRows:     subRows,
		})
	}
	r.clearTurnFamily(turnID)
	return out
}

func (r *ToolLogReader) collectSubRows(parentTurnID string) []TurnRowSubRow {
	prefix := parentTurnID + ".sub-"
	var subs []TurnRowSubRow
	for subTurnID, paths := range r.turnBuf {
		if !strings.HasPrefix(subTurnID, prefix) {
			continue
		}
		for _, path := range sortedKeys(paths) {
			touches := paths[path]
			subs = append(subs, TurnRowSubRow{
				TurnID: subTurnID,
				Path:   path,
				Count:  len(touches),
				Events: drillDownFromRecords(touches),
			})
		}
	}
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].TurnID != subs[j].TurnID {
			return subs[i].TurnID < subs[j].TurnID
		}
		return subs[i].Path < subs[j].Path
	})
	return subs
}

func (r *ToolLogReader) clearTurnFamily(parentTurnID string) {
	delete(r.turnBuf, parentTurnID)
	prefix := parentTurnID + ".sub-"
	for turnID := range r.turnBuf {
		if strings.HasPrefix(turnID, prefix) {
			delete(r.turnBuf, turnID)
		}
	}
}

func drillDownFromRecords(recs []ToolLogRecord) []TurnRowDrillDown {
	out := make([]TurnRowDrillDown, len(recs))
	for i, rec := range recs {
		out[i] = TurnRowDrillDown{
			ToolUseID:     rec.ToolUseID,
			ToolName:      rec.ToolName,
			FileEventKind: rec.FileEventKind,
			TS:            rec.TS,
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
