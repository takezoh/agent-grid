package runtime

import (
	"encoding/json"
	"testing"
)

func TestParseToolLogRecord_LegacyLineUnmodified(t *testing.T) {
	legacy := `{"kind":"auto","tool_name":"Read","tool_use_id":"old-id"}`
	rec, ok := ParseToolLogRecord(legacy)
	if !ok {
		t.Fatal("expected legacy line to parse")
	}
	if !rec.IsLegacy() {
		t.Fatal("expected IsLegacy true")
	}
	if rec.SchemaVersion != 0 {
		t.Errorf("schema_version = %d, want 0/absent", rec.SchemaVersion)
	}
}

func TestParseToolLogRecord_V2Fields(t *testing.T) {
	line := toolLogJSONLine(t, classifiedRec("turn-1", "src/a.go", "tu-1", FileEventRead))
	rec, ok := ParseToolLogRecord(line)
	if !ok {
		t.Fatal("parse failed")
	}
	if rec.SchemaVersion != ToolLogSchemaVersion {
		t.Errorf("schema_version = %d", rec.SchemaVersion)
	}
	if rec.TurnID != "turn-1" {
		t.Errorf("turn_id = %q", rec.TurnID)
	}
	if rec.FileEventKind != FileEventRead {
		t.Errorf("file_event_kind = %q", rec.FileEventKind)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("json: %v", err)
	}
	if raw["schema_version"] != float64(2) {
		t.Errorf("wire schema_version = %v", raw["schema_version"])
	}
}
