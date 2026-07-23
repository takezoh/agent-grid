package stream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/host/state"
)

func TestExtractThreadID(t *testing.T) {
	if got := extractThreadID([]byte(`{"threadId":"t1"}`)); got != "t1" {
		t.Errorf("flat: %q", got)
	}
	if got := extractThreadID([]byte(`{"thread":{"id":"t2"}}`)); got != "t2" {
		t.Errorf("nested: %q", got)
	}
	if got := extractThreadID([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
	if got := extractThreadID([]byte(`{}`)); got != "" {
		t.Errorf("empty: %q", got)
	}
}

func TestExtractTurnID(t *testing.T) {
	if got := extractTurnID([]byte(`{"turnId":"tu1"}`)); got != "tu1" {
		t.Errorf("flat: %q", got)
	}
	if got := extractTurnID([]byte(`{"turn":{"id":"tu2"}}`)); got != "tu2" {
		t.Errorf("nested: %q", got)
	}
	if got := extractTurnID([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestExtractTurnPrompt(t *testing.T) {
	raw := []byte(`{
		"threadId":"t1",
		"turn":{
			"id":"tu1",
			"items":[
				{"id":"u1","type":"userMessage","content":[
					" first line ",
					{"type":"text","text":" diagnose the app "},
					{"type":"image","url":"https://example.test/image.png"},
					{"type":"text","text":"include logs"}
				]},
				{"id":"a1","type":"agentMessage","text":"ok"}
			],
			"status":"inProgress"
		}
	}`)
	if got := extractTurnPrompt(raw); got != "first line\ndiagnose the app\ninclude logs" {
		t.Errorf("prompt: %q", got)
	}
	if got := extractTurnPrompt([]byte(`{"turn":{"items":[]}}`)); got != "" {
		t.Errorf("empty items: %q", got)
	}
	if got := extractTurnPrompt([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestNormalizeCodexThreadMetadata(t *testing.T) {
	raw := []byte(`{
		"thread":{"id":"t1","name":" saved\nsession ","preview":" first\npreview "},
		"turn":{"items":[{"type":"userMessage","content":[" diagnose ",{"type":"text","text":" app "}]}]}
	}`)
	got := normalizeCodexThreadMetadata(raw)
	if got.threadID != "t1" || got.title != "saved session" || !got.titleSet || got.preview != "first preview" || got.prompt != "diagnose app" {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestNormalizeCodexThreadMetadataTopLevelFields(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want codexThreadMetadata
	}{
		{
			name: "threadName title",
			raw:  `{"threadId":"t1","threadName":" named "}`,
			want: codexThreadMetadata{threadID: "t1", title: "named", titleSet: true},
		},
		{
			name: "name title",
			raw:  `{"threadId":"t1","name":" named "}`,
			want: codexThreadMetadata{threadID: "t1", title: "named", titleSet: true},
		},
		{
			name: "preview",
			raw:  `{"threadId":"t1","preview":" preview text "}`,
			want: codexThreadMetadata{threadID: "t1", preview: "preview text"},
		},
		{
			name: "string content prompt",
			raw:  `{"threadId":"t1","turn":{"items":[{"type":"userMessage","content":" prompt text "}]}}`,
			want: codexThreadMetadata{threadID: "t1", prompt: "prompt text"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCodexThreadMetadata([]byte(tc.raw))
			if got != tc.want {
				t.Fatalf("metadata = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNormalizeCodexThreadMetadataTitleClear(t *testing.T) {
	for _, raw := range []string{
		`{"threadId":"t1","threadName":null}`,
		`{"threadId":"t1","threadName":""}`,
		`{"thread":{"id":"t1","name":null}}`,
	} {
		got := normalizeCodexThreadMetadata([]byte(raw))
		if got.threadID != "t1" || !got.titleSet || got.title != "" {
			t.Fatalf("metadata = %+v, raw=%s", got, raw)
		}
	}
}

func TestNormalizeCodexThreadSettings(t *testing.T) {
	got := normalizeCodexThreadSettings([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":{"level":"high"}}}`))
	if got.threadID != "t1" || got.model != "gpt-5" || !got.modelSet || got.effort != "high" || !got.effortSet {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestNormalizeCodexThreadSettingsNullEffort(t *testing.T) {
	got := normalizeCodexThreadSettings([]byte(`{"threadId":"t1","threadSettings":{"effort":null}}`))
	if got.threadID != "t1" || !got.effortSet || got.effort != "" {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestNormalizeCodexThreadSettingsReasoningEffortAlias(t *testing.T) {
	got := normalizeCodexThreadSettings([]byte(`{"threadId":"t1","threadSettings":{"reasoning_effort":{"level":"low"}}}`))
	if got.threadID != "t1" || !got.effortSet || got.effort != "low" {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestNormalizeCodexThreadSettingsReasoningEffortAliasOverridesNullEffort(t *testing.T) {
	got := normalizeCodexThreadSettings([]byte(`{"threadId":"t1","threadSettings":{"effort":null,"reasoning_effort":{"level":"high"}}}`))
	if got.threadID != "t1" || !got.effortSet || got.effort != "high" {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestExtractText(t *testing.T) {
	if got := extractText([]byte(`{"text":"hi"}`)); got != "hi" {
		t.Errorf("text: %q", got)
	}
	if got := extractText([]byte(`{"delta":"d"}`)); got != "d" {
		t.Errorf("delta: %q", got)
	}
	if got := extractText([]byte(`{"item":{"text":"ti"}}`)); got != "ti" {
		t.Errorf("item.text: %q", got)
	}
	if got := extractText([]byte(`{"item":{"content":"c"}}`)); got != "c" {
		t.Errorf("item.content: %q", got)
	}
	if got := extractText([]byte(`{"turn":{"items":[{"type":"agentMessage","text":"from turn"}]}}`)); got != "from turn" {
		t.Errorf("turn.items.text: %q", got)
	}
	if got := extractText([]byte(`{"turn":{"items":[{"type":"agentMessage","content":[{"type":"text","text":"from content"}]}]}}`)); got != "from content" {
		t.Errorf("turn.items.content: %q", got)
	}
	if got := extractText([]byte(`{"turn":{"items":[{"type":"agentMessage","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}]}}`)); got != "hello world" {
		t.Errorf("turn.items.fragments: %q", got)
	}
	if got := extractText([]byte(`{"turn":{"items":[{"type":"agentMessage","phase":"commentary","text":"draft"},{"type":"agentMessage","phase":"final_answer","text":"final"}]}}`)); got != "final" {
		t.Errorf("turn.items.phase: %q", got)
	}
	if got := extractText([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestNestedString(t *testing.T) {
	if got := nestedString([]byte(`{"x":"a"}`), "x"); got != "a" {
		t.Errorf("flat: %q", got)
	}
	if got := nestedString([]byte(`{"item":{"x":"b"}}`), "x"); got != "b" {
		t.Errorf("nested: %q", got)
	}
	if got := nestedString([]byte(`bad`), "x"); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestExtractThreadStatus(t *testing.T) {
	raw := []byte(`{"threadId":"t1","status":{"type":"active","activeFlags":["waitingOnApproval"]}}`)
	st, wa, tid := extractThreadStatus(raw)
	if st != "active" || !wa || tid != "t1" {
		t.Errorf("got st=%q wa=%v tid=%q", st, wa, tid)
	}
	raw2 := []byte(`{"thread":{"id":"t2","status":{"type":"idle"}}}`)
	st, wa, tid = extractThreadStatus(raw2)
	if st != "idle" || wa || tid != "t2" {
		t.Errorf("nested: %q %v %q", st, wa, tid)
	}
	// invalid JSON must not panic
	_, _, _ = extractThreadStatus([]byte(`bad`))
	st, _, _ = extractThreadStatus([]byte(`{"threadId":"t"}`))
	if st != "" {
		t.Errorf("no status: %q", st)
	}
}

func TestThreadStatusEventsActive(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active"}}`)
	out, st, wa := threadStatusEvents(raw, "t", "", false)
	if st != "active" || wa {
		t.Errorf("st=%q wa=%v", st, wa)
	}
	if len(out) != 1 || out[0].kind != state.SubsystemTurnStarted {
		t.Errorf("expected TurnStarted, got %+v", out)
	}
}

func TestThreadStatusEventsActiveApproval(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active","activeFlags":["waitingOnApproval"]}}`)
	out, _, _ := threadStatusEvents(raw, "t", "idle", false)
	hasTurnStart, hasApproval := false, false
	for _, e := range out {
		if e.kind == state.SubsystemTurnStarted {
			hasTurnStart = true
		}
		if e.kind == state.SubsystemApprovalRequested {
			hasApproval = true
		}
	}
	if !hasTurnStart || !hasApproval {
		t.Errorf("missing events: %+v", out)
	}
}

func TestThreadStatusEventsApprovalResolved(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active"}}`)
	out, _, _ := threadStatusEvents(raw, "t", "active", true)
	found := false
	for _, e := range out {
		if e.kind == state.SubsystemApprovalResolved {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ApprovalResolved, got %+v", out)
	}
}

func TestThreadStatusEventsIdle(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"idle"}}`)
	out, _, _ := threadStatusEvents(raw, "t", "active", false)
	if len(out) != 1 || out[0].kind != state.SubsystemTurnCompleted {
		t.Errorf("expected TurnCompleted, got %+v", out)
	}
}

func TestThreadStatusEventsForeignThread(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"other","status":{"type":"active"}}`)
	out, _, _ := threadStatusEvents(raw, "self", "", false)
	if out != nil {
		t.Errorf("foreign thread should be filtered, got %+v", out)
	}
}

func TestItemLifecycleEvents(t *testing.T) {
	startCmd := json.RawMessage(`{"item":{"type":"commandExecution","itemId":"i1","command":"ls","cwd":"/p"}}`)
	out := itemLifecycleEvents("item/started", startCmd, "t")
	if len(out) != 1 || out[0].kind != state.SubsystemToolStarted || out[0].payload.Tool.Name != "command" {
		t.Errorf("cmd start: %+v", out)
	}
	endCmd := json.RawMessage(`{"item":{"type":"commandExecution","itemId":"i1","command":"ls","error":"oops"}}`)
	out = itemLifecycleEvents("item/completed", endCmd, "t")
	if len(out) != 1 || out[0].kind != state.SubsystemToolCompleted || out[0].payload.Tool.Error != "oops" {
		t.Errorf("cmd end: %+v", out)
	}
	fileStart := json.RawMessage(`{"item":{"type":"fileChange","itemId":"i2","path":"/x"}}`)
	out = itemLifecycleEvents("item/started", fileStart, "t")
	if len(out) != 1 || out[0].payload.Tool.Name != "file_change" {
		t.Errorf("file start: %+v", out)
	}
	out = itemLifecycleEvents("item/completed", fileStart, "t")
	if len(out) != 1 {
		t.Errorf("file end: %+v", out)
	}
	if out := itemLifecycleEvents("unknown", fileStart, "t"); out != nil {
		t.Errorf("unknown method should return nil: %+v", out)
	}
}

func TestSummarizePlan(t *testing.T) {
	if got := summarizePlan([]byte(`{"summary":"s"}`)); got != "s" {
		t.Errorf("summary: %q", got)
	}
	got := summarizePlan([]byte(`{"items":[{"step":"a","status":"done"},{"step":"b","status":"pending"}]}`))
	if !strings.Contains(got, "a done") || !strings.Contains(got, "b pending") {
		t.Errorf("items: %q", got)
	}
	if summarizePlan([]byte(`bad`)) != "" {
		t.Errorf("bad")
	}
}

func TestSummarizeDiff(t *testing.T) {
	got := summarizeDiff([]byte(`{"paths":["a","b"]}`))
	if got != "a, b" {
		t.Errorf("got %q", got)
	}
	if summarizeDiff([]byte(`{}`)) != "" {
		t.Errorf("empty")
	}
	if summarizeDiff([]byte(`bad`)) != "" {
		t.Errorf("bad")
	}
}

func TestApprovalFromParams(t *testing.T) {
	a := approvalFromParams("item/commandExecution/requestApproval", []byte(`{"itemId":"i","command":"c","reason":"r"}`), true)
	if a.Kind != "command" || a.Command != "c" || !a.AutoApprove {
		t.Errorf("%+v", a)
	}
	b := approvalFromParams("item/fileChange/requestApproval", []byte(`{"path":"/p"}`), false)
	if b.Kind != "file_change" || b.Path != "/p" {
		t.Errorf("%+v", b)
	}
}

func TestAppendHistory(t *testing.T) {
	var h []state.SubsystemTurn
	for range 10 {
		appendHistory(&h, "user", "msg")
	}
	if len(h) != 6 {
		t.Errorf("len = %d", len(h))
	}
	appendHistory(&h, "user", "   ")
	if len(h) != 6 {
		t.Errorf("whitespace should be ignored")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Errorf("got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("all empty: %q", got)
	}
}

func TestItemType(t *testing.T) {
	if got := itemType([]byte(`{"item":{"type":"x"}}`)); got != "x" {
		t.Errorf("got %q", got)
	}
}
