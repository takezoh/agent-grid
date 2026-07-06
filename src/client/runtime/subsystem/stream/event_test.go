package stream

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-grid/client/state"
)

func newTestBackend() (*Backend, *fakeRuntime) {
	fr := &fakeRuntime{}
	b := New(fr, nil, "sid", "sess1", "/p", "codex", nil, "", "", false, false, "/sock", 0)
	return b, fr
}

func TestHandleThreadStarted(t *testing.T) {
	// Threads are bound at creation (bindThread); thread.started only confirms.
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", startDir: "/work", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadStarted(json.RawMessage(`{"thread":{"id":"t1","cwd":"/work"}}`))

	b.mu.Lock()
	bound := b.frames["f1"]
	b.mu.Unlock()
	if bound.resumePhase != resumePhaseAttached {
		t.Errorf("binding not confirmed attached: %+v", bound)
	}
	if len(fr.events) == 0 {
		t.Errorf("expected emitted SessionReady event")
	}
}

func TestHandleThreadStartedEmitsMetadata(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", startDir: "/work", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadStarted(json.RawMessage(`{"thread":{"id":"t1","name":" saved-session ","preview":" preview text "}}`))
	if len(fr.events) != 2 {
		t.Fatalf("expected ready + metadata events, got %d", len(fr.events))
	}
	meta := fr.events[1].(state.EvSubsystem)
	if meta.Kind != state.SubsystemMetadataUpdated {
		t.Fatalf("Kind = %q, want %q", meta.Kind, state.SubsystemMetadataUpdated)
	}
	if meta.Payload.Title != "saved-session" || meta.Payload.Preview != "preview text" {
		t.Fatalf("metadata payload = %+v", meta.Payload)
	}
}

func TestHandleThreadStartedUnknownThreadDrops(t *testing.T) {
	// A waiting frame exists, but the thread is not bound to it. A thread.started
	// for an unknown thread must NOT be adopted (no cwd/active-frame heuristic) —
	// it is dropped. This pins the removal of the cross-talk fallback.
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", startDir: "/work"}
	b.mu.Unlock()
	b.handleThreadStarted([]byte(`{"thread":{"id":"t1","cwd":"/work"}}`))
	if len(fr.events) != 0 {
		t.Errorf("unknown thread must not emit, got %d events", len(fr.events))
	}
	b.mu.Lock()
	_, bound := b.threads["t1"]
	b.mu.Unlock()
	if bound {
		t.Error("unknown thread must not bind to the waiting frame")
	}
}

func TestHandleTurnCompleted(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnCompleted([]byte(`{"threadId":"t1","text":"hello"}`))
	if len(fr.events) == 0 {
		t.Errorf("expected event")
	}
	b.mu.Lock()
	last := b.frames["f1"].lastAssistant
	b.mu.Unlock()
	if last != "hello" {
		t.Errorf("lastAssistant = %q", last)
	}
}

func TestHandleTurnCompletedFallsBackToDeltaStateWhenCompletionHasNoText(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"hello "}`))
	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"world"}`))
	b.handleTurnCompleted([]byte(`{"threadId":"t1","turn":{"id":"tu1","items":[],"itemsView":"notLoaded","status":"completed"}}`))

	if got := len(fr.events); got != 3 {
		t.Fatalf("events = %d, want 3", got)
	}
	ev, ok := fr.events[2].(state.EvSubsystem)
	if !ok {
		t.Fatalf("event = %T, want EvSubsystem", fr.events[2])
	}
	if ev.Payload.LastAssistantMessage != "hello world" {
		t.Fatalf("LastAssistantMessage = %q, want %q", ev.Payload.LastAssistantMessage, "hello world")
	}
	b.mu.Lock()
	last := b.frames["f1"].lastAssistant
	history := append([]state.SubsystemTurn(nil), b.frames["f1"].history...)
	b.mu.Unlock()
	if last != "hello world" {
		t.Fatalf("lastAssistant = %q, want %q", last, "hello world")
	}
	if len(history) != 1 || history[0].Role != "assistant" || history[0].Text != "hello world" {
		t.Fatalf("history = %+v", history)
	}
}

func TestHandleTurnCompletedUsesTurnItemsAssistantText(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnCompleted([]byte(`{
		"threadId":"t1",
		"turn":{
			"id":"tu1",
			"items":[{"type":"agentMessage","content":[{"type":"text","text":"from items"}]}],
			"status":"completed"
		}
	}`))

	ev, ok := fr.events[0].(state.EvSubsystem)
	if !ok {
		t.Fatalf("event = %T, want EvSubsystem", fr.events[0])
	}
	if ev.Payload.LastAssistantMessage != "from items" {
		t.Fatalf("LastAssistantMessage = %q, want %q", ev.Payload.LastAssistantMessage, "from items")
	}
}

func TestHandleTurnCompletedDoesNotReusePreviousTurnAssistantWhenCurrentTurnIsEmpty(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", lastAssistant: "previous turn"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnCompleted([]byte(`{"threadId":"t1","turn":{"id":"tu2","items":[],"itemsView":"notLoaded","status":"completed"}}`))

	if got := len(fr.events); got != 1 {
		t.Fatalf("events = %d, want 1", got)
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Payload.LastAssistantMessage != "" {
		t.Fatalf("LastAssistantMessage = %q, want empty", ev.Payload.LastAssistantMessage)
	}
	b.mu.Lock()
	last := b.frames["f1"].lastAssistant
	history := append([]state.SubsystemTurn(nil), b.frames["f1"].history...)
	b.mu.Unlock()
	if last != "" {
		t.Fatalf("lastAssistant = %q, want empty", last)
	}
	if len(history) != 0 {
		t.Fatalf("history = %+v, want empty", history)
	}
}

func TestHandleTurnCompletedDropsStaleCompletedTurn(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{
		frameID:       "f1",
		threadID:      "t1",
		activeTurnID:  "turn-2",
		turnAssistant: "current",
	}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnCompleted([]byte(`{"threadId":"t1","turn":{"id":"turn-1","items":[{"type":"agentMessage","text":"old"}],"status":"completed"}}`))

	if len(fr.events) != 0 {
		t.Fatalf("events = %d, want 0", len(fr.events))
	}
	b.mu.Lock()
	binding := b.frames["f1"]
	b.mu.Unlock()
	if binding.activeTurnID != "turn-2" {
		t.Fatalf("activeTurnID = %q, want turn-2", binding.activeTurnID)
	}
	if binding.turnAssistant != "current" {
		t.Fatalf("turnAssistant = %q, want current", binding.turnAssistant)
	}
}

func TestHandleTurnCompletedUnknownThread(t *testing.T) {
	b, fr := newTestBackend()
	b.handleTurnCompleted([]byte(`{"threadId":"unknown"}`))
	if len(fr.events) != 0 {
		t.Errorf("expected no events")
	}
}

func TestHandleAgentMessageDelta(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"abc"}`))
	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"def"}`))
	b.mu.Lock()
	turn := b.frames["f1"].turnAssistant
	last := b.frames["f1"].lastAssistant
	b.mu.Unlock()
	if turn != "abcdef" {
		t.Errorf("turnAssistant = %q", turn)
	}
	if last != "" {
		t.Errorf("lastAssistant = %q, want empty before completion", last)
	}
	if len(fr.events) != 2 {
		t.Errorf("expected 2 events, got %d", len(fr.events))
	}
}

func TestHandleAgentMessageDeltaLegacyTextOnly(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","text":"legacy"}`))

	b.mu.Lock()
	turn := b.frames["f1"].turnAssistant
	b.mu.Unlock()
	if turn != "legacy" {
		t.Fatalf("turnAssistant = %q, want %q", turn, "legacy")
	}
	if len(fr.events) != 1 {
		t.Fatalf("events = %d, want 1", len(fr.events))
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Payload.LastAssistantMessage != "legacy" {
		t.Fatalf("LastAssistantMessage = %q, want %q", ev.Payload.LastAssistantMessage, "legacy")
	}
}

func TestHandleAgentMessageDeltaDropsStaleTurnDelta(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", activeTurnID: "turn-2"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","turnId":"turn-1","delta":"stale"}`))

	b.mu.Lock()
	turn := b.frames["f1"].turnAssistant
	b.mu.Unlock()
	if turn != "" {
		t.Fatalf("turnAssistant = %q, want empty", turn)
	}
	if len(fr.events) != 0 {
		t.Fatalf("events = %d, want 0", len(fr.events))
	}
}

func TestHandleTurnCompletedIgnoresStaleDeltaFromPreviousTurn(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnStarted([]byte(`{"threadId":"t1","turn":{"id":"turn-1","items":[],"status":"inProgress"}}`))
	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","turnId":"turn-1","delta":"first"}`))
	b.handleTurnStarted([]byte(`{"threadId":"t1","turn":{"id":"turn-2","items":[],"status":"inProgress"}}`))
	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","turnId":"turn-1","delta":" stale"}`))
	b.handleTurnCompleted([]byte(`{"threadId":"t1","turn":{"id":"turn-2","items":[],"itemsView":"notLoaded","status":"completed"}}`))

	ev := fr.events[len(fr.events)-1].(state.EvSubsystem)
	if ev.Payload.LastAssistantMessage != "" {
		t.Fatalf("LastAssistantMessage = %q, want empty", ev.Payload.LastAssistantMessage)
	}
	b.mu.Lock()
	history := append([]state.SubsystemTurn(nil), b.frames["f1"].history...)
	b.mu.Unlock()
	if len(history) != 0 {
		t.Fatalf("history = %+v, want empty", history)
	}
}

func TestHandleAgentMessageDeltaIgnored(t *testing.T) {
	b, fr := newTestBackend()
	b.handleAgentMessageDelta([]byte(`bad`))           // bad json
	b.handleAgentMessageDelta([]byte(`{}`))            // no text
	b.handleAgentMessageDelta([]byte(`{"delta":"x"}`)) // no thread match
	if len(fr.events) != 0 {
		t.Errorf("expected no events, got %d", len(fr.events))
	}
}

func TestHandleThreadNameUpdated(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadNameUpdated([]byte(`{"threadId":"t1","threadName":" saved-session "}`))
	if len(fr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fr.events))
	}
	ev, ok := fr.events[0].(state.EvSubsystem)
	if !ok {
		t.Fatalf("event = %T, want EvSubsystem", fr.events[0])
	}
	if ev.Kind != state.SubsystemMetadataUpdated {
		t.Fatalf("Kind = %q, want %q", ev.Kind, state.SubsystemMetadataUpdated)
	}
	if ev.Payload.Title != "saved-session" {
		t.Fatalf("Title = %q", ev.Payload.Title)
	}
	if !ev.Payload.TitleSet {
		t.Fatal("TitleSet = false, want true")
	}
}

func TestHandleThreadNameUpdatedEmptyAndUnknown(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadNameUpdated([]byte(`{"threadId":"unknown","threadName":"ignored"}`))
	b.handleThreadNameUpdated([]byte(`{"threadId":"t1","threadName":null}`))
	b.handleThreadNameUpdated([]byte(`{"threadId":"t1","threadName":""}`))
	if len(fr.events) != 2 {
		t.Fatalf("expected two title clear metadata events, got %d", len(fr.events))
	}
	for _, raw := range fr.events {
		ev := raw.(state.EvSubsystem)
		if ev.Kind != state.SubsystemMetadataUpdated || !ev.Payload.TitleSet || ev.Payload.Title != "" {
			t.Fatalf("clear metadata event = %+v", ev)
		}
	}
}

func TestHandleNotificationUnknownMethodIsNoop(t *testing.T) {
	b, fr := newTestBackend()
	b.handleNotification("unknown/method", []byte(`{}`))
	if len(fr.events) != 0 {
		t.Errorf("unknown method should emit nothing, got %d events", len(fr.events))
	}
}

func TestHandleTurnStartedEmitsMetadataBeforeTurnStarted(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleNotification("turn/started", []byte(`{
		"threadId":"t1",
		"turn":{
			"id":"tu1",
			"items":[{"id":"u1","type":"userMessage","content":[{"type":"text","text":"diagnose the app"}]}],
			"status":"inProgress"
		}
	}`))
	if len(fr.events) != 2 {
		t.Fatalf("expected metadata + turn events, got %d", len(fr.events))
	}
	meta := fr.events[0].(state.EvSubsystem)
	if meta.Kind != state.SubsystemMetadataUpdated || meta.Payload.Prompt != "diagnose the app" {
		t.Fatalf("metadata event = %+v", meta)
	}
	started := fr.events[1].(state.EvSubsystem)
	if started.Kind != state.SubsystemTurnStarted || started.Payload.TurnID != "tu1" {
		t.Fatalf("turn event = %+v", started)
	}
}

func TestHandleTurnStartedEmitsMetadataWhenOnlyPreview(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleNotification("turn/started", []byte(`{"threadId":"t1","preview":"live preview","turn":{"id":"tu1","items":[]}}`))
	if len(fr.events) != 2 {
		t.Fatalf("events = %d, want 2", len(fr.events))
	}
	meta := fr.events[0].(state.EvSubsystem)
	if meta.Kind != state.SubsystemMetadataUpdated || meta.Payload.Preview != "live preview" {
		t.Fatalf("metadata event = %+v", meta)
	}
}

func TestHandleNotificationRoutesToHandlers(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	for _, method := range []string{"turn/started", "turn/plan/updated", "turn/diff/updated", "thread/name/updated", "thread/settings/updated"} {
		params := []byte(`{"threadId":"t1"}`)
		if method == "thread/name/updated" {
			params = []byte(`{"threadId":"t1","threadName":"title"}`)
		}
		if method == "thread/settings/updated" {
			params = []byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":{"level":"high"}}}`)
		}
		b.handleNotification(method, params)
	}
	if len(fr.events) != 5 {
		t.Errorf("expected 5 events from known methods, got %d", len(fr.events))
	}
}

func TestHandleThreadSettingsUpdated(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadSettingsUpdated([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":{"level":"high"}}}`))
	if len(fr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fr.events))
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Kind != state.SubsystemMetadataUpdated || ev.Payload.Model != "gpt-5" || ev.Payload.Effort != "high" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestHandleThreadSettingsUpdatedBecomesAuthoritativeForLaterPayloads(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", model: "gpt-4.1", effort: "low"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadSettingsUpdated([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":{"level":"high"}}}`))

	payload := b.payload("f1")
	if payload.Model != "gpt-5" || payload.Effort != "high" {
		t.Fatalf("payload = %+v, want updated model/effort to persist", payload)
	}
}

func TestHandleThreadNameUpdatedDoesNotPromoteSeedMetadata(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", model: "gpt-4.1", modelSet: true, effort: "low", effortSet: true}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadNameUpdated([]byte(`{"threadId":"t1","threadName":"saved name"}`))
	if len(fr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fr.events))
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Kind != state.SubsystemMetadataUpdated {
		t.Fatalf("Kind = %q, want metadata_updated", ev.Kind)
	}
	if ev.Payload.ModelSet || ev.Payload.EffortSet {
		t.Fatalf("payload unexpectedly promoted seed metadata: %+v", ev.Payload)
	}
	if ev.Payload.Model != "" || ev.Payload.Effort != "" {
		t.Fatalf("payload unexpectedly carried seed metadata: %+v", ev.Payload)
	}
}

func TestHandleThreadSettingsUpdatedClearsEffortWhenServerSendsNull(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", model: "gpt-5", effort: "high"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadSettingsUpdated([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":null}}`))
	if len(fr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fr.events))
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Kind != state.SubsystemMetadataUpdated {
		t.Fatalf("Kind = %q, want metadata_updated", ev.Kind)
	}
	if !ev.Payload.EffortSet {
		t.Fatalf("EffortSet = false, want true: %+v", ev.Payload)
	}
	if ev.Payload.Effort != "" {
		t.Fatalf("Effort = %q, want cleared", ev.Payload.Effort)
	}

	payload := b.payload("f1")
	if !payload.EffortSet {
		t.Fatalf("payload EffortSet = false, want true: %+v", payload)
	}
	if payload.Effort != "" {
		t.Fatalf("payload Effort = %q, want cleared", payload.Effort)
	}
}

func TestHandleThreadSettingsUpdatedUsesReasoningEffortAliasWhenEffortIsNull(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", model: "gpt-5", effort: "low"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleThreadSettingsUpdated([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":null,"reasoning_effort":{"level":"high"}}}`))
	if len(fr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fr.events))
	}
	ev := fr.events[0].(state.EvSubsystem)
	if ev.Kind != state.SubsystemMetadataUpdated {
		t.Fatalf("Kind = %q, want metadata_updated", ev.Kind)
	}
	if !ev.Payload.EffortSet {
		t.Fatalf("EffortSet = false, want true: %+v", ev.Payload)
	}
	if ev.Payload.Effort != "high" {
		t.Fatalf("Effort = %q, want alias high", ev.Payload.Effort)
	}

	payload := b.payload("f1")
	if !payload.EffortSet {
		t.Fatalf("payload EffortSet = false, want true: %+v", payload)
	}
	if payload.Effort != "high" {
		t.Fatalf("payload Effort = %q, want alias high", payload.Effort)
	}
}

func TestHandleThreadSettingsUpdatedDoesNotBleedAcrossThreads(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1", model: "gpt-4.1", effort: "low"}
	b.frames["f2"] = &frameBinding{frameID: "f2", threadID: "t2", model: "gpt-4o", effort: "medium"}
	b.threads["t1"] = "f1"
	b.threads["t2"] = "f2"
	b.mu.Unlock()

	b.handleThreadSettingsUpdated([]byte(`{"threadId":"t1","threadSettings":{"model":"gpt-5","effort":{"level":"high"}}}`))

	payloadA := b.payload("f1")
	payloadB := b.payload("f2")
	if payloadA.Model != "gpt-5" || payloadA.Effort != "high" {
		t.Fatalf("payloadA = %+v", payloadA)
	}
	if payloadB.Model != "gpt-4o" || payloadB.Effort != "medium" {
		t.Fatalf("payloadB = %+v, want untouched sibling thread settings", payloadB)
	}
}

func TestFailFrame(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.failFrame("f1", nil)
	if len(fr.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(fr.events))
	}
	// duplicate suppressed
	b.failFrame("f1", nil)
	if len(fr.events) != 1 {
		t.Errorf("duplicate failFrame should be suppressed, got %d", len(fr.events))
	}
	// unknown frame is no-op
	b.failFrame("unknown", nil)
	if len(fr.events) != 1 {
		t.Errorf("unknown frame: got %d events", len(fr.events))
	}
}

func TestEmitToThreadUnknown(t *testing.T) {
	b, fr := newTestBackend()
	b.emitToThread("unknown", state.SubsystemTurnStarted, nil)
	if len(fr.events) != 0 {
		t.Errorf("unknown thread should emit nothing")
	}
}

func TestPayloadFromBinding(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{
		frameID:     "f1",
		threadID:    "t1",
		sessionID:   "sess-1",
		requestedID: "req",
		observedID:  "obs",
		resumePhase: "phase",
	}
	b.mu.Unlock()
	p := b.payload("f1")
	if p.SessionID != "t1" || p.ColdStartSessionID != "sess-1" || p.RequestedTargetID != "req" || p.ResumePhase != "phase" {
		t.Errorf("payload = %+v", p)
	}
	// Unknown frame: empty payload
	pe := b.payload("missing")
	if pe.SessionID != "" {
		t.Errorf("missing frame should produce empty payload: %+v", pe)
	}
}
