package driver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/state"
)

func toolStartedEv(id, name, cmd string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolStarted,
		Timestamp: ts,
		Payload: state.SubsystemPayload{
			Tool: &state.SubsystemTool{ID: id, Name: name, Command: cmd},
		},
	}
}

func toolCompletedEv(id, name, cmd string, toolErr string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolCompleted,
		Timestamp: ts,
		Payload: state.SubsystemPayload{
			Tool: &state.SubsystemTool{ID: id, Name: name, Command: cmd, Error: toolErr},
		},
	}
}

func TestCodexSubsystemToolCompletedEmitsToolLog(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	cs, _ = d.handleSubsystem(cs, ctx, toolStartedEv("tool-1", "Bash", "echo hi", now))

	next, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("tool-1", "Bash", "echo hi", "", now.Add(2*time.Second)))

	if next.CurrentTool != "" {
		t.Fatalf("CurrentTool = %q, want empty", next.CurrentTool)
	}

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}
	if appendEff.Namespace != CodexDriverName {
		t.Fatalf("Namespace = %q, want %q", appendEff.Namespace, CodexDriverName)
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "auto" {
		t.Fatalf("Kind = %q, want auto", entry.Kind)
	}
	if entry.ToolName != "Bash" {
		t.Fatalf("ToolName = %q, want Bash", entry.ToolName)
	}
	if entry.DurationMs != 2000 {
		t.Fatalf("DurationMs = %d, want 2000", entry.DurationMs)
	}
}

func TestCodexSubsystemToolCompletedRecordsToolError(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	cs, _ = d.handleSubsystem(cs, ctx, toolStartedEv("tool-err", "Bash", "false", now))
	_, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("tool-err", "Bash", "false", "exit 1", now.Add(time.Second)))

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}
	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "failed" {
		t.Fatalf("Kind = %q, want failed", entry.Kind)
	}
	if entry.Error != "exit 1" {
		t.Fatalf("Error = %q, want exit 1", entry.Error)
	}
}

func TestCodexSubsystemToolCompletedWithoutStartIsOrphan(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	_, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("missing-id", "Read", "", "", now))

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "orphan" {
		t.Fatalf("Kind = %q, want orphan", entry.Kind)
	}
}

func codexTurnStartedEv(turnID string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemTurnStarted,
		Timestamp: ts,
		Payload:   state.SubsystemPayload{TurnID: turnID},
	}
}

func codexToolStartedEv(id, name, path string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolStarted,
		Timestamp: ts,
		Payload: state.SubsystemPayload{
			Tool: &state.SubsystemTool{ID: id, Name: name, Path: path},
		},
	}
}

func decodeCodexToolLogLine(t *testing.T, effs []state.Effect) map[string]any {
	t.Helper()
	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(appendEff.Line), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestCodexToolLogV2_ReadAndApplyPatchClassification(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	cs, _ = d.handleSubsystem(cs, ctx, codexTurnStartedEv("turn-1", now))
	cs, _ = d.handleSubsystem(cs, ctx, codexToolStartedEv("t-read", "read_file", "/repo/lib/a.go", now))
	_, effsRead := d.handleSubsystem(cs, ctx, toolCompletedEv("t-read", "read_file", "", "", now.Add(time.Second)))
	mRead := decodeCodexToolLogLine(t, effsRead)
	if mRead["schema_version"] != float64(2) {
		t.Errorf("schema_version = %v", mRead["schema_version"])
	}
	if mRead["file_event_kind"] != "read" {
		t.Errorf("read kind = %v", mRead["file_event_kind"])
	}
	if mRead["turn_id"] != "turn-1" {
		t.Errorf("turn_id = %v, want turn-1", mRead["turn_id"])
	}

	cs, _ = d.handleSubsystem(cs, ctx, codexToolStartedEv("t-patch", "apply_patch", "/repo/lib/b.go", now.Add(2*time.Second)))
	_, effsPatch := d.handleSubsystem(cs, ctx, toolCompletedEv("t-patch", "apply_patch", "", "", now.Add(3*time.Second)))
	mPatch := decodeCodexToolLogLine(t, effsPatch)
	if mPatch["file_event_kind"] != "edit" {
		t.Errorf("apply_patch kind = %v, want edit", mPatch["file_event_kind"])
	}
	if mPatch["workspace_relative_path"] != "lib/b.go" {
		t.Errorf("path = %v", mPatch["workspace_relative_path"])
	}
}

func TestCodexToolLogV2_TurnIDChangesOnTurnStarted(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	cs, _ = d.handleSubsystem(cs, ctx, codexTurnStartedEv("turn-a", now))
	cs, _ = d.handleSubsystem(cs, ctx, codexToolStartedEv("t1", "read_file", "/repo/a.go", now))
	_, effs1 := d.handleSubsystem(cs, ctx, toolCompletedEv("t1", "read_file", "", "", now.Add(time.Second)))
	if decodeCodexToolLogLine(t, effs1)["turn_id"] != "turn-a" {
		t.Fatal("expected turn-a on first turn")
	}

	cs, _ = d.handleSubsystem(cs, ctx, codexTurnStartedEv("turn-b", now.Add(2*time.Second)))
	cs, _ = d.handleSubsystem(cs, ctx, codexToolStartedEv("t2", "read_file", "/repo/b.go", now.Add(2*time.Second)))
	_, effs2 := d.handleSubsystem(cs, ctx, toolCompletedEv("t2", "read_file", "", "", now.Add(3*time.Second)))
	if decodeCodexToolLogLine(t, effs2)["turn_id"] != "turn-b" {
		t.Fatal("expected turn-b after SubsystemTurnStarted")
	}
}

func TestCodexSubsystemToolCompletedSummarisesToolInput(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	longCmd := strings.Repeat("x", 240)
	_, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("tool-2", "Bash", longCmd, "", now))

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	cmd, _ := entry.ToolInput["command"].(string)
	if len([]rune(cmd)) > 201 {
		t.Fatalf("command too long: %d runes", len([]rune(cmd)))
	}
	if !strings.HasSuffix(cmd, "…") {
		t.Fatalf("command = %q, want truncated suffix", cmd)
	}
}
