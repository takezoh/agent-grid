package fakeclaude

import (
	"encoding/json"

	"github.com/takezoh/agent-grid/platform/lib/claude/streamjson"
)

// Stream-json line constants — the six fixtures that shim_test.go / conformance_test.go
// and every other claude-app-server test have used since the shim was written.
// Kept as literal strings so the exact bytes an assertion sees match the exact
// bytes that a hand-rolled real-claude replay would produce.
//
// LineSystemInit uses session id "claude-sess-1" — matches the resume assertion
// in TestShim_ContinuationResume, so tests that check resume must use this literal
// or build their own via SystemInit(id).
const (
	LineSystemInit = `{"type":"system","subtype":"init","session_id":"claude-sess-1"}`
	LineAssistant  = `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`
	LineResultOK   = `{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":10,"output_tokens":5}}`
	LineResultFail = `{"type":"result","subtype":"error","result":"oops","is_error":true,"usage":{"input_tokens":1,"output_tokens":0}}`
	LineToolUse    = `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu-1","name":"Bash","input":{"command":"ls"}}]}}`
	LineToolResult = `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu-1","content":"file1.txt","is_error":false}]}}`
)

// SystemInit returns a stream-json system init line with the given session id.
// Round-trips through streamjson.Parse as SystemInit{SessionID: sessionID}.
func SystemInit(sessionID string) string {
	return marshalLine(map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": sessionID,
	})
}

// AssistantText returns a stream-json assistant line carrying a single text block.
func AssistantText(text string) string {
	return marshalLine(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": text},
			},
		},
	})
}

// ToolUse returns an assistant line carrying a single tool_use block.
// input must be JSON-marshalable; when nil, the input field is omitted.
func ToolUse(id, name string, input any) string {
	block := map[string]any{
		"type": "tool_use",
		"id":   id,
		"name": name,
	}
	if input != nil {
		block["input"] = input
	}
	return marshalLine(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{block},
		},
	})
}

// ToolResult returns a user line carrying a single tool_result block.
func ToolResult(toolUseID, content string, isError bool) string {
	return marshalLine(map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content":     content,
					"is_error":    isError,
				},
			},
		},
	})
}

// ResultOK returns a successful result line.
func ResultOK(text string, usage streamjson.Usage) string {
	return marshalLine(map[string]any{
		"type":     "result",
		"subtype":  "success",
		"result":   text,
		"is_error": false,
		"usage":    usageMap(usage),
	})
}

// ResultFail returns a failed result line.
func ResultFail(errText string, usage streamjson.Usage) string {
	return marshalLine(map[string]any{
		"type":     "result",
		"subtype":  "error",
		"result":   errText,
		"is_error": true,
		"usage":    usageMap(usage),
	})
}

func usageMap(u streamjson.Usage) map[string]any {
	m := map[string]any{
		"input_tokens":  u.InputTokens,
		"output_tokens": u.OutputTokens,
	}
	if u.TotalTokens > 0 {
		m["total_tokens"] = u.TotalTokens
	}
	return m
}

func marshalLine(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Only nil-or-cyclic values reach here; a test that exercises this branch
		// is a test bug, not a runtime concern.
		panic("fakeclaude: marshal line: " + err.Error())
	}
	return string(b)
}
