package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

var responsesDynamicToolNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

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
	for _, want := range []string{"existing", "agent_frames_list", "agent_frames_read", "agent_frames_send_message", "agent_frames_reply"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
	if names["agent_frames.deliver_prompt"] {
		t.Fatal("deliver_prompt must not be injected")
	}
}

func TestCodexShimDownstreamThreadStart_InjectsResponsesCompatibleDynamicToolNames(t *testing.T) {
	downstreamPeer, upstreamPeer, run := newShimTestSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run(ctx)

	req := `{"id":"thread-start","method":"thread/start","params":{"cwd":"/ws","dynamicTools":[{"name":"existing_tool","inputSchema":{"type":"object"}}]}}`
	if err := downstreamPeer.WriteMessage(ctx, []byte(req)); err != nil {
		t.Fatalf("write downstream thread/start: %v", err)
	}

	fwd, err := upstreamPeer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read forwarded upstream request: %v", err)
	}
	var env wireEnvelope
	if err := json.Unmarshal(fwd, &env); err != nil {
		t.Fatalf("unmarshal forwarded request: %v", err)
	}
	if env.Method != codexschema.MethodThreadStart {
		t.Fatalf("forwarded method = %q, want %q", env.Method, codexschema.MethodThreadStart)
	}
	var params struct {
		DynamicTools []struct {
			Name string `json:"name"`
		} `json:"dynamicTools"`
	}
	if err := json.Unmarshal(env.Params, &params); err != nil {
		t.Fatalf("unmarshal forwarded params: %v", err)
	}
	if len(params.DynamicTools) != 5 {
		t.Fatalf("dynamicTools count = %d, want 5", len(params.DynamicTools))
	}
	names := map[string]bool{}
	for _, tool := range params.DynamicTools {
		names[tool.Name] = true
		if !responsesDynamicToolNameRE.MatchString(tool.Name) {
			t.Fatalf("dynamic tool name %q violates Responses API regex", tool.Name)
		}
	}
	for _, want := range []string{
		"existing_tool",
		"agent_frames_list",
		"agent_frames_read",
		"agent_frames_send_message",
		"agent_frames_reply",
	} {
		if !names[want] {
			t.Fatalf("missing injected tool %q", want)
		}
	}
	for _, forbidden := range []string{
		"agent_frames.list",
		"agent_frames.read",
		"agent_frames.send_message",
		"agent_frames.reply",
	} {
		if names[forbidden] {
			t.Fatalf("canonical MCP tool name %q leaked into forwarded thread/start", forbidden)
		}
	}

	reply, err := json.Marshal(wireEnvelope{ID: env.ID, Result: json.RawMessage(`{"threadId":"thread-1","sessionId":"thread-1","path":"/tmp/thread-1"}`)})
	if err != nil {
		t.Fatalf("marshal upstream reply: %v", err)
	}
	if err := upstreamPeer.WriteMessage(ctx, reply); err != nil {
		t.Fatalf("write upstream reply: %v", err)
	}
	if _, err := downstreamPeer.ReadMessage(ctx); err != nil {
		t.Fatalf("read downstream reply: %v", err)
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
		{"list", `{"tool":"agent_frames_list","threadId":"thread-1","arguments":{}}`},
		{"read", `{"tool":"agent_frames_read","threadId":"thread-1","arguments":{"peerFrameId":"frame-2"}}`},
		{"send", `{"tool":"agent_frames_send_message","threadId":"thread-1","arguments":{"targetFrameId":"frame-2","body":"hello"}}`},
		{"reply", `{"tool":"agent_frames_reply","threadId":"thread-1","arguments":{"messageId":"msg-1","finalAnswer":"done"}}`},
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

func TestCodexShimHandleToolCall_LegacyCanonicalNamesStillWork(t *testing.T) {
	server, client := protoPipe(t)
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		expectFrameToolCall(t, server, proto.CmdFrameListByThread{SessionID: "sess-1", ThreadID: "thread-1"}, proto.RespFrameList{})
	}()

	session := &codexShimSession{daemon: client, sessionID: "sess-1"}
	handled, _, err := session.handleToolCall(json.RawMessage(`{"tool":"agent_frames.list","threadId":"thread-1","arguments":{}}`))
	if err != nil {
		t.Fatalf("handleToolCall: %v", err)
	}
	if !handled {
		t.Fatal("legacy canonical frame tool name must still be handled")
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

// pipeTransportPair wires two in-process transports back-to-back so a test
// can play the role of one side of a codexclient.Conn while the shim owns
// the other.
func pipeTransportPair() (codexclient.Transport, codexclient.Transport) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return codexclient.StdioTransport(pr1, pw2), codexclient.StdioTransport(pr2, pw1)
}

// wireEnvelope is a minimal JSON-RPC 2.0 envelope for tests to inspect/build
// wire messages directly, preserving the "id" member's raw bytes rather than
// decoding it into a fixed Go type.
type wireEnvelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// newShimTestSession wires a codexShimSession's downstream/upstream Conns to
// in-process transports, returning the peer-facing Transport halves so a
// test can play the downstream CLI on one side and the real upstream
// codex-app-server on the other. Call run(ctx) to start both read loops.
func newShimTestSession(t *testing.T) (downstreamPeer, upstreamPeer codexclient.Transport, run func(ctx context.Context)) {
	t.Helper()
	downstreamSelf, downstreamPeerT := pipeTransportPair()
	upstreamSelf, upstreamPeerT := pipeTransportPair()
	session := &codexShimSession{
		downstream: codexclient.NewConn(downstreamSelf, 5*time.Second),
		upstream:   codexclient.NewConn(upstreamSelf, 5*time.Second),
	}
	run = func(ctx context.Context) {
		go func() { _ = session.downstream.Run(ctx, shimDownstreamHandler{session: session}) }()
		go func() { _ = session.upstream.Run(ctx, shimUpstreamHandler{session: session}) }()
	}
	return downstreamPeerT, upstreamPeerT, run
}

// TestCodexShimDownstreamToUpstream_PreservesStringIDBytes pins AC-001: a
// downstream-initiated request (e.g. codex-cli's "initialize") carrying a
// JSON string id must be forwarded upstream under a freshly minted numeric
// id (the upstream Conn's own numbering), and the upstream reply must come
// back to the downstream peer with the ORIGINAL string id bytes preserved
// verbatim — not coerced to a number or renumbered — so codex-cli 0.142.5's
// initialize round trip resolves instead of hitting its 10s timeout.
func TestCodexShimDownstreamToUpstream_PreservesStringIDBytes(t *testing.T) {
	downstreamPeer, upstreamPeer, run := newShimTestSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run(ctx)

	if err := downstreamPeer.WriteMessage(ctx, []byte(`{"id":"abc","method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("write downstream request: %v", err)
	}

	fwd, err := upstreamPeer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read forwarded upstream request: %v", err)
	}
	var fwdEnv wireEnvelope
	if err := json.Unmarshal(fwd, &fwdEnv); err != nil {
		t.Fatalf("unmarshal forwarded request: %v", err)
	}
	if fwdEnv.Method != "initialize" {
		t.Fatalf("forwarded method = %q, want initialize", fwdEnv.Method)
	}
	if _, err := strconv.ParseInt(string(fwdEnv.ID), 10, 64); err != nil {
		t.Fatalf("forwarded id %s is not a freshly minted decimal numeric id: %v", fwdEnv.ID, err)
	}

	reply, err := json.Marshal(wireEnvelope{ID: fwdEnv.ID, Result: json.RawMessage(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("marshal upstream reply: %v", err)
	}
	if err := upstreamPeer.WriteMessage(ctx, reply); err != nil {
		t.Fatalf("write upstream reply: %v", err)
	}

	got, err := downstreamPeer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read downstream reply: %v", err)
	}
	var gotEnv wireEnvelope
	if err := json.Unmarshal(got, &gotEnv); err != nil {
		t.Fatalf("unmarshal downstream reply: %v", err)
	}
	if string(gotEnv.ID) != `"abc"` {
		t.Fatalf("downstream reply id = %s, want original string id bytes \"abc\" preserved verbatim", gotEnv.ID)
	}
	var result map[string]any
	if err := json.Unmarshal(gotEnv.Result, &result); err != nil {
		t.Fatalf("unmarshal downstream reply result: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("downstream reply result = %v, want ok=true", result)
	}
}

// TestCodexShimUpstreamToDownstream_RenumbersAndEchoesID pins AC-004: an
// upstream server-initiated request must be forwarded downstream under a
// freshly minted numeric id (the downstream Conn's own numbering, distinct
// from whatever id shape the real codex-app-server used), and the
// downstream reply must be echoed back to the upstream peer under the
// ORIGINAL upstream request id bytes.
func TestCodexShimUpstreamToDownstream_RenumbersAndEchoesID(t *testing.T) {
	downstreamPeer, upstreamPeer, run := newShimTestSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run(ctx)

	if err := upstreamPeer.WriteMessage(ctx, []byte(`{"id":7,"method":"session/request_permission","params":{"q":1}}`)); err != nil {
		t.Fatalf("write upstream server-initiated request: %v", err)
	}

	fwd, err := downstreamPeer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read forwarded downstream request: %v", err)
	}
	var fwdEnv wireEnvelope
	if err := json.Unmarshal(fwd, &fwdEnv); err != nil {
		t.Fatalf("unmarshal forwarded request: %v", err)
	}
	if fwdEnv.Method != "session/request_permission" {
		t.Fatalf("forwarded method = %q, want session/request_permission", fwdEnv.Method)
	}
	if string(fwdEnv.ID) == "7" {
		t.Fatalf("forwarded id = %s, want a freshly minted downstream numeric id, not the original upstream id 7", fwdEnv.ID)
	}
	if _, err := strconv.ParseInt(string(fwdEnv.ID), 10, 64); err != nil {
		t.Fatalf("forwarded id %s is not a decimal numeric id: %v", fwdEnv.ID, err)
	}

	reply, err := json.Marshal(wireEnvelope{ID: fwdEnv.ID, Result: json.RawMessage(`{"granted":true}`)})
	if err != nil {
		t.Fatalf("marshal downstream reply: %v", err)
	}
	if err := downstreamPeer.WriteMessage(ctx, reply); err != nil {
		t.Fatalf("write downstream reply: %v", err)
	}

	got, err := upstreamPeer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read upstream reply: %v", err)
	}
	var gotEnv wireEnvelope
	if err := json.Unmarshal(got, &gotEnv); err != nil {
		t.Fatalf("unmarshal upstream reply: %v", err)
	}
	if string(gotEnv.ID) != "7" {
		t.Fatalf("upstream reply id = %s, want original id bytes \"7\" echoed verbatim", gotEnv.ID)
	}
	var result map[string]any
	if err := json.Unmarshal(gotEnv.Result, &result); err != nil {
		t.Fatalf("unmarshal upstream reply result: %v", err)
	}
	if result["granted"] != true {
		t.Fatalf("upstream reply result = %v, want granted=true", result)
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

// If the daemon container endpoint is not yet listening when the shim boots
// (subsystem spawn races the endpoint startup — observed 41s gap in prod),
// the shim must NOT exit. It must continue proxying the codex app-server so
// the daemon's DialUDS to the shim's listen socket succeeds within its 15s
// cap, avoiding the 504 daemon_timeout on POST /api/sessions. Retry the
// daemon dial for up to totalTimeout; return nil so callers degrade
// frame-messaging tools to error at call time rather than kill the shim.
func TestDialDaemonWithRetry_ReturnsNilWhenSocketNeverAppears(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "absent.sock")
	start := time.Now()
	client := dialDaemonWithRetry(context.Background(), sock, 200*time.Millisecond, 20*time.Millisecond)
	elapsed := time.Since(start)
	if client != nil {
		t.Fatalf("expected nil client when sock never appears, got %v", client)
	}
	if elapsed < 200*time.Millisecond {
		t.Fatalf("returned too early: %v < 200ms", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("did not honor totalTimeout: %v", elapsed)
	}
}

func TestDialDaemonWithRetry_ConnectsWhenSocketAppearsMidway(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "late.sock")
	ready := make(chan struct{})
	go func() {
		time.Sleep(80 * time.Millisecond)
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Errorf("net.Listen: %v", err)
			close(ready)
			return
		}
		close(ready)
		defer ln.Close()
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	client := dialDaemonWithRetry(context.Background(), sock, 2*time.Second, 20*time.Millisecond)
	<-ready
	if client == nil {
		t.Fatal("expected non-nil client once sock appears mid-retry")
	}
	_ = client.Close()
}

func TestDialDaemonWithRetry_HonorsContextCancel(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "cancelled.sock")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	client := dialDaemonWithRetry(ctx, sock, 10*time.Second, 20*time.Millisecond)
	elapsed := time.Since(start)
	if client != nil {
		t.Fatal("expected nil client after context cancel")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("did not exit promptly on cancel: %v", elapsed)
	}
}

// When daemon dial ultimately failed (retry exhausted), a shim tool call for
// agent_frames.* must return an error surface — not nil-deref — so the
// Codex user sees a clean tool-call failure while the primary code path
// keeps working.
func TestCodexShimHandleToolCall_NilDaemonReturnsError(t *testing.T) {
	session := &codexShimSession{daemon: nil, sessionID: "sess-1"}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"list", `{"tool":"agent_frames_list","threadId":"thread-1","arguments":{}}`},
		{"read", `{"tool":"agent_frames_read","threadId":"thread-1","arguments":{}}`},
		{"send", `{"tool":"agent_frames_send_message","threadId":"thread-1","arguments":{"targetFrameId":"f2","body":"x"}}`},
		{"reply", `{"tool":"agent_frames_reply","threadId":"thread-1","arguments":{"messageId":"m1"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handled, _, err := session.handleToolCall(json.RawMessage(tc.raw))
			if !handled {
				t.Fatal("agent_frames tool must still be recognized as handled")
			}
			if err == nil {
				t.Fatal("expected error when daemon is unavailable")
			}
		})
	}
}
