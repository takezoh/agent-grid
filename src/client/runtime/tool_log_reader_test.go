package runtime

import (
	"encoding/json"
	"testing"
	"time"
)

func toolLogJSONLine(t *testing.T, rec ToolLogRecord) string {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	return string(b)
}

func classifiedRec(turnID, path, toolUseID string, kind FileEventKind) ToolLogRecord {
	return ToolLogRecord{
		SchemaVersion:         ToolLogSchemaVersion,
		TS:                    time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
		RoostSessionID:        "sess-1",
		ToolUseID:             toolUseID,
		ToolName:              "Read",
		Kind:                  "auto",
		TurnID:                turnID,
		FileEventKind:         kind,
		WorkspaceRelativePath: path,
	}
}

func turnCompleteRec(turnID string, failure bool) ToolLogRecord {
	return ToolLogRecord{
		SchemaVersion:  ToolLogSchemaVersion,
		TS:             time.Date(2026, 7, 14, 12, 1, 0, 0, time.UTC),
		RoostSessionID: "sess-1",
		TurnID:         turnID,
		TurnComplete:   true,
		TurnFailure:    failure,
	}
}

func processLines(t *testing.T, r *ToolLogReader, lines ...string) []ActivityEvent {
	t.Helper()
	var all []ActivityEvent
	for _, line := range lines {
		all = append(all, r.ProcessLine("sess-1", line)...)
	}
	return all
}

func turnRows(events []ActivityEvent) []TurnRowEvent {
	var rows []TurnRowEvent
	for _, ev := range events {
		if row, ok := ev.(TurnRowEvent); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func midTouches(events []ActivityEvent) []MidTurnTouchEvent {
	var touches []MidTurnTouchEvent
	for _, ev := range events {
		if touch, ok := ev.(MidTurnTouchEvent); ok {
			touches = append(touches, touch)
		}
	}
	return touches
}

func TestToolLogReaderClassification(t *testing.T) {
	r := NewToolLogReader()

	classified := classifiedRec("turn-1", "src/a.go", "tu-1", FileEventRead)
	events := r.ProcessLine("sess-1", toolLogJSONLine(t, classified))
	if len(events) != 1 {
		t.Fatalf("classified line: got %d events, want 1", len(events))
	}
	touch, ok := events[0].(MidTurnTouchEvent)
	if !ok {
		t.Fatalf("expected MidTurnTouchEvent, got %T", events[0])
	}
	if touch.Path != "src/a.go" || touch.FileEventKind != FileEventRead {
		t.Fatalf("touch = %+v", touch)
	}

	legacyLine := `{"kind":"auto","tool_name":"Read","tool_use_id":"old"}`
	if evs := r.ProcessLine("sess-1", legacyLine); len(evs) != 0 {
		t.Fatalf("legacy line should emit nothing, got %v", evs)
	}
	if r.LegacySkipped() != 1 {
		t.Fatalf("legacy_skipped = %d, want 1", r.LegacySkipped())
	}

	unclassified := ToolLogRecord{
		SchemaVersion:  ToolLogSchemaVersion,
		RoostSessionID: "sess-1",
		ToolName:       "Bash",
		ToolUseID:      "tu-bash",
		TurnID:         "turn-1",
		FileEventKind:  FileEventUnclassified,
	}
	if evs := r.ProcessLine("sess-1", toolLogJSONLine(t, unclassified)); len(evs) != 0 {
		t.Fatalf("unclassified line should emit nothing, got %v", evs)
	}
	if r.UnclassifiedSkipped() != 1 {
		t.Fatalf("unclassified_skipped = %d, want 1", r.UnclassifiedSkipped())
	}

	all := processLines(t, r,
		toolLogJSONLine(t, classifiedRec("turn-2", "pkg/b.go", "tu-2", FileEventEdit)),
		toolLogJSONLine(t, turnCompleteRec("turn-2", false)),
	)
	rows := turnRows(all)
	if len(rows) != 1 {
		t.Fatalf("turn_complete: got %d turn rows, want 1", len(rows))
	}
	if rows[0].Count != 1 || rows[0].Path != "pkg/b.go" {
		t.Fatalf("turn row = %+v", rows[0])
	}
	if len(rows[0].Events) != 1 || rows[0].Events[0].ToolUseID != "tu-2" {
		t.Fatalf("drill-down = %+v", rows[0].Events)
	}

	for _, ev := range all {
		switch e := ev.(type) {
		case TurnRowEvent:
			if e.Path == "" {
				t.Fatal("fabricated empty path in turn row")
			}
		case MidTurnTouchEvent:
			if e.Path == "" {
				t.Fatal("fabricated empty path in mid_turn_touch")
			}
		}
	}
}

func TestTurnAggregation(t *testing.T) {
	t.Run("codex_same_path_three_calls_one_row", func(t *testing.T) {
		r := NewToolLogReader()
		turnID := "codex-turn-1"
		lines := []string{
			toolLogJSONLine(t, classifiedRec(turnID, "lib/x.go", "tu-1", FileEventEdit)),
			toolLogJSONLine(t, classifiedRec(turnID, "lib/x.go", "tu-2", FileEventEdit)),
			toolLogJSONLine(t, classifiedRec(turnID, "lib/x.go", "tu-3", FileEventEdit)),
			toolLogJSONLine(t, turnCompleteRec(turnID, false)),
		}
		events := processLines(t, r, lines...)
		rows := turnRows(events)
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1", len(rows))
		}
		if rows[0].Count != 3 {
			t.Fatalf("count = %d, want 3", rows[0].Count)
		}
		if len(midTouches(events)) != 3 {
			t.Fatalf("mid_turn_touch count = %d, want 3", len(midTouches(events)))
		}
	})

	t.Run("claude_same_path_three_calls_one_row", func(t *testing.T) {
		r := NewToolLogReader()
		turnID := "1"
		lines := []string{
			toolLogJSONLine(t, classifiedRec(turnID, "src/main.go", "tu-a", FileEventRead)),
			toolLogJSONLine(t, classifiedRec(turnID, "src/main.go", "tu-b", FileEventRead)),
			toolLogJSONLine(t, classifiedRec(turnID, "src/main.go", "tu-c", FileEventRead)),
			toolLogJSONLine(t, turnCompleteRec(turnID, false)),
		}
		events := processLines(t, r, lines...)
		rows := turnRows(events)
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1", len(rows))
		}
		if rows[0].Count != 3 {
			t.Fatalf("count = %d, want 3", rows[0].Count)
		}
	})

	t.Run("claude_stop_failure_then_new_turn", func(t *testing.T) {
		r := NewToolLogReader()
		turn1 := []string{
			toolLogJSONLine(t, classifiedRec("1", "a.txt", "tu-1", FileEventEdit)),
			toolLogJSONLine(t, classifiedRec("1", "a.txt", "tu-2", FileEventEdit)),
			toolLogJSONLine(t, classifiedRec("1", "a.txt", "tu-3", FileEventEdit)),
			toolLogJSONLine(t, turnCompleteRec("1", true)),
		}
		turn2 := []string{
			toolLogJSONLine(t, classifiedRec("2", "b.txt", "tu-4", FileEventCreate)),
			toolLogJSONLine(t, turnCompleteRec("2", false)),
		}
		events := processLines(t, r, append(turn1, turn2...)...)
		rows := turnRows(events)
		if len(rows) != 2 {
			t.Fatalf("got %d rows, want 2", len(rows))
		}
		if rows[0].Count != 3 || !rows[0].TurnFailure {
			t.Fatalf("first row = %+v, want count=3 failure=true", rows[0])
		}
		if rows[1].Count != 1 || rows[1].Path != "b.txt" {
			t.Fatalf("second row = %+v", rows[1])
		}
	})

	t.Run("claude_nested_subagent", func(t *testing.T) {
		r := NewToolLogReader()
		lines := []string{
			toolLogJSONLine(t, classifiedRec("1", "parent.go", "tu-parent", FileEventRead)),
			toolLogJSONLine(t, classifiedRec("1.sub-1", "sub/agent.go", "tu-sub", FileEventEdit)),
			toolLogJSONLine(t, turnCompleteRec("1", false)),
		}
		events := processLines(t, r, lines...)
		rows := turnRows(events)
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1 parent row", len(rows))
		}
		row := rows[0]
		if row.Path != "parent.go" || row.Count != 1 {
			t.Fatalf("parent row = %+v", row)
		}
		if len(row.SubRows) != 1 {
			t.Fatalf("sub_rows = %+v, want 1", row.SubRows)
		}
		sub := row.SubRows[0]
		if sub.TurnID != "1.sub-1" || sub.Path != "sub/agent.go" || sub.Count != 1 {
			t.Fatalf("sub row = %+v", sub)
		}
	})
}
