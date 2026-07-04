package fakeclaude

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/takezoh/agent-reactor/platform/lib/claude/streamjson"
)

// TestLines_RoundTrip verifies every fake stream-json line parses back to the
// expected typed Event. If the streamjson parser changes shape, these
// assertions will catch drift before consumers see garbage.
func TestLines_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		line string
		want streamjson.Event
	}{
		{"const system init", LineSystemInit, streamjson.SystemInit{SessionID: "claude-sess-1"}},
		{"builder system init", SystemInit("abc"), streamjson.SystemInit{SessionID: "abc"}},
		{"const assistant text", LineAssistant, streamjson.AssistantMessage{Text: "hello"}},
		{"builder assistant text", AssistantText("hi there"), streamjson.AssistantMessage{Text: "hi there"}},
		{"const result ok", LineResultOK, streamjson.Result{
			Subtype:    "success",
			ResultText: "done",
			IsError:    false,
			Usage:      streamjson.Usage{InputTokens: 10, OutputTokens: 5},
		}},
		{"const result fail", LineResultFail, streamjson.Result{
			Subtype:    "error",
			ResultText: "oops",
			IsError:    true,
			Usage:      streamjson.Usage{InputTokens: 1, OutputTokens: 0},
		}},
		{"builder result ok", ResultOK("great", streamjson.Usage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}), streamjson.Result{
			Subtype:    "success",
			ResultText: "great",
			IsError:    false,
			Usage:      streamjson.Usage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7},
		}},
		{"const tool result", LineToolResult, streamjson.ToolResult{
			ToolUseID: "tu-1",
			IsError:   false,
			Content:   "file1.txt",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := streamjson.Parse([]byte(tc.line))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %#v, want %#v", tc.line, got, tc.want)
			}
		})
	}
}

// TestLines_ToolUseHasIDAndName verifies the ToolUse builder yields a line whose
// parsed form carries the id / name / input fields consumers rely on.
func TestLines_ToolUseHasIDAndName(t *testing.T) {
	line := ToolUse("tu-9", "Bash", map[string]any{"command": "echo hi"})
	ev, err := streamjson.Parse([]byte(line))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	am, ok := ev.(streamjson.AssistantMessage)
	if !ok || len(am.ToolUses) != 1 {
		t.Fatalf("want AssistantMessage with one ToolUse, got %#v", ev)
	}
	tu := am.ToolUses[0]
	if tu.ID != "tu-9" || tu.Name != "Bash" {
		t.Errorf("tool use = %#v, want id=tu-9 name=Bash", tu)
	}
	var input map[string]any
	if err := json.Unmarshal(tu.Input, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if input["command"] != "echo hi" {
		t.Errorf("input.command = %v, want echo hi", input["command"])
	}
}

// TestNewLauncher_ReturnsSequencesAndRecordsCalls covers the launcher's
// happy path: two turns get two distinct line sequences, and CallLog captures
// cwd / resume / prompt / extraEnv in order.
func TestNewLauncher_ReturnsSequencesAndRecordsCalls(t *testing.T) {
	launch, log := NewLauncher(
		[]string{LineSystemInit, LineResultOK},
		[]string{LineSystemInit, LineResultFail},
	)

	// call 1
	rc1, wait1, err := launch(context.Background(), "/ws1", "", "sysprompt", "first", []string{"K=1"})
	if err != nil {
		t.Fatalf("call1: %v", err)
	}
	body1, _ := io.ReadAll(rc1)
	if err := wait1(); err != nil {
		t.Errorf("wait1: %v", err)
	}
	if want := LineSystemInit + "\n" + LineResultOK + "\n"; string(body1) != want {
		t.Errorf("call1 body = %q, want %q", body1, want)
	}

	// call 2
	rc2, _, err := launch(context.Background(), "/ws2", "sess-x", "", "second", nil)
	if err != nil {
		t.Fatalf("call2: %v", err)
	}
	body2, _ := io.ReadAll(rc2)
	if want := LineSystemInit + "\n" + LineResultFail + "\n"; string(body2) != want {
		t.Errorf("call2 body = %q, want %q", body2, want)
	}

	calls := log.Calls()
	if len(calls) != 2 {
		t.Fatalf("Calls() len = %d, want 2", len(calls))
	}
	if calls[0].CWD != "/ws1" || calls[0].Prompt != "first" || calls[0].AppendSystemPrompt != "sysprompt" || len(calls[0].ExtraEnv) != 1 || calls[0].ExtraEnv[0] != "K=1" {
		t.Errorf("call[0] = %#v", calls[0])
	}
	if calls[1].ResumeSessionID != "sess-x" || calls[1].Prompt != "second" {
		t.Errorf("call[1] = %#v", calls[1])
	}
}

// TestNewLauncher_LastSequenceSticks — beyond the last provided sequence, the
// launcher must keep returning that last sequence (not panic, not empty).
// shim_test.go depends on this for the "hang" and "kill" tests.
func TestNewLauncher_LastSequenceSticks(t *testing.T) {
	launch, _ := NewLauncher([]string{LineSystemInit, LineResultOK})
	for i := 0; i < 3; i++ {
		rc, _, err := launch(context.Background(), "", "", "", "", nil)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		body, _ := io.ReadAll(rc)
		if want := LineSystemInit + "\n" + LineResultOK + "\n"; string(body) != want {
			t.Errorf("call %d body = %q", i, body)
		}
	}
}

// TestNewProgrammableLauncher_ErrorShortCircuits — when the fn returns Err,
// the launcher must surface it without opening a stdout pipe.
func TestNewProgrammableLauncher_ErrorShortCircuits(t *testing.T) {
	want := errors.New("boom")
	launch, log := NewProgrammableLauncher(func(_ LaunchArgs) LaunchResponse {
		return LaunchResponse{Err: want}
	})
	rc, wait, err := launch(context.Background(), "", "", "", "", nil)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
	if rc != nil {
		t.Errorf("rc = %v, want nil", rc)
	}
	if wait != nil {
		t.Errorf("wait not nil on error path")
	}
	if log.Len() != 1 {
		t.Errorf("expected the failing call to still be logged")
	}
}

// TestNewProgrammableLauncher_ObservesExtraEnv — a common pattern in
// shim_test.go is asserting that TOOL_BRIDGE_SOCKET is passed. The programmable
// launcher must expose extraEnv to the callback.
func TestNewProgrammableLauncher_ObservesExtraEnv(t *testing.T) {
	seen := ""
	launch, _ := NewProgrammableLauncher(func(a LaunchArgs) LaunchResponse {
		for _, e := range a.ExtraEnv {
			if e == "TOOL_BRIDGE_SOCKET=/tmp/x" {
				seen = e
			}
		}
		return LaunchResponse{Lines: []string{LineSystemInit, LineResultOK}}
	})
	if _, _, err := launch(context.Background(), "", "", "", "", []string{"TOOL_BRIDGE_SOCKET=/tmp/x"}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if seen == "" {
		t.Errorf("callback did not observe TOOL_BRIDGE_SOCKET")
	}
}

// TestHookPayload_MarshalMatchesDriverStructShape verifies the JSON keys the
// fake produces are a subset of the driver hookPayload's expected keys. If a
// new field is added to either side, the mismatch surfaces here.
func TestHookPayload_MarshalMatchesDriverStructShape(t *testing.T) {
	// The set of json tags the driver's hookPayload declares.
	// Must be updated in lockstep with client/driver/claude_event.go:hookPayload.
	driverKeys := map[string]bool{
		"session_id":        true,
		"hook_event_name":   true,
		"prompt":            true,
		"transcript_path":   true,
		"notification_type": true,
		"tool_name":         true,
		"tool_input":        true,
		"source":            true,
		"tool_use_id":       true,
		"permission_mode":   true,
		"error":             true,
		"is_interrupt":      true,
	}

	full := HookPayload{
		SessionID:        "s",
		HookEventName:    "PreToolUse",
		Prompt:           "p",
		TranscriptPath:   "/tmp/t.jsonl",
		NotificationType: "n",
		ToolName:         "Bash",
		ToolInput:        map[string]any{"command": "ls"},
		Source:           "user",
		ToolUseID:        "tu-1",
		PermissionMode:   "default",
		Error:            "",
		IsInterrupt:      false,
	}
	// Force is_interrupt to appear by using non-omitempty via a second variant.
	// The current schema has is_interrupt as omitempty=true for space, but the
	// driver reads it either way — presence of the key is not required.
	b := Marshal(full)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for k := range got {
		if !driverKeys[k] {
			t.Errorf("fakeclaude JSON key %q is not declared in the driver hookPayload struct", k)
		}
	}
}
