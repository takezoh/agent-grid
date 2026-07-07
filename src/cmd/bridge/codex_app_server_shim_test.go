package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

func TestInjectFrameMessagingDynamicTools(t *testing.T) {
	raw := injectFrameMessagingDynamicTools(json.RawMessage(`{"cwd":"/ws","dynamicTools":[{"name":"existing","inputSchema":{"type":"object"}}]}`))
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tools, ok := payload["dynamicTools"].([]any)
	if !ok {
		t.Fatalf("dynamicTools type = %T", payload["dynamicTools"])
	}
	if len(tools) != 5 {
		t.Fatalf("tool count = %d, want 5", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		m, ok := tool.(map[string]any)
		if !ok {
			t.Fatalf("tool entry type = %T", tool)
		}
		name, _ := m["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"existing", "agent_frames.list", "agent_frames.read", "agent_frames.send_message", "agent_frames.reply"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
	if names["agent_frames.deliver_prompt"] {
		t.Fatal("deliver_prompt must not be injected")
	}
}

func TestShouldInjectFrameMessagingTools(t *testing.T) {
	for _, tc := range []struct {
		method string
		want   bool
	}{
		{method: codexschema.MethodThreadStart, want: true},
		{method: codexschema.MethodThreadResume, want: true},
		{method: codexschema.MethodTurnStart, want: false},
	} {
		if got := shouldInjectFrameMessagingTools(tc.method); got != tc.want {
			t.Fatalf("shouldInjectFrameMessagingTools(%q) = %v, want %v", tc.method, got, tc.want)
		}
	}
}

func TestCodexShimHandleToolCall_UsesThreadScopedDaemonCommands(t *testing.T) {
	server, client := protoPipe(t)
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		expectFrameToolCall(t, server, proto.CmdFrameListByThread{SessionID: "sess-1", ThreadID: "thread-1"}, proto.RespFrameList{
			Frames: []proto.FrameRef{{SessionID: "s1", FrameID: "frame-2", Command: "codex", Sendable: true}},
		})
		expectFrameToolCall(t, server, proto.CmdFrameReadByThread{SessionID: "sess-1", ThreadID: "thread-1", PeerFrameID: "frame-2"}, proto.RespFrameRead{
			SessionID: "s1",
			Messages:  []proto.SessionMessage{{ID: "msg-1"}},
		})
		expectFrameToolCall(t, server, proto.CmdFrameSendByThread{SessionID: "sess-1", ThreadID: "thread-1", TargetFrameID: "frame-2", Body: "hello"}, proto.RespFrameSend{
			SessionID: "s1",
			Message:   proto.SessionMessage{ID: "msg-1"},
		})
		expectFrameToolCall(t, server, proto.CmdFrameReplyByThread{SessionID: "sess-1", ThreadID: "thread-1", MessageID: "msg-1", FinalAnswer: "done"}, proto.RespFrameReply{
			SessionID: "s1",
			MessageID: "msg-1",
			Reply:     proto.SessionMessageReply{ID: "reply-1"},
		})
	}()

	session := &codexShimSession{daemon: client, sessionID: "sess-1"}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"list", `{"tool":"agent_frames.list","threadId":"thread-1","arguments":{}}`},
		{"read", `{"tool":"agent_frames.read","threadId":"thread-1","arguments":{"peerFrameId":"frame-2"}}`},
		{"send", `{"tool":"agent_frames.send_message","threadId":"thread-1","arguments":{"targetFrameId":"frame-2","body":"hello"}}`},
		{"reply", `{"tool":"agent_frames.reply","threadId":"thread-1","arguments":{"messageId":"msg-1","finalAnswer":"done"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handled, reply, err := session.handleToolCall(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("handleToolCall: %v", err)
			}
			if !handled {
				t.Fatal("agent_frames tool must be handled by shim")
			}
			if !reply.Success || reply.Output == "" {
				t.Fatalf("reply = %+v", reply)
			}
		})
	}
	<-done
}

func TestCodexShimHandleToolCall_RejectsSpoofingFields(t *testing.T) {
	_, client := protoPipe(t)
	defer client.Close()
	session := &codexShimSession{daemon: client}
	_, _, err := session.handleToolCall(json.RawMessage(`{
		"tool":"agent_frames.send_message",
		"threadId":"thread-1",
		"arguments":{"targetFrameId":"frame-2","body":"hello","sourceFrameId":"spoof"}
	}`))
	if err == nil {
		t.Fatal("spoofing fields must fail strict decode")
	}
}

func TestCodexShimHandleToolCall_NonFrameToolPassesThrough(t *testing.T) {
	session := &codexShimSession{sessionID: "sess-1"}
	handled, reply, err := session.handleToolCall(json.RawMessage(`{
		"tool":"downstream.custom",
		"threadId":"thread-1",
		"arguments":{"x":1}
	}`))
	if err != nil {
		t.Fatalf("handleToolCall: %v", err)
	}
	if handled {
		t.Fatalf("handled = true, reply = %+v; want pass-through", reply)
	}
}

func protoPipe(t *testing.T) (net.Conn, *proto.Client) {
	t.Helper()
	server, clientConn := net.Pipe()
	return server, proto.DialConn(clientConn)
}

func expectFrameToolCall(t *testing.T, conn net.Conn, want proto.Command, resp proto.Response) {
	t.Helper()
	dec := json.NewDecoder(conn)
	var env proto.Envelope
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	cmd, err := proto.DecodeCommand(env)
	if err != nil {
		t.Fatalf("decode command: %v", err)
	}
	if got, wantType := cmd.CommandName(), want.CommandName(); got != wantType {
		t.Fatalf("command name = %q, want %q", got, wantType)
	}
	gotRaw, _ := json.Marshal(cmd)
	wantRaw, _ := json.Marshal(want)
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("command = %s, want %s", gotRaw, wantRaw)
	}
	wire, err := proto.EncodeResponse(env.ReqID, resp)
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if _, err := conn.Write(append(wire, '\n')); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func TestDecodeStrictJSONEmptyObject(t *testing.T) {
	var dst struct{}
	if err := decodeStrictJSON(nil, &dst); err != nil {
		t.Fatalf("decodeStrictJSON: %v", err)
	}
}

func TestParseCodexShimArgs(t *testing.T) {
	cfg, err := parseCodexShimArgs([]string{
		"--listen", "unix:///tmp/a.sock",
		"--upstream", "unix:///tmp/b.sock",
		"--server-bin", "codex",
		"--session-id", "sess-1",
		"--server-arg", "-c",
		"--server-arg", "foo=bar",
		"--sandbox-external",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.listenSock != "/tmp/a.sock" || cfg.upstreamSock != "/tmp/b.sock" || cfg.serverBin != "codex" || cfg.sessionID != "sess-1" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.serverArgs) != 2 || !cfg.sandboxExternal {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestRemoveShimSockets_RemovesAllConfiguredPaths(t *testing.T) {
	dir := t.TempDir()
	listenSock := filepath.Join(dir, "listen.sock")
	upstreamSock := filepath.Join(dir, "upstream.sock")
	for _, path := range []string{listenSock, upstreamSock} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("seed socket placeholder %q: %v", path, err)
		}
	}

	removeShimSockets(listenSock, upstreamSock)

	for _, path := range []string{listenSock, upstreamSock} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("socket placeholder %q still exists, err=%v", path, err)
		}
	}
}

func TestShimToolCallContextDeadlineSafe(t *testing.T) {
	server, client := protoPipe(t)
	defer server.Close()
	defer client.Close()
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = server.Close()
	}()
	session := &codexShimSession{daemon: client}
	_, _, _ = session.handleToolCall(json.RawMessage(`{"tool":"agent_frames.list","threadId":"thread-1","arguments":{}}`))
	_ = context.Background()
}
