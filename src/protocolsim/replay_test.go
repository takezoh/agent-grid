// Package protocolsim holds T0 tests that lock the protocol/simulator
// recorded scenarios without a live agent (FR-P1-07/08).
package protocolsim

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApprovalRoundTripRecording(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// src/protocolsim → repo root → protocol/simulator/recordings
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	path := filepath.Join(root, "protocol", "simulator", "recordings", "approval-round-trip.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0]["k"] != "ar" || events[1]["k"] != "ax" {
		t.Fatalf("k sequence = %v, %v", events[0]["k"], events[1]["k"])
	}
	// Fixture twin must agree on ids.
	fixPath := filepath.Join(root, "protocol", "simulator", "fixtures", "approval-round-trip.json")
	raw, err := os.ReadFile(fixPath)
	if err != nil {
		t.Fatal(err)
	}
	var fix struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(raw, &fix); err != nil {
		t.Fatal(err)
	}
	if len(fix.Events) != len(events) {
		t.Fatalf("fixture events %d != recording %d", len(fix.Events), len(events))
	}
	for i := range events {
		if events[i]["k"] != fix.Events[i]["k"] {
			t.Fatalf("event %d k mismatch: %v vs %v", i, events[i]["k"], fix.Events[i]["k"])
		}
	}
}
