package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/proto"
)

// human-input roundtrip contract pins (contract-human-input-roundtrip / UAC-043 / FR-008).
// Production code is not changed; these assert as-is gateway behaviour:
// hello carries no pending, ar/ax frames are live-only, REST conflict is 409.

func sampleApprovalWire(id, status, decision string) proto.ApprovalWire {
	return proto.ApprovalWire{
		ID:              id,
		SessionID:       "sess-1",
		FrameID:         "frame-1",
		Kind:            "command",
		Command:         "rm -rf /",
		Status:          status,
		DefaultDecision: "deny",
		Decision:        decision,
	}
}

// fanoutLifecycleAttacher fans each published event to every active lifecycle
// subscriber (one channel per AttachLifecycleWS). The shared single-channel
// fakeLifecycleAttacher cannot express multi-client broadcast contracts.
type fanoutLifecycleAttacher struct {
	mu   sync.Mutex
	subs []chan proto.ServerEvent
}

func newFanoutLifecycleAttacher() *fanoutLifecycleAttacher {
	return &fanoutLifecycleAttacher{}
}

// publish fans an event to the currently-registered subscribers. The whole fan-out
// runs under the mutex so it cannot race an unsubscribe, and a saturated buffer
// fails the test instead of blocking it (a hang would surface as a package timeout
// that reads like a contract regression).
func (f *fanoutLifecycleAttacher) publish(t *testing.T, ev proto.ServerEvent) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, ch := range f.subs {
		select {
		case ch <- ev:
		default:
			t.Fatalf("subscriber %d buffer full publishing %T — test harness saturated", i, ev)
		}
	}
}

// waitSubscribers blocks until exactly n subscribers are registered. Used instead
// of a sleep so subscriber registration/teardown is a deterministic barrier.
func (f *fanoutLifecycleAttacher) waitSubscribers(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock()
		got := len(f.subs)
		f.mu.Unlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers = %d, want %d after 3s", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *fanoutLifecycleAttacher) SubscribeLifecycle(ctx context.Context) (<-chan proto.ServerEvent, error) {
	ch := make(chan proto.ServerEvent, 32)
	f.mu.Lock()
	f.subs = append(f.subs, ch)
	f.mu.Unlock()
	go func() {
		<-ctx.Done()
		// Deregister under the lock. Do not close(ch): publish holds the lock
		// while sending, and the reader exits on ctx cancellation, so closing
		// here would only risk a send-on-closed panic.
		f.mu.Lock()
		out := f.subs[:0]
		for _, s := range f.subs {
			if s != ch {
				out = append(out, s)
			}
		}
		f.subs = out
		f.mu.Unlock()
	}()
	return ch, nil
}

func (f *fanoutLifecycleAttacher) SubscribeSurface(context.Context, string, uint16, uint16) (<-chan proto.ServerEvent, error) {
	return nil, errors.New("not implemented")
}
func (f *fanoutLifecycleAttacher) UnsubscribeSurface(context.Context, string) error { return nil }
func (f *fanoutLifecycleAttacher) SendSurfaceSubscribe(context.Context, string, string, uint16, uint16) error {
	return nil
}
func (f *fanoutLifecycleAttacher) SendSurfaceUnsubscribe(context.Context, string, string) error {
	return nil
}
func (f *fanoutLifecycleAttacher) WriteRaw(context.Context, string, []byte) error { return nil }
func (f *fanoutLifecycleAttacher) Resize(context.Context, string, uint16, uint16) error {
	return nil
}
func (f *fanoutLifecycleAttacher) PushChannelFor(<-chan proto.ServerEvent) <-chan []byte {
	return make(chan []byte)
}

// forbiddenHelloSubstrings are matched as *substrings* of a lowercased field name
// or json tag. Exact-match lists rot: helloFrame uses camelCase tags
// (clientInstanceId, serverTime), so the idiomatic name for a new member would be
// "pendingApprovals" — a form an exhaustive literal list keeps missing. Substrings
// catch every separator style (pendingApprovals / pending_approvals / PendingApprovals).
//
// Deliberately NOT bare "pending" or "held": those are generic scheduling words that
// legitimately appear in unrelated hello state (e.g. FrameMessagingSummary
// .pending_delivery_count), and a blocklist that cries wolf gets deleted.
// UAC-043 / FR-008: reconnect must not fabricate pending cards.
var forbiddenHelloSubstrings = []string{"approval", "question", "humaninput", "human_input"}

func TestHelloFrameCarriesCompositeStatusButNotRequestPayloads(t *testing.T) {
	// Reconnect receives the current composite human-input fact, while full
	// approval/question payloads remain on their dedicated event channel.
	frame := encodeHelloFrame(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", 0)},
		Features: []string{"surface"},
	}, 1_700_000_100, "ci-hello")
	if frame == nil {
		t.Fatal("nil hello frame")
	}
	var raw map[string]any
	if err := json.Unmarshal(frame, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["k"] != "h" {
		t.Fatalf("k = %v, want h", raw["k"])
	}
	for key := range raw {
		for _, forbidden := range forbiddenHelloSubstrings {
			if strings.Contains(strings.ToLower(key), forbidden) {
				t.Fatalf("hello must not carry %q; got %v", key, raw[key])
			}
		}
	}
	sessions, _ := raw["sessions"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d", len(sessions))
	}
	sess0, _ := sessions[0].(map[string]any)
	for key := range sess0 {
		for _, forbidden := range forbiddenHelloSubstrings {
			if strings.Contains(strings.ToLower(key), forbidden) {
				t.Fatalf("hello session must not carry %q", key)
			}
		}
	}
}

func TestPendingHumanInputPlaceholderIsEmpty(t *testing.T) {
	// Pins the as-is fact that **no REST replay path exists yet**: the
	// pending-human-input endpoint is a placeholder that returns a body with no
	// members at all (handlePendingHumanInput writes proto.RespPendingHumanInput{}
	// and both fields are omitempty).
	//
	// Scope note: this is deliberately NOT cited as the pin for FR-008/UAC-043.
	// FR-008 forbids re-presenting *resolved* requests, whereas replay of
	// *unresolved* pending is an accepted future direction (design-plan.json
	// out_of_scope / DP-PENDING-REPLAY). When that lands this test is expected to
	// go red as a signal to restate the contract explicitly — it is a change
	// detector on the placeholder shape, not a non-fabrication proof.
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1/pending-human-input", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%q", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// Exact shape — an assertion that actually executes. Any populated member
	// (under any key name) fails, not just "approvals" / "questions".
	if len(body) != 0 {
		t.Fatalf("placeholder body = %#v, want no members (no REST replay path)", body)
	}
}

func TestGatewayLifecycle_ApprovalRequestedAndResolvedFrames(t *testing.T) {
	// witness-approve-roundtrip-normal (gateway surface): ar then ax on lifecycle WS.
	// witness-other-client-normal: both subscribers observe the same ax broadcast.
	t.Parallel()
	fake := newFanoutLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c1 := dialLifecycleWS(t, srv)
	c2 := dialLifecycleWS(t, srv)
	// Deterministic barrier: both AttachLifecycleWS goroutines have registered.
	fake.waitSubscribers(t, 2)

	// Seed hello so subsequent approval frames are not stuck behind helloSent=false.
	fake.publish(t, proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", 0)},
	})
	hello1 := readJSONFrame(t, c1)
	hello2 := readJSONFrame(t, c2)
	if hello1["k"] != "h" || hello2["k"] != "h" {
		t.Fatalf("expected hello on both clients, got k=%v / %v", hello1["k"], hello2["k"])
	}
	for _, h := range []map[string]any{hello1, hello2} {
		if _, has := h["approvals"]; has {
			t.Fatal("hello must not include approvals")
		}
	}

	pending := sampleApprovalWire("ap-1", "pending", "")
	fake.publish(t, proto.EvtApprovalRequested{Approval: pending})

	ar1 := readJSONFrame(t, c1)
	ar2 := readJSONFrame(t, c2)
	for i, ar := range []map[string]any{ar1, ar2} {
		if ar["k"] != "ar" {
			t.Fatalf("client %d: k = %v, want ar", i+1, ar["k"])
		}
		appr, _ := ar["approval"].(map[string]any)
		if appr["id"] != "ap-1" || appr["status"] != "pending" {
			t.Fatalf("client %d: approval = %#v", i+1, appr)
		}
	}

	resolved := sampleApprovalWire("ap-1", "resolved", "accept")
	resolved.ResolvingClientInstanceID = "ci-A"
	resolved.ResolutionReason = "client"
	fake.publish(t, proto.EvtApprovalResolved{Approval: resolved})

	ax1 := readJSONFrame(t, c1)
	ax2 := readJSONFrame(t, c2)
	for i, ax := range []map[string]any{ax1, ax2} {
		if ax["k"] != "ax" {
			t.Fatalf("client %d: k = %v, want ax", i+1, ax["k"])
		}
		appr, _ := ax["approval"].(map[string]any)
		if appr["id"] != "ap-1" || appr["status"] != "resolved" || appr["decision"] != "accept" {
			t.Fatalf("client %d: resolved approval = %#v", i+1, appr)
		}
	}
}

func TestGatewayLifecycle_ReconnectDoesNotReplayResolvedApproval(t *testing.T) {
	// UAC-043 / witness-race-adversarial: re-subscribe after resolve yields hello only;
	// already-resolved ar/ax are not re-emitted by the gateway.
	// rejects counterexample: hello or first view re-injects prior ar for ap-resolved
	t.Parallel()
	fake := newFanoutLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c1 := dialLifecycleWS(t, srv)
	fake.waitSubscribers(t, 1)

	fake.publish(t, proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", 0)},
	})
	if got := readJSONFrame(t, c1); got["k"] != "h" {
		t.Fatalf("first frame k=%v", got["k"])
	}

	fake.publish(t, proto.EvtApprovalRequested{Approval: sampleApprovalWire("ap-resolved", "pending", "")})
	if got := readJSONFrame(t, c1); got["k"] != "ar" {
		t.Fatalf("want ar, got %v", got["k"])
	}
	fake.publish(t, proto.EvtApprovalResolved{Approval: sampleApprovalWire("ap-resolved", "resolved", "accept")})
	if got := readJSONFrame(t, c1); got["k"] != "ax" {
		t.Fatalf("want ax, got %v", got["k"])
	}
	_ = c1.CloseNow()
	// Deterministic barrier: the server handler has dropped c1's subscriber.
	fake.waitSubscribers(t, 0)

	// New client connects: only hello from sessions-changed, never a synthetic ar.
	c2 := dialLifecycleWS(t, srv)
	fake.waitSubscribers(t, 1)
	fake.publish(t, proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", 0)},
	})
	hello := readJSONFrame(t, c2)
	if hello["k"] != "h" {
		t.Fatalf("reconnect first frame k=%v, want h", hello["k"])
	}
	if _, has := hello["approvals"]; has {
		t.Fatal("reconnect hello must not carry approvals")
	}

	// Next live sessions-changed is a view-update (k=v), not a replayed ar for ap-resolved.
	fake.publish(t, proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", 0)},
	})
	next := readJSONFrame(t, c2)
	if next["k"] == "ar" || next["k"] == "ax" {
		t.Fatalf("reconnect must not replay approval frames, got k=%v frame=%#v", next["k"], next)
	}
	if next["k"] != "v" {
		t.Fatalf("expected view-update after reconnect hello, got k=%v", next["k"])
	}

	// A fresh live request after reconnect is still deliverable (live path only).
	fake.publish(t, proto.EvtApprovalRequested{Approval: sampleApprovalWire("ap-new", "pending", "")})
	live := readJSONFrame(t, c2)
	if live["k"] != "ar" {
		t.Fatalf("live ar after reconnect: k=%v", live["k"])
	}
	appr, _ := live["approval"].(map[string]any)
	if appr["id"] != "ap-new" {
		t.Fatalf("live ar id = %v, want ap-new (not ap-resolved)", appr["id"])
	}
}

func TestMux_ApprovalRespondFirstWriterWinsHTTP(t *testing.T) {
	// witness-other-client-normal (HTTP surface): winner 200, loser 409 resolved_by_other (not 5xx).
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	// First respond: accept.
	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		env := daemon.recv()
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		respond, ok := cmd.(proto.CmdApprovalRespond)
		if !ok {
			t.Errorf("cmd = %T, want CmdApprovalRespond", cmd)
			return
		}
		if respond.ApprovalID != "ap-1" || respond.Decision != "accept" {
			t.Errorf("respond = %+v", respond)
		}
		if respond.ClientInstanceID != "ci-A" {
			t.Errorf("client_instance_id = %q", respond.ClientInstanceID)
		}
		daemon.sendResp(env.ReqID, nil)
	}()

	req1 := httptest.NewRequest(http.MethodPost, "/api/sessions/sess-1/approvals/ap-1",
		bytes.NewBufferString(`{"decision":"accept","client_instance_id":"ci-A"}`))
	req1.Header.Set("Authorization", "Bearer tok")
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)
	<-done1
	if w1.Code != http.StatusOK {
		t.Fatalf("winner status = %d body=%q", w1.Code, w1.Body.String())
	}

	// Second respond: daemon returns resolved_by_other → HTTP 409 (not 5xx).
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		env := daemon.recv()
		daemon.sendErr(env.ReqID, proto.ErrResolvedByOther, "approval already resolved by another client")
	}()

	req2 := httptest.NewRequest(http.MethodPost, "/api/sessions/sess-1/approvals/ap-1",
		bytes.NewBufferString(`{"decision":"deny","client_instance_id":"ci-B"}`))
	req2.Header.Set("Authorization", "Bearer tok")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	<-done2
	// Forbidden outcome first (witness-other-client-normal: "後着への 5xx"), so the
	// forbidden-outcome check is the primary assertion rather than dead code behind
	// a stricter equality check.
	if w2.Code >= 500 {
		t.Fatalf("conflict must not be 5xx, got %d body=%q", w2.Code, w2.Body.String())
	}
	if w2.Code != http.StatusConflict {
		t.Fatalf("loser status = %d, want 409 body=%q", w2.Code, w2.Body.String())
	}
	var errBody map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody["code"] != string(proto.ErrResolvedByOther) {
		t.Fatalf("error code = %v, want resolved_by_other", errBody["code"])
	}
}

func TestMux_ApprovalRespondInvalidDecision400(t *testing.T) {
	// witness-race-adversarial (malformed decision): rejected at gateway before daemon RPC.
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/sess-1/approvals/ap-1",
		bytes.NewBufferString(`{"decision":"maybe"}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%q", w.Code, w.Body.String())
	}
	// No daemon RPC should have been issued for invalid_argument at the edge.
	assertNoDaemonRPC(t, daemon)
}

func TestWireEncode_ApprovalFramesArAx(t *testing.T) {
	// Pin ar/ax wire shape used by lifecycle fan-out.
	ar := encodeServerEvent(proto.EvtApprovalRequested{Approval: sampleApprovalWire("ap-1", "pending", "")})
	if ar == nil {
		t.Fatal("nil ar frame")
	}
	var arm map[string]any
	if err := json.Unmarshal(ar, &arm); err != nil {
		t.Fatal(err)
	}
	if arm["k"] != "ar" {
		t.Fatalf("k = %v, want ar", arm["k"])
	}
	ax := encodeServerEvent(proto.EvtApprovalResolved{Approval: sampleApprovalWire("ap-1", "resolved", "deny")})
	if ax == nil {
		t.Fatal("nil ax frame")
	}
	var axm map[string]any
	if err := json.Unmarshal(ax, &axm); err != nil {
		t.Fatal(err)
	}
	if axm["k"] != "ax" {
		t.Fatalf("k = %v, want ax", axm["k"])
	}
}

// islandRequiredApprovalKeys / islandRequiredQuestionKeys are the json keys the
// island's WsFrameParser.cs actually reads off approval/question wire payloads
// (clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/WsFrameParser.cs). They
// are the C# consumer's contract on the Go producer's wire shape.
var islandRequiredApprovalKeys = []string{
	"id", "session_id", "command", "status", "decision",
	"expires_at", "resolving_client_instance_id",
}

var islandRequiredQuestionKeys = []string{
	"id", "session_id", "prompt", "status", "answer",
	"expires_at", "resolving_client_instance_id",
}

// jsonKeySet returns the set of json tag names (first segment, ignoring
// omitempty/etc.) declared on typ's exported fields.
func jsonKeySet(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	keys := make(map[string]bool)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			t.Fatalf("%s.%s has no json tag; the island parser keys off tag names, not Go field names",
				typ.Name(), field.Name)
		}
		keys[tag] = true
	}
	return keys
}

func TestApprovalWireCarriesIslandRequiredKeys(t *testing.T) {
	// Cross-task wire contract: WsFrameParserTests (island side, C#) hand-writes JSON
	// literals keyed on these exact strings. Nothing on the Go side previously tied
	// proto.ApprovalWire's json tags to that literal set — renaming session_id to
	// sessionId here would leave every Go test green while every island fixture
	// silently stopped matching (TryGetProperty/ReqString return null-not-found, not
	// an error, so the island would just drop session attribution). Pin the key set
	// so a rename fails at the Go end, not silently at the C# end.
	got := jsonKeySet(t, reflect.TypeOf(proto.ApprovalWire{}))
	for _, want := range islandRequiredApprovalKeys {
		if !got[want] {
			t.Errorf("ApprovalWire lost json key %q that clients/windows-shell/.../WsFrameParser.cs requires", want)
		}
	}
}

func TestQuestionWireCarriesIslandRequiredKeys(t *testing.T) {
	got := jsonKeySet(t, reflect.TypeOf(proto.QuestionWire{}))
	for _, want := range islandRequiredQuestionKeys {
		if !got[want] {
			t.Errorf("QuestionWire lost json key %q that clients/windows-shell/.../WsFrameParser.cs requires", want)
		}
	}
}
