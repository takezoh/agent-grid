package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRunAgentFramesMCP_RequiresToken(t *testing.T) {
	err := runAgentFramesMCP([]string{"--sock", filepath.Join(t.TempDir(), "missing.sock")})
	if err == nil || err.Error() != "AG_SOCKET_TOKEN is required" {
		t.Fatalf("err = %v, want missing token", err)
	}
}

func TestHandleMCPMessage_ListRejectsUnknownArguments(t *testing.T) {
	resp := handleMCPMessage(nil, "", mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params: mustJSONBridge(t, mcpToolCallParams{
			Name:      "agent_frames.list",
			Arguments: json.RawMessage(`{"sourceFrameId":"spoof"}`),
		}),
	})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("invalid params response = %+v", resp)
	}
}

func TestHandleMCPMessage_ToolsListExposesPhase1Only(t *testing.T) {
	resp := handleMCPMessage(nil, "", mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})
	if resp == nil {
		t.Fatal("tools/list returned nil")
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T", resp.Result)
	}
	rawTools, ok := result["tools"].([]mcpTool)
	if !ok {
		t.Fatalf("tools type = %T", result["tools"])
	}
	if len(rawTools) != 4 {
		t.Fatalf("tool count = %d, want 4", len(rawTools))
	}
	names := map[string]mcpTool{}
	for _, tool := range rawTools {
		names[tool.Name] = tool
	}
	for _, name := range []string{
		"agent_frames.list",
		"agent_frames.read",
		"agent_frames.send_message",
		"agent_frames.reply",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("missing tool %q", name)
		}
	}
	if _, ok := names["agent_frames.deliver_prompt"]; ok {
		t.Fatal("deliver_prompt must not be exposed in phase 1")
	}
}

func TestMCPStdioStream_ReadParsesContentLengthFrames(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	stream := mcpStdioStream{
		r: bufio.NewReader(strings.NewReader("Content-Length: " + strconv.Itoa(len(body)) + "\r\nX-Test: ok\r\n\r\n" + body)),
	}
	msg, err := stream.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.Method != "tools/list" {
		t.Fatalf("method = %q, want tools/list", msg.Method)
	}
}

func TestMCPStdioStream_WriteUsesContentLengthFrames(t *testing.T) {
	var out bytes.Buffer
	stream := mcpStdioStream{w: bufio.NewWriter(&out)}
	if err := stream.Write(mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasPrefix(out.String(), "Content-Length: ") {
		t.Fatalf("missing Content-Length header: %q", out.String())
	}
}

func mustJSONBridge(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
