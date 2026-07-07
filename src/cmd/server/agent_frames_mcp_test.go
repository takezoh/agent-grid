package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
)

type frameMCPTestServer struct {
	listener net.Listener
	commands chan proto.Command
}

func startFrameMCPTestServer(t *testing.T) (*frameMCPTestServer, string) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "frame-mcp.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &frameMCPTestServer{
		listener: ln,
		commands: make(chan proto.Command, 8),
	}
	go s.serve(t)
	t.Cleanup(func() { _ = s.listener.Close() })
	return s, sock
}

func (s *frameMCPTestServer) serve(t *testing.T) {
	t.Helper()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(t, conn)
	}
}

func (s *frameMCPTestServer) handleConn(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var env proto.Envelope
		if err := dec.Decode(&env); err != nil {
			return
		}
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			t.Errorf("decode command: %v", err)
			return
		}
		s.commands <- cmd

		var resp proto.Response
		switch c := cmd.(type) {
		case proto.CmdFrameList:
			resp = proto.RespFrameList{
				Frames: []proto.FrameRef{{SessionID: "s1", FrameID: "frame-codex", Command: "codex", Sendable: true}},
			}
		case proto.CmdFrameRead:
			resp = proto.RespFrameRead{
				SessionID: "s1",
				Messages:  []proto.SessionMessage{{ID: "msg-1", SourceFrameID: c.PeerFrameID, TargetFrameID: "frame-claude"}},
			}
		case proto.CmdFrameSend:
			resp = proto.RespFrameSend{
				SessionID: "s1",
				Message:   proto.SessionMessage{ID: "msg-1", SourceFrameID: "frame-claude", TargetFrameID: c.TargetFrameID, Body: c.Body},
			}
		case proto.CmdFrameReply:
			resp = proto.RespFrameReply{
				SessionID: "s1",
				MessageID: c.MessageID,
				Reply:     proto.SessionMessageReply{ID: "reply-1", SourceFrameID: "frame-codex", FinalAnswer: c.FinalAnswer},
			}
		default:
			t.Errorf("unexpected command type %T", cmd)
			return
		}

		wire, err := proto.EncodeResponse(env.ReqID, resp)
		if err != nil {
			t.Errorf("encode response: %v", err)
			return
		}
		if err := enc.Encode(json.RawMessage(wire)); err != nil {
			t.Errorf("write response: %v", err)
			return
		}
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
		if _, ok := tool.InputSchema["additionalProperties"]; !ok {
			t.Fatalf("tool %q missing additionalProperties=false", tool.Name)
		}
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
	if props, ok := names["agent_frames.send_message"].InputSchema["properties"].(map[string]any); ok {
		if _, bad := props["sourceFrameId"]; bad {
			t.Fatal("sourceFrameId must not appear in send_message schema")
		}
		if _, bad := props["sourceSessionId"]; bad {
			t.Fatal("sourceSessionId must not appear in send_message schema")
		}
	}
}

func TestHandleMCPToolCall_UsesBoundTokenAndRejectsSpoofingFields(t *testing.T) {
	srv, sock := startFrameMCPTestServer(t)
	client, err := proto.Dial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name      string
		tool      string
		args      string
		assertCmd func(t *testing.T, cmd proto.Command)
	}{
		{
			name: "list",
			tool: "agent_frames.list",
			args: `{}`,
			assertCmd: func(t *testing.T, cmd proto.Command) {
				got, ok := cmd.(proto.CmdFrameList)
				if !ok {
					t.Fatalf("command type = %T, want CmdFrameList", cmd)
				}
				if got.Token != "bound-token" {
					t.Fatalf("token = %q, want bound-token", got.Token)
				}
			},
		},
		{
			name: "read",
			tool: "agent_frames.read",
			args: `{"peerFrameId":"frame-codex"}`,
			assertCmd: func(t *testing.T, cmd proto.Command) {
				got, ok := cmd.(proto.CmdFrameRead)
				if !ok {
					t.Fatalf("command type = %T, want CmdFrameRead", cmd)
				}
				if got.Token != "bound-token" || got.PeerFrameID != "frame-codex" {
					t.Fatalf("read cmd = %+v", got)
				}
			},
		},
		{
			name: "send",
			tool: "agent_frames.send_message",
			args: `{"targetFrameId":"frame-codex","body":"hello","priority":"normal"}`,
			assertCmd: func(t *testing.T, cmd proto.Command) {
				got, ok := cmd.(proto.CmdFrameSend)
				if !ok {
					t.Fatalf("command type = %T, want CmdFrameSend", cmd)
				}
				if got.Token != "bound-token" || got.TargetFrameID != "frame-codex" || got.Body != "hello" {
					t.Fatalf("send cmd = %+v", got)
				}
			},
		},
		{
			name: "reply",
			tool: "agent_frames.reply",
			args: `{"messageId":"msg-1","finalAnswer":"done","confidence":"high"}`,
			assertCmd: func(t *testing.T, cmd proto.Command) {
				got, ok := cmd.(proto.CmdFrameReply)
				if !ok {
					t.Fatalf("command type = %T, want CmdFrameReply", cmd)
				}
				if got.Token != "bound-token" || got.MessageID != "msg-1" || got.FinalAnswer != "done" {
					t.Fatalf("reply cmd = %+v", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := handleMCPMessage(client, "bound-token", mcpEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  "tools/call",
				Params: mustJSON(t, mcpToolCallParams{
					Name:      tc.tool,
					Arguments: json.RawMessage(tc.args),
				}),
			})
			if resp == nil {
				t.Fatal("tools/call returned nil")
			}
			tc.assertCmd(t, waitFrameMCPCommand(t, srv.commands))
		})
	}

	spoof := handleMCPMessage(client, "bound-token", mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params: mustJSON(t, mcpToolCallParams{
			Name: "agent_frames.send_message",
			Arguments: json.RawMessage(`{
				"targetFrameId":"frame-codex",
				"body":"hello",
				"sourceFrameId":"spoofed"
			}`),
		}),
	})
	if spoof == nil {
		t.Fatal("spoofing call returned nil")
	}
	result, ok := spoof.Result.(mcpToolResult)
	if !ok {
		t.Fatalf("spoof result type = %T", spoof.Result)
	}
	if !result.IsError {
		t.Fatal("spoofing call must fail")
	}
	select {
	case cmd := <-srv.commands:
		t.Fatalf("spoofing request must be rejected before broker call, got %T", cmd)
	case <-time.After(100 * time.Millisecond):
	}
}

func waitFrameMCPCommand(t *testing.T, ch <-chan proto.Command) proto.Command {
	t.Helper()
	select {
	case cmd := <-ch:
		return cmd
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broker command")
		return nil
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestRunAgentFramesMCP_RequiresToken(t *testing.T) {
	err := runAgentFramesMCP([]string{"--sock", filepath.Join(t.TempDir(), "missing.sock")})
	if err == nil || err.Error() != "AG_SOCKET_TOKEN is required" {
		t.Fatalf("err = %v, want missing token", err)
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
	if string(msg.ID) != "1" {
		t.Fatalf("id = %s, want 1", string(msg.ID))
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
	raw := out.String()
	if !strings.HasPrefix(raw, "Content-Length: ") {
		t.Fatalf("missing Content-Length header: %q", raw)
	}
	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("missing header separator: %q", raw)
	}
	body := parts[1]
	var msg mcpEnvelope
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if msg.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want 2.0", msg.JSONRPC)
	}
}

func TestReadMCPContentLength_MissingHeader(t *testing.T) {
	_, err := readMCPContentLength(bufio.NewReader(strings.NewReader("\r\n")))
	if err == nil || !strings.Contains(err.Error(), "missing Content-Length") {
		t.Fatalf("err = %v, want missing Content-Length", err)
	}
}

func TestHandleMCPToolCall_UnknownTool(t *testing.T) {
	resp := handleMCPMessage(nil, "", mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  mustJSON(t, mcpToolCallParams{Name: "agent_frames.unknown"}),
	})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("unknown tool response = %+v", resp)
	}
}

func TestHandleMCPToolCall_InvalidParams(t *testing.T) {
	resp := handleMCPMessage(nil, "", mcpEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":1}`),
	})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("invalid params response = %+v", resp)
	}
}
