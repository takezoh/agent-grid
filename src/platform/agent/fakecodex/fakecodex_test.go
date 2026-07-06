package fakecodex

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
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
		codexschema.MethodItemAgentMessageDelta,
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

	rawStarted := nc.last(codexschema.MethodThreadStarted)
	var started map[string]any
	if err := json.Unmarshal(rawStarted, &started); err != nil {
		t.Fatalf("unmarshal thread/started: %v", err)
	}
	thread, _ := started["thread"].(map[string]any)
	if thread["cwd"] != "/ws" {
		t.Fatalf("thread.cwd = %v, want /ws", thread["cwd"])
	}
	if thread["path"] != DefaultThreadPath {
		t.Fatalf("thread.path = %v, want %s", thread["path"], DefaultThreadPath)
	}
}

func TestServer_DefaultTokenUsagePreservesCachedOnlyOverride(t *testing.T) {
	s := New(Config{
		TokenUsage: TokenUsageSpec{
			Last: TokenBreakdown{CachedTokens: 7},
		},
	})
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
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	rawUsage := nc.last(codexschema.MethodThreadTokenUsageUpdated)
	var payload struct {
		TokenUsage struct {
			Last  map[string]float64 `json:"last"`
			Total map[string]float64 `json:"total"`
		} `json:"tokenUsage"`
	}
	if err := json.Unmarshal(rawUsage, &payload); err != nil {
		t.Fatalf("unmarshal tokenUsage: %v", err)
	}
	if got := int(payload.TokenUsage.Last["inputTokens"]); got != 0 {
		t.Fatalf("last.inputTokens = %d, want 0 when only CachedTokens is overridden", got)
	}
	if got := int(payload.TokenUsage.Last["outputTokens"]); got != 0 {
		t.Fatalf("last.outputTokens = %d, want 0 when only CachedTokens is overridden", got)
	}
	if got := int(payload.TokenUsage.Last["cachedTokens"]); got != 7 {
		t.Fatalf("last.cachedTokens = %d, want 7", got)
	}
	if got := int(payload.TokenUsage.Total["totalTokens"]); got != 0 {
		t.Fatalf("total.totalTokens = %d, want 0 when only CachedTokens is counted separately", got)
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

func TestDefaultTurnContractPreservesMissingKeys(t *testing.T) {
	t.Run("missing_text_stays_missing", func(t *testing.T) {
		contract := defaultTurnRecordedEvent(t, codexschema.MethodTurnCompleted, json.RawMessage(`{"threadId":"thread-1"}`))
		want := map[string]any{
			"method": codexschema.MethodTurnCompleted,
			"params": map[string]any{
				"threadId": "<id>",
			},
		}
		if !reflect.DeepEqual(contract, want) {
			t.Fatalf("contract = %#v, want %#v", contract, want)
		}
	})

	t.Run("empty_text_stays_present", func(t *testing.T) {
		contract := defaultTurnRecordedEvent(t, codexschema.MethodTurnCompleted, json.RawMessage(`{"threadId":"thread-1","text":""}`))
		want := map[string]any{
			"method": codexschema.MethodTurnCompleted,
			"params": map[string]any{
				"threadId": "<id>",
				"text":     "",
			},
		}
		if !reflect.DeepEqual(contract, want) {
			t.Fatalf("contract = %#v, want %#v", contract, want)
		}
	})

	t.Run("contract_projection_keeps_nested_turn", func(t *testing.T) {
		contract := defaultTurnEventContract(t, codexschema.MethodTurnCompleted, json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[{"type":"agentMessage","phase":"final_answer","text":"done"}]}}`))
		want := map[string]any{
			"method": codexschema.MethodTurnCompleted,
			"params": map[string]any{
				"threadId": "<id>",
				"turn": map[string]any{
					"id":     "<id>",
					"status": "completed",
					"items": []any{
						map[string]any{
							"type":  "agentMessage",
							"phase": "final_answer",
							"text":  "done",
						},
					},
				},
			},
		}
		if !reflect.DeepEqual(contract, want) {
			t.Fatalf("contract = %#v, want %#v", contract, want)
		}
	})

	t.Run("recorded_thread_started_normalizes_cli_version", func(t *testing.T) {
		recorded := defaultTurnRecordedEvent(t, codexschema.MethodThreadStarted, json.RawMessage(`{"thread":{"id":"thread-1","cwd":"/tmp/work","path":"/tmp/work/rollout.jsonl","cliVersion":"0.999.0"}}`))
		want := map[string]any{
			"method": codexschema.MethodThreadStarted,
			"params": map[string]any{
				"thread": map[string]any{
					"id":         "<id>",
					"cwd":        "<path>",
					"path":       "<path>",
					"cliVersion": "string",
				},
			},
		}
		if !reflect.DeepEqual(recorded, want) {
			t.Fatalf("recorded = %#v, want %#v", recorded, want)
		}
	})
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

func TestServer_TextTurnHandlerPreservesCompletedTextAsFinalAnswer(t *testing.T) {
	s := New(Config{Handler: TextTurnHandler("draft", "final")})
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
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	rawCompleted := nc.last(codexschema.MethodTurnCompleted)
	var payload struct {
		Turn struct {
			Items []struct {
				Text  string `json:"text"`
				Phase string `json:"phase"`
			} `json:"items"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(rawCompleted, &payload); err != nil {
		t.Fatalf("unmarshal turn/completed: %v", err)
	}
	if len(payload.Turn.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(payload.Turn.Items))
	}
	if got := payload.Turn.Items[0].Text; got != "final" {
		t.Fatalf("completion item text = %q, want %q", got, "final")
	}
	if got := payload.Turn.Items[0].Phase; got != "final_answer" {
		t.Fatalf("completion item phase = %q, want final_answer", got)
	}

	rawDelta := nc.last(codexschema.MethodItemAgentMessageDelta)
	var delta struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal(rawDelta, &delta); err != nil {
		t.Fatalf("unmarshal delta: %v", err)
	}
	if delta.Delta != "draft" {
		t.Fatalf("delta = %q, want %q", delta.Delta, "draft")
	}
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
