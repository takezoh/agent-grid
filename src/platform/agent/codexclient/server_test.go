package codexclient_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
)

// emitAndCapture is a helper that creates a Server backed by a write pipe
// and a scanner that captures the emitted lines.
func emitAndCapture(t *testing.T, emit func(*codexclient.Server)) map[string]any {
	t.Helper()
	pr, pw := io.Pipe()
	tr := codexclient.StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), pw)
	conn := codexclient.NewConn(tr, time.Second)
	srv := codexclient.NewServer(conn)

	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		if scanner.Scan() {
			done <- scanner.Text()
		}
	}()

	emit(srv)

	select {
	case line := <-done:
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for emitted message")
		return nil
	}
}

func TestServer_EmitTurnCompleted(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTurnCompleted("tid1", "turn1"); err != nil {
			t.Fatalf("EmitTurnCompleted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodTurnCompleted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodTurnCompleted)
	}
	params, _ := msg["params"].(map[string]any)
	if params["threadId"] != "tid1" {
		t.Fatalf("params = %v", params)
	}
	turn, _ := params["turn"].(map[string]any)
	if turn["id"] != "turn1" {
		t.Fatalf("params.turn.id = %v, want turn1", turn["id"])
	}
	if _, ok := params["text"]; ok {
		t.Fatalf("turn/completed leaked legacy text field: %v", params)
	}
}

func TestServer_EmitTurnFailed(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTurnFailed("tid1", "turn1", "something broke"); err != nil {
			t.Fatalf("EmitTurnFailed: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodError {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodError)
	}
	params, _ := msg["params"].(map[string]any)
	if params["message"] != "something broke" {
		t.Fatalf("params = %v", params)
	}
}

func TestServer_EmitThreadStarted(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitThreadStarted("t1", "/work"); err != nil {
			t.Fatalf("EmitThreadStarted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodThreadStarted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodThreadStarted)
	}
	params, _ := msg["params"].(map[string]any)
	thread, _ := params["thread"].(map[string]any)
	if thread["id"] != "t1" {
		t.Fatalf("thread.id = %v, want t1", thread["id"])
	}
	if thread["cwd"] != "/work" {
		t.Fatalf("thread.cwd = %v, want /work", thread["cwd"])
	}
	if thread["sessionId"] != "t1" {
		t.Fatalf("thread.sessionId = %v, want t1", thread["sessionId"])
	}
	if thread["source"] != "appServer" {
		t.Fatalf("thread.source = %v, want appServer", thread["source"])
	}
	status, _ := thread["status"].(map[string]any)
	if status["type"] != "idle" {
		t.Fatalf("thread.status.type = %v, want idle", status["type"])
	}
	if _, ok := thread["path"]; ok {
		t.Fatalf("thread.path must not be synthesized from cwd: %v", thread)
	}
}

func TestServer_EmitThreadStartedWithPath(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitThreadStartedWithPath("t1", "/work", "/tmp/rollout.jsonl"); err != nil {
			t.Fatalf("EmitThreadStartedWithPath: %v", err)
		}
	})
	params, _ := msg["params"].(map[string]any)
	thread, _ := params["thread"].(map[string]any)
	if thread["cwd"] != "/work" {
		t.Fatalf("thread.cwd = %v, want /work", thread["cwd"])
	}
	if thread["path"] != "/tmp/rollout.jsonl" {
		t.Fatalf("thread.path = %v, want /tmp/rollout.jsonl", thread["path"])
	}
}

func TestThreadStartResponse(t *testing.T) {
	resp := codexclient.ThreadStartResponse(codexclient.ThreadDescriptor{
		ThreadID:    "t1",
		SessionID:   "s1",
		CWD:         "/work",
		RolloutPath: "/tmp/rollout.jsonl",
	})
	if resp["approvalPolicy"] != "never" {
		t.Fatalf("approvalPolicy = %v, want never", resp["approvalPolicy"])
	}
	if resp["approvalsReviewer"] != "user" {
		t.Fatalf("approvalsReviewer = %v, want user", resp["approvalsReviewer"])
	}
	thread, _ := resp["thread"].(map[string]any)
	if thread["sessionId"] != "s1" {
		t.Fatalf("thread.sessionId = %v, want s1", thread["sessionId"])
	}
	if thread["path"] != "/tmp/rollout.jsonl" {
		t.Fatalf("thread.path = %v, want /tmp/rollout.jsonl", thread["path"])
	}
}

func TestTurnStartResponseAt(t *testing.T) {
	resp := codexclient.TurnStartResponseAt("turn1", time.Unix(123, 0))
	turn, _ := resp["turn"].(map[string]any)
	if turn["id"] != "turn1" {
		t.Fatalf("turn.id = %v, want turn1", turn["id"])
	}
	if turn["status"] != "inProgress" {
		t.Fatalf("turn.status = %v, want inProgress", turn["status"])
	}
	if turn["startedAt"] != int64(123) {
		t.Fatalf("turn.startedAt = %v, want 123", turn["startedAt"])
	}
}

func TestServer_EmitAgentMessageDelta(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitAgentMessageDelta("t1", "partial text"); err != nil {
			t.Fatalf("EmitAgentMessageDelta: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodItemAgentMessageDelta {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodItemAgentMessageDelta)
	}
	params, _ := msg["params"].(map[string]any)
	if params["delta"] != "partial text" {
		t.Fatalf("params = %v", params)
	}
}

func TestClient_Initialize(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go connB.Run(ctx, &initHandler{conn: connB}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})            //nolint:errcheck

	if err := codexclient.Initialize(connA); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
}

type initHandler struct{ conn *codexclient.Conn }

func (h *initHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *initHandler) OnServerRequest(id codexclient.RequestID, _ string, _ json.RawMessage) {
	_ = h.conn.Reply(id, map[string]any{})
}

// --- server accessor and emit methods ---

func TestServer_Conn(t *testing.T) {
	pr, pw := io.Pipe()
	tr := codexclient.StdioTransport(pr, pw)
	conn := codexclient.NewConn(tr, time.Second)
	srv := codexclient.NewServer(conn)
	if srv.Conn() != conn {
		t.Fatal("Conn() must return the wrapped conn")
	}
	_ = pr.Close()
	_ = pw.Close()
}

func TestServer_EmitNotification(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitNotification("custom/event", map[string]any{"k": "v"}); err != nil {
			t.Fatalf("EmitNotification: %v", err)
		}
	})
	if msg["method"] != "custom/event" {
		t.Fatalf("method = %v, want custom/event", msg["method"])
	}
}

func TestServer_EmitTurnStarted(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTurnStarted("tid1", "turn1"); err != nil {
			t.Fatalf("EmitTurnStarted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodTurnStarted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodTurnStarted)
	}
	params, _ := msg["params"].(map[string]any)
	turn, _ := params["turn"].(map[string]any)
	if turn["id"] != "turn1" {
		t.Fatalf("params.turn.id = %v, want turn1", turn["id"])
	}
}

func TestServer_EmitItemStarted(t *testing.T) {
	item := map[string]any{"type": "tool", "id": "x1"}
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitItemStarted("tid1", "turn1", item); err != nil {
			t.Fatalf("EmitItemStarted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodItemStarted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodItemStarted)
	}
}

func TestServer_EmitItemCompleted(t *testing.T) {
	item := map[string]any{"type": "tool", "id": "x1", "output": "done"}
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitItemCompleted("tid1", "turn1", item); err != nil {
			t.Fatalf("EmitItemCompleted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodItemCompleted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodItemCompleted)
	}
}

func TestServer_EmitTokenUsage(t *testing.T) {
	last := map[string]any{"inputTokens": float64(10), "outputTokens": float64(5)}
	total := map[string]any{"inputTokens": float64(100), "outputTokens": float64(50)}
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTokenUsage("tid1", "turn1", last, total); err != nil {
			t.Fatalf("EmitTokenUsage: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodThreadTokenUsageUpdated {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodThreadTokenUsageUpdated)
	}
}

// --- client role functions ---

// threadStartHandler responds to thread/start with a fixed thread id.
type threadStartHandler struct{ conn *codexclient.Conn }

func (h *threadStartHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *threadStartHandler) OnServerRequest(id codexclient.RequestID, method string, _ json.RawMessage) {
	if method == codexschema.MethodThreadStart {
		_ = h.conn.Reply(id, map[string]any{"thread": map[string]any{"id": "th-42"}})
	} else {
		_ = h.conn.Reply(id, map[string]any{})
	}
}

func TestStartThread(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, &threadStartHandler{conn: connB}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})                   //nolint:errcheck

	session, err := codexclient.StartThread(connA, "/work", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if session.ThreadID != "th-42" {
		t.Fatalf("got thread id %q, want th-42", session.ThreadID)
	}
}

func TestStartThread_WithOptions(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, &threadStartHandler{conn: connB}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})                   //nolint:errcheck

	opts := codexclient.ThreadOptions{ApprovalPolicy: "never", SandboxMode: "workspace-write", ServiceName: "test"}
	session, err := codexclient.StartThread(connA, "/work", []any{"tool1"}, opts)
	if err != nil {
		t.Fatalf("StartThread with options: %v", err)
	}
	if session.ThreadID != "th-42" {
		t.Fatalf("got thread id %q, want th-42", session.ThreadID)
	}
}

func TestResumeThread(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, &echoHandler{conn: connB}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})            //nolint:errcheck

	_, err := codexclient.ResumeThread(connA, codexclient.ResumeOptions{ThreadID: "th-1", Cwd: "/work"})
	if err != nil {
		t.Fatalf("ResumeThread: %v", err)
	}
}

type unsubscribeHandler struct {
	conn   *codexclient.Conn
	status string
	recv   chan map[string]any
}

func (h *unsubscribeHandler) OnNotification(string, json.RawMessage) {}
func (h *unsubscribeHandler) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	var decoded map[string]any
	_ = json.Unmarshal(params, &decoded)
	h.recv <- map[string]any{"method": method, "threadId": decoded["threadId"]}
	_ = h.conn.Reply(id, map[string]any{"status": h.status})
}

func TestUnsubscribeThreadContract(t *testing.T) {
	for _, want := range []codexclient.ThreadUnsubscribeStatus{
		codexclient.ThreadUnsubscribed,
		codexclient.ThreadUnsubscribeNotSubscribed,
		codexclient.ThreadUnsubscribeNotLoaded,
	} {
		t.Run(string(want), func(t *testing.T) {
			ta, tb := pipeTransport()
			client := codexclient.NewConn(ta, time.Second)
			server := codexclient.NewConn(tb, time.Second)
			recv := make(chan map[string]any, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go server.Run(ctx, &unsubscribeHandler{conn: server, status: string(want), recv: recv}) //nolint:errcheck
			go client.Run(ctx, &noopHandler{})                                                      //nolint:errcheck

			got, err := codexclient.UnsubscribeThread(client, "th-1")
			if err != nil || got != want {
				t.Fatalf("UnsubscribeThread = (%q, %v), want (%q, nil)", got, err, want)
			}
			request := <-recv
			if request["method"] != codexschema.MethodThreadUnsubscribe || request["threadId"] != "th-1" {
				t.Fatalf("request = %#v", request)
			}
		})
	}
}

func TestUnsubscribeThreadRejectsUnknownStatus(t *testing.T) {
	ta, tb := pipeTransport()
	client := codexclient.NewConn(ta, time.Second)
	server := codexclient.NewConn(tb, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx, &unsubscribeHandler{conn: server, status: "futureStatus", recv: make(chan map[string]any, 1)}) //nolint:errcheck
	go client.Run(ctx, &noopHandler{})                                                                                //nolint:errcheck
	if _, err := codexclient.UnsubscribeThread(client, "th-1"); err == nil {
		t.Fatal("unknown unsubscribe status must be rejected")
	}
}

func TestStartTurn(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	recv := make(chan string, 1)
	h := &turnStartHandler{conn: connB, recv: recv}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, h)              //nolint:errcheck
	go connA.Run(ctx, &noopHandler{}) //nolint:errcheck

	opts := codexclient.TurnOptions{ApprovalPolicy: "never", SandboxPolicy: "workspace-write"}
	if err := codexclient.StartTurn(connA, "th-1", "/work", []byte("hello"), opts); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	select {
	case got := <-recv:
		if got == "" {
			t.Fatal("expected request params")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for turn/start request")
	}
}

type turnStartHandler struct {
	conn *codexclient.Conn
	recv chan string
}

func (h *turnStartHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *turnStartHandler) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		_ = h.conn.Reply(id, map[string]any{})
		return
	}
	h.recv <- string(params)
	_ = h.conn.Reply(id, map[string]any{"turn": map[string]any{"id": "turn-1"}})
}

func TestConn_Close(t *testing.T) {
	pr, pw := io.Pipe()
	tr := codexclient.StdioTransport(pr, pw)
	conn := codexclient.NewConn(tr, time.Second)
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDefaultStdioTransport(t *testing.T) {
	tr := codexclient.DefaultStdioTransport()
	if tr == nil {
		t.Fatal("DefaultStdioTransport returned nil")
	}
}
