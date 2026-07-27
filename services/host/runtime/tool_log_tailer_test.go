package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolLogTailerResumeOffsetAfterRestart(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "tool.jsonl")

	line1 := toolLogJSONLine(t, classifiedRec("turn-1", "a.go", "tu-1", FileEventRead))
	line2 := toolLogJSONLine(t, classifiedRec("turn-1", "b.go", "tu-2", FileEventEdit))
	line3 := toolLogJSONLine(t, turnCompleteRec("turn-1", false))

	if err := os.WriteFile(logPath, []byte(line1+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader1 := NewToolLogReader()
	tailer1 := NewToolLogTailer(reader1)
	events1, err := tailer1.ReplayFile("sess-1", "claude", "-proj", logPath)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(midTouches(events1)) != 1 {
		t.Fatalf("first replay: got %d mid_turn_touch, want 1", len(midTouches(events1)))
	}
	offset := tailer1.Offset("claude", "-proj")
	if offset == 0 {
		t.Fatal("expected non-zero offset after first line")
	}

	// Simulate daemon restart: new tailer restores offset; reader state is
	// preserved in-process (turn buffers survive until turn_complete).
	tailer2 := NewToolLogTailer(reader1)
	tailer2.offsets[tailer2.key("claude", "-proj")] = offset

	if err := os.WriteFile(logPath, []byte(line1+"\n"+line2+"\n"+line3+"\n"), 0o644); err != nil {
		t.Fatalf("append write: %v", err)
	}
	events2, err := tailer2.ReplayFile("sess-1", "claude", "-proj", logPath)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if len(midTouches(events2)) != 1 {
		t.Fatalf("resume replay: got %d mid_turn_touch, want 1 (line1 skipped)", len(midTouches(events2)))
	}
	if len(turnRows(events2)) != 2 {
		t.Fatalf("resume replay: got %d turn rows, want 2 (b.go + turn complete)", len(turnRows(events2)))
	}
}
