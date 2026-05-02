package driver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/state"
)

func TestCodexPostToolUseEmitsToolLog(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"

	pre := codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_use_id":     "tool-1",
	}, now)
	next, _ := d.handleHook(cs, state.FrameContext{IsRoot: true}, pre)
	if next.CurrentTool != "Bash" {
		t.Fatalf("CurrentTool = %q, want Bash", next.CurrentTool)
	}

	post := codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_use_id":     "tool-1",
	}, now.Add(2*time.Second))
	next, effs := d.handleHook(next, state.FrameContext{IsRoot: true}, post)
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

func TestCodexPostToolUseWithoutPreIsOrphan(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"

	post := codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Read",
		"tool_use_id":     "missing",
	}, now)
	_, effs := d.handleHook(cs, state.FrameContext{IsRoot: true}, post)

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

func TestCodexPostToolUseSummarisesToolInput(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	long := strings.Repeat("x", 240)

	raw, _ := json.Marshal(map[string]any{
		"session_id":      "sess-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input": map[string]any{
			"command": long,
		},
	})
	ev := state.DEvHook{Payload: raw, Timestamp: now}
	_, effs := d.handleHook(cs, state.FrameContext{IsRoot: true}, ev)

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
		t.Fatalf("command too long: %d", len([]rune(cmd)))
	}
	if !strings.HasSuffix(cmd, "…") {
		t.Fatalf("command = %q, want truncated suffix", cmd)
	}
}
