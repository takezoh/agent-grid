package web

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
)

func TestEncodeFromActivityEvents_TurnRowAndMidTurnTouch(t *testing.T) {
	ev := proto.EvtActivityEvents{
		SessionID: "s1",
		Events: []proto.ActivityEventWire{
			{
				Type:      "turn_row",
				Sequence:  1,
				SessionID: "s1",
				TurnID:    "t1",
				Path:      "src/foo.ts",
				Count:     2,
				Events: []proto.ActivityDrillDownWire{
					{ToolUseID: "tc1", FileEventKind: "read"},
					{ToolUseID: "tc2", FileEventKind: "edit"},
				},
			},
			{
				Type:      "mid_turn_touch",
				Sequence:  2,
				SessionID: "s1",
				Path:      "src/foo.ts",
				ToolUseID: "tc3",
			},
		},
	}
	got := encodeFromActivityEvents(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k: got %v, want \"v\"", m["k"])
	}
	if m["activity_session_id"] != "s1" {
		t.Errorf("activity_session_id: got %v, want \"s1\"", m["activity_session_id"])
	}
	if _, has := m["sessions"]; has {
		t.Errorf("activity-only frame must omit sessions; got %v", m["sessions"])
	}

	events, ok := m["activity_events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("activity_events: got %T %v", m["activity_events"], m["activity_events"])
	}

	turnRow, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("events[0] type: %T", events[0])
	}
	if turnRow["type"] != "turn_row" {
		t.Errorf("turn_row type: got %v", turnRow["type"])
	}
	if turnRow["path"] != "src/foo.ts" {
		t.Errorf("turn_row path: got %v, want workspace_relative_path mapped to path", turnRow["path"])
	}
	if turnRow["kind"] != "edit" {
		t.Errorf("turn_row kind: got %v, want dominant \"edit\"", turnRow["kind"])
	}
	drill, ok := turnRow["events"].([]any)
	if !ok || len(drill) != 2 {
		t.Fatalf("turn_row events: got %v", turnRow["events"])
	}
	d0 := drill[0].(map[string]any)
	if d0["path"] != "src/foo.ts" || d0["kind"] != "read" || d0["tool_call_id"] != "tc1" {
		t.Errorf("drill-down[0]: got %v", d0)
	}

	touch, ok := events[1].(map[string]any)
	if !ok {
		t.Fatalf("events[1] type: %T", events[1])
	}
	if touch["type"] != "mid_turn_touch" {
		t.Errorf("mid_turn_touch type: got %v", touch["type"])
	}
	if touch["tool_call_id"] != "tc3" {
		t.Errorf("mid_turn_touch tool_call_id: got %v, want mapped from tool_use_id", touch["tool_call_id"])
	}
}

func TestEncodeFromActivityEvents_OperatorActorPassesThrough(t *testing.T) {
	ev := proto.EvtActivityEvents{
		SessionID: "s1",
		Events: []proto.ActivityEventWire{{
			Type:          "mid_turn_touch",
			Sequence:      1,
			SessionID:     "s1",
			Path:          "src/foo.ts",
			FileEventKind: "edit",
			Actor:         "operator",
			ToolUseID:     "tc-op",
		}},
	}
	got := encodeFromActivityEvents(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	events, ok := m["activity_events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("activity_events: got %T %v", m["activity_events"], m["activity_events"])
	}
	touch := events[0].(map[string]any)
	if touch["actor"] != "operator" {
		t.Errorf("actor: got %v, want operator", touch["actor"])
	}
	if touch["kind"] != "edit" {
		t.Errorf("kind: got %v, want edit", touch["kind"])
	}
}

func TestEncodeFromActivityEvents_EmptyEventsReturnsNil(t *testing.T) {
	got := encodeFromActivityEvents(proto.EvtActivityEvents{SessionID: "s1"})
	if got != nil {
		t.Errorf("expected nil for empty events, got %s", got)
	}
}

func TestViewUpdateActivityLatency(t *testing.T) {
	ev := proto.EvtActivityEvents{
		SessionID: "s1",
		Events: []proto.ActivityEventWire{
			{
				Type:      "turn_row",
				Sequence:  1,
				SessionID: "s1",
				TurnID:    "t1",
				Path:      "src/foo.ts",
				Count:     1,
				Events:    []proto.ActivityDrillDownWire{{ToolUseID: "tc1", FileEventKind: "read"}},
			},
		},
	}
	const iterations = 200
	latencies := make([]time.Duration, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		frame := encodeFromActivityEvents(ev)
		latencies[i] = time.Since(start)
		if frame == nil {
			t.Fatal("encode returned nil")
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[(iterations*95)/100]
	// Server-side encode must stay far below the 750 ms end-to-end contract
	// budget; this guards against accidental synchronous heavy work on the
	// hot path (JSON marshal + field mapping only).
	if p95 > 5*time.Millisecond {
		t.Errorf("encode p95 = %v, want <= 5ms (hot-path budget)", p95)
	}
}

func TestViewUpdateActivityCrossSessionIsolation(t *testing.T) {
	evA := proto.EvtActivityEvents{
		SessionID: "session-a",
		Events: []proto.ActivityEventWire{{
			Type: "mid_turn_touch", Sequence: 1, SessionID: "session-a",
			Path: "a.go", ToolUseID: "tc-a",
		}},
	}
	evB := proto.EvtActivityEvents{
		SessionID: "session-b",
		Events: []proto.ActivityEventWire{{
			Type: "mid_turn_touch", Sequence: 1, SessionID: "session-b",
			Path: "b.go", ToolUseID: "tc-b",
		}},
	}
	frameA := encodeFromActivityEvents(evA)
	frameB := encodeFromActivityEvents(evB)
	var mA, mB map[string]any
	if err := json.Unmarshal(frameA, &mA); err != nil {
		t.Fatalf("unmarshal A: %v", err)
	}
	if err := json.Unmarshal(frameB, &mB); err != nil {
		t.Fatalf("unmarshal B: %v", err)
	}
	if mA["activity_session_id"] != "session-a" {
		t.Errorf("session A id: %v", mA["activity_session_id"])
	}
	if mB["activity_session_id"] != "session-b" {
		t.Errorf("session B id: %v", mB["activity_session_id"])
	}
	eventsA := mA["activity_events"].([]any)[0].(map[string]any)
	if eventsA["session_id"] != "session-a" {
		t.Errorf("event session_id leak: %v", eventsA["session_id"])
	}
}

func TestEncodeServerEvent_ActivityEvents(t *testing.T) {
	ev := proto.EvtActivityEvents{
		SessionID: "s1",
		Events: []proto.ActivityEventWire{{
			Type: "mid_turn_touch", Sequence: 1, SessionID: "s1", Path: "a.go", ToolUseID: "tc",
		}},
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("encodeServerEvent returned nil")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k: got %v", m["k"])
	}
}
