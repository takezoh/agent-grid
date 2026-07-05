package fakecodex

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

// pipeToClient wires a fake Server to a client Conn via two io.Pipe pairs
// and returns the client Conn plus a cleanup func.
func pipeToClient(t *testing.T, s *Server) (*codexclient.Conn, func()) {
	t.Helper()
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	// server reads pr2 (data from client), writes pw1 (data to client)
	ctx, cancel := context.WithCancel(context.Background())
	stop := s.Attach(ctx, pr2, pw1)

	// client reads pr1, writes pw2
	client := codexclient.NewConn(codexclient.StdioTransport(pr1, pw2), 3*time.Second)
	return client, func() {
		cancel()
		stop()
		_ = pw2.Close()
		_ = pr1.Close()
	}
}

// notifRecorder captures notifications for assertion.
type notifRecorder struct {
	mu     sync.Mutex
	events []struct {
		method string
		params json.RawMessage
	}
}

func (r *notifRecorder) OnNotification(method string, params json.RawMessage) {
	r.mu.Lock()
	r.events = append(r.events, struct {
		method string
		params json.RawMessage
	}{method, append(json.RawMessage(nil), params...)})
	r.mu.Unlock()
}
func (r *notifRecorder) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

func (r *notifRecorder) methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.events))
	for i, e := range r.events {
		out[i] = e.method
	}
	return out
}

func (r *notifRecorder) last(method string) json.RawMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].method == method {
			return r.events[i].params
		}
	}
	return nil
}

func waitFor(t *testing.T, nc *notifRecorder, want []string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got := nc.methods()
		if len(got) >= len(want) {
			for i, m := range want {
				if got[i] != m {
					t.Fatalf("methods[%d] = %q, want %q (all=%v)", i, got[i], m, got)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %v, got %v", want, nc.methods())
}

// TestServer_DefaultTurn — default handler emits the canonical event sequence.
func TestServer_DefaultTurn(t *testing.T) {
	s := New(Config{})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	nc := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(client, "", "/ws", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	waitFor(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	// LastTurnParams / LastMessage populated.
	if s.LastMessage() != "hi" {
		t.Errorf("LastMessage = %q, want 'hi'", s.LastMessage())
	}
	if s.LastCWD() != "/ws" {
		t.Errorf("LastCWD = %q, want '/ws'", s.LastCWD())
	}

	// tokenUsage.total is populated.
	rawUsage := nc.last(codexschema.MethodThreadTokenUsageUpdated)
	var pu map[string]any
	if err := json.Unmarshal(rawUsage, &pu); err != nil {
		t.Fatalf("unmarshal tokenUsage: %v", err)
	}
	tu, _ := pu["tokenUsage"].(map[string]any)
	last, _ := tu["last"].(map[string]any)
	if int(last["inputTokens"].(float64)) != 10 {
		t.Errorf("last.inputTokens = %v, want 10", last["inputTokens"])
	}
	if _, ok := tu["modelContextWindow"]; !ok {
		t.Errorf("tokenUsage missing modelContextWindow field")
	}
}

// TestServer_FailingTurn — a failing handler emits `error` instead of turn/completed.
func TestServer_FailingTurn(t *testing.T) {
	s := New(Config{Handler: FailingTurnHandler("boom")})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	nc := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(client, "", "/ws", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	waitFor(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodError,
	})
	raw := nc.last(codexschema.MethodError)
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	if p["message"] != "boom" {
		t.Errorf("error.message = %v, want 'boom'", p["message"])
	}
}

// TestServer_ThreadStartParams — the fake records thread/start params.
func TestServer_ThreadStartParams(t *testing.T) {
	s := New(Config{})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	go func() { _ = client.Run(context.Background(), &notifRecorder{}) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	ts, err := codexclient.StartThread(client, "/ws", []any{
		map[string]any{"name": "linear_graphql", "inputSchema": map[string]any{"type": "object"}},
	}, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if ts.ThreadID != DefaultThreadID {
		t.Errorf("threadID = %q, want %q", ts.ThreadID, DefaultThreadID)
	}
	if s.LastThreadParams() == nil {
		t.Errorf("LastThreadParams was not recorded")
	}
}

// TestServer_ItemPairHandler — item/started + item/completed appear between
// turn/started and thread/tokenUsage/updated.
func TestServer_ItemPairHandler(t *testing.T) {
	started := map[string]any{"id": "tu-1", "type": "dynamicToolCall", "tool": "Bash"}
	completed := map[string]any{"id": "tu-1", "type": "dynamicToolCall", "status": "completed", "output": "ok"}
	s := New(Config{Handler: ItemPairHandler(started, completed, "done")})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	nc := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(client, "", "/ws", []byte("run"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	waitFor(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemStarted,
		codexschema.MethodItemCompleted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})
}

// TestServer_HangTurn — turn never resolves; assertion is that no
// turn/completed nor error appears within a short window.
func TestServer_HangTurn(t *testing.T) {
	s := New(Config{HangTurn: true})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	nc := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(client, "", "/ws", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// Wait for the two ordering-defined emissions, then confirm no completion arrives.
	waitFor(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
	})
	time.Sleep(200 * time.Millisecond)
	for _, m := range nc.methods() {
		if m == codexschema.MethodTurnCompleted || m == codexschema.MethodError {
			t.Fatalf("hang turn produced %s", m)
		}
	}
}

// TestServer_ResumeReturnsSameThreadID — thread/resume replies with the
// configured thread id.
func TestServer_ResumeReturnsSameThreadID(t *testing.T) {
	s := New(Config{ThreadID: "thr-42"})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	go func() { _ = client.Run(context.Background(), &notifRecorder{}) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	ts, err := codexclient.ResumeThread(client, codexclient.ResumeOptions{ThreadID: "thr-42", Cwd: "/ws"})
	if err != nil {
		t.Fatalf("ResumeThread: %v", err)
	}
	if ts.ThreadID != "thr-42" {
		t.Errorf("threadID = %q, want thr-42", ts.ThreadID)
	}
	if s.LastResumeParams() == nil {
		t.Errorf("LastResumeParams was not recorded")
	}
}

// TestServer_FailInit — initialize returns a JSON-RPC error.
func TestServer_FailInit(t *testing.T) {
	s := New(Config{FailInit: true})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	go func() { _ = client.Run(context.Background(), &notifRecorder{}) }()

	err := codexclient.Initialize(client)
	if err == nil {
		t.Fatalf("Initialize: want error, got nil")
	}
}

func TestServer_SettingsUpdatedHandlerSupportsVariants(t *testing.T) {
	s := New(Config{Handler: SettingsUpdatedHandler(
		SettingsUpdatedSpec{Model: "gpt-5", ModelSet: true},
		SettingsUpdatedSpec{Effort: "medium", EffortSet: true, EffortField: settingsFieldReasoningEffort},
		SettingsUpdatedSpec{EffortSet: true},
	)})
	client, cleanup := pipeToClient(t, s)
	defer cleanup()

	nc := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), nc) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := codexclient.StartTurn(client, "", "/ws", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	waitFor(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	nc.mu.Lock()
	defer nc.mu.Unlock()
	var payloads []map[string]any
	for _, ev := range nc.events {
		if ev.method != codexschema.MethodThreadSettingsUpdated {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(ev.params, &decoded); err != nil {
			t.Fatalf("unmarshal settings payload: %v", err)
		}
		payloads = append(payloads, decoded)
	}
	if got := len(payloads); got != 3 {
		t.Fatalf("settings payload count = %d, want 3", got)
	}
	firstSettings, _ := payloads[0]["threadSettings"].(map[string]any)
	if firstSettings["model"] != "gpt-5" {
		t.Fatalf("first settings = %+v, want model update", firstSettings)
	}
	secondSettings, _ := payloads[1]["threadSettings"].(map[string]any)
	if value, ok := secondSettings["reasoning_effort"]; !ok || value != "medium" {
		t.Fatalf("second settings = %+v, want reasoning_effort:medium", secondSettings)
	}
	thirdSettings, _ := payloads[2]["threadSettings"].(map[string]any)
	if value, ok := thirdSettings["effort"]; !ok || value != nil {
		t.Fatalf("third settings = %+v, want effort:null clear", thirdSettings)
	}
}
