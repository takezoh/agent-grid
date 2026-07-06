package codexclient

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-grid/platform/agent/codexschema/v2"
)

func TestServer_EmitTurnCompletedTracksStartedAt(t *testing.T) {
	pr, pw := io.Pipe()
	tr := StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), pw)
	conn := NewConn(tr, time.Second)
	srv := NewServer(conn)
	times := []time.Time{
		time.Unix(100, 0),
		time.Unix(105, 0),
	}
	var nowCalls int
	srv.now = func() time.Time {
		call := nowCalls
		nowCalls++
		return times[call]
	}

	done := make(chan []map[string]any, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		var messages []map[string]any
		for scanner.Scan() {
			var msg map[string]any
			_ = json.Unmarshal(scanner.Bytes(), &msg)
			messages = append(messages, msg)
			if len(messages) == 2 {
				done <- messages
				return
			}
		}
	}()

	if err := srv.EmitTurnStarted("tid1", "turn1"); err != nil {
		t.Fatalf("EmitTurnStarted: %v", err)
	}
	if err := srv.EmitTurnCompleted("tid1", "turn1"); err != nil {
		t.Fatalf("EmitTurnCompleted: %v", err)
	}

	select {
	case messages := <-done:
		if messages[1]["method"] != codexschema.MethodTurnCompleted {
			t.Fatalf("method = %v, want %v", messages[1]["method"], codexschema.MethodTurnCompleted)
		}
		params, _ := messages[1]["params"].(map[string]any)
		turn, _ := params["turn"].(map[string]any)
		if got := int64(turn["startedAt"].(float64)); got != 100 {
			t.Fatalf("turn.startedAt = %d, want 100", got)
		}
		if got := int64(turn["completedAt"].(float64)); got != 105 {
			t.Fatalf("turn.completedAt = %d, want 105", got)
		}
		if got := int64(turn["durationMs"].(float64)); got != 5000 {
			t.Fatalf("turn.durationMs = %d, want 5000", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for emitted messages")
	}
}

func TestServer_EmitTurnCompletedFinalAnswerTracksStartedAt(t *testing.T) {
	pr, pw := io.Pipe()
	tr := StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), pw)
	conn := NewConn(tr, time.Second)
	srv := NewServer(conn)
	times := []time.Time{
		time.Unix(100, 0),
		time.Unix(105, 0),
	}
	var nowCalls int
	srv.now = func() time.Time {
		call := nowCalls
		nowCalls++
		return times[call]
	}

	done := make(chan map[string]any, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			var msg map[string]any
			_ = json.Unmarshal(scanner.Bytes(), &msg)
			if msg["method"] == codexschema.MethodTurnCompleted {
				done <- msg
				return
			}
		}
	}()

	if err := srv.EmitTurnStarted("tid1", "turn1"); err != nil {
		t.Fatalf("EmitTurnStarted: %v", err)
	}
	if err := srv.EmitTurnCompletedFinalAnswer("tid1", "turn1", "agent-turn1", "done"); err != nil {
		t.Fatalf("EmitTurnCompletedFinalAnswer: %v", err)
	}

	select {
	case msg := <-done:
		raw, err := json.Marshal(msg["params"])
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		notification, err := codexschemav2.UnmarshalTurnCompletedNotification(raw)
		if err != nil {
			t.Fatalf("UnmarshalTurnCompletedNotification: %v", err)
		}
		if got := notification.Turn.StartedAt; got == nil || *got != 100 {
			t.Fatalf("turn.startedAt = %v, want 100", got)
		}
		if got := notification.Turn.CompletedAt; got == nil || *got != 105 {
			t.Fatalf("turn.completedAt = %v, want 105", got)
		}
		if got := notification.Turn.DurationMS; got == nil || *got != 5000 {
			t.Fatalf("turn.durationMs = %v, want 5000", got)
		}
		if len(notification.Turn.Items) != 1 {
			t.Fatalf("items = %d, want 1", len(notification.Turn.Items))
		}
		if notification.Turn.Items[0].Text == nil || *notification.Turn.Items[0].Text != "done" {
			t.Fatalf("item.text = %v, want done", notification.Turn.Items[0].Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for emitted completion")
	}
}

func TestTurnCompletedParamsAtUsesExplicitTiming(t *testing.T) {
	params := TurnCompletedParamsAt("tid1", "turn1", time.Unix(10, 0), time.Unix(13, 0))
	turn, _ := params["turn"].(map[string]any)
	if got := turn["startedAt"].(int64); got != 10 {
		t.Fatalf("turn.startedAt = %d, want 10", got)
	}
	if got := turn["completedAt"].(int64); got != 13 {
		t.Fatalf("turn.completedAt = %d, want 13", got)
	}
	if got := turn["durationMs"].(int64); got != 3000 {
		t.Fatalf("turn.durationMs = %d, want 3000", got)
	}
}

func TestTurnCompletedFinalAnswerParamsAtBuildsV2AgentMessage(t *testing.T) {
	params := TurnCompletedFinalAnswerParamsAt(
		"tid1",
		"turn1",
		"session1",
		time.Unix(10, 0),
		time.Unix(13, 0),
		"agent-turn1",
		"done",
	)
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	notification, err := codexschemav2.UnmarshalTurnCompletedNotification(raw)
	if err != nil {
		t.Fatalf("UnmarshalTurnCompletedNotification: %v", err)
	}
	if len(notification.Turn.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(notification.Turn.Items))
	}
	item := notification.Turn.Items[0]
	if item.Type != codexschemav2.AgentMessage {
		t.Fatalf("item.type = %q, want %q", item.Type, codexschemav2.AgentMessage)
	}
	if item.Text == nil || *item.Text != "done" {
		t.Fatalf("item.text = %v, want done", item.Text)
	}
	if item.Phase == nil || *item.Phase != codexschemav2.FinalAnswer {
		t.Fatalf("item.phase = %v, want %q", item.Phase, codexschemav2.FinalAnswer)
	}
}

func TestServer_EmitTurnCompletedFallsBackWhenStartIsUnknown(t *testing.T) {
	pr, pw := io.Pipe()
	tr := StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), pw)
	conn := NewConn(tr, time.Second)
	srv := NewServer(conn)
	srv.now = func() time.Time { return time.Unix(42, 0) }

	done := make(chan map[string]any, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		if scanner.Scan() {
			var msg map[string]any
			_ = json.Unmarshal(scanner.Bytes(), &msg)
			done <- msg
		}
	}()

	if err := srv.EmitTurnCompleted("tid1", "turn1"); err != nil {
		t.Fatalf("EmitTurnCompleted: %v", err)
	}

	select {
	case msg := <-done:
		params, _ := msg["params"].(map[string]any)
		turn, _ := params["turn"].(map[string]any)
		if got := int64(turn["startedAt"].(float64)); got != 42 {
			t.Fatalf("turn.startedAt = %d, want 42", got)
		}
		if got := int64(turn["durationMs"].(float64)); got != 0 {
			t.Fatalf("turn.durationMs = %d, want 0", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for emitted message")
	}
}

func TestServer_EmitTurnFailedClearsOnlyFailedTurnStart(t *testing.T) {
	tr := StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), io.Discard)
	conn := NewConn(tr, time.Second)
	srv := NewServer(conn)
	srv.now = func() time.Time { return time.Unix(42, 0) }

	if err := srv.EmitTurnStarted("tid1", "turn1"); err != nil {
		t.Fatalf("EmitTurnStarted: %v", err)
	}
	if err := srv.EmitTurnStarted("tid1", "turn2"); err != nil {
		t.Fatalf("EmitTurnStarted: %v", err)
	}
	if err := srv.EmitTurnFailed("tid1", "turn1", "boom"); err != nil {
		t.Fatalf("EmitTurnFailed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if _, ok := srv.turnStartedAt[turnKey("tid1", "turn1")]; ok {
		t.Fatal("turnStartedAt must be cleared after EmitTurnFailed")
	}
	if _, ok := srv.turnStartedAt[turnKey("tid1", "turn2")]; !ok {
		t.Fatal("unrelated turnStartedAt must be preserved")
	}
}
