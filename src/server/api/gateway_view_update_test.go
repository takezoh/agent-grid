package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/client/proto"
	stateview "github.com/takezoh/agent-grid/client/state/view"
)

// fakeLifecycleAttacher is a test double for Attacher that supports
// lifecycle subscriptions. SubscribeLifecycle returns a channel the
// test controls; all surface/input methods are no-ops.
type fakeLifecycleAttacher struct {
	mu              sync.Mutex
	events          chan proto.ServerEvent
	subscribeErr    error
	seedOnSubscribe *proto.EvtSurfaceOutput
	subscribeCols   uint16
	subscribeRows   uint16
	subscribeCalls  int
}

func newFakeLifecycleAttacher() *fakeLifecycleAttacher {
	return &fakeLifecycleAttacher{
		events: make(chan proto.ServerEvent, 16),
	}
}

func (f *fakeLifecycleAttacher) SubscribeLifecycle(_ context.Context) (<-chan proto.ServerEvent, error) {
	if f.subscribeErr != nil {
		return nil, f.subscribeErr
	}
	return f.events, nil
}

func (f *fakeLifecycleAttacher) SubscribeSurface(_ context.Context, _ string, _, _ uint16) (<-chan proto.ServerEvent, error) {
	return nil, errors.New("not implemented in lifecycle fake")
}

func (f *fakeLifecycleAttacher) UnsubscribeSurface(_ context.Context, _ string) error { return nil }

func (f *fakeLifecycleAttacher) SendSurfaceSubscribe(ctx context.Context, _ string, subscriberID string, cols, rows uint16) error {
	f.mu.Lock()
	f.subscribeCols = cols
	f.subscribeRows = rows
	f.subscribeCalls++
	f.mu.Unlock()
	if f.seedOnSubscribe == nil {
		return nil
	}
	seed := *f.seedOnSubscribe
	seed.SubscriberID = subscriberID
	f.events <- seed
	for len(f.events) != 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	return nil
}

func (f *fakeLifecycleAttacher) SendSurfaceUnsubscribe(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeLifecycleAttacher) WriteRaw(_ context.Context, _ string, _ []byte) error { return nil }

func (f *fakeLifecycleAttacher) Resize(_ context.Context, _ string, _ uint16, _ uint16) error {
	return nil
}

func (f *fakeLifecycleAttacher) PushChannelFor(_ <-chan proto.ServerEvent) <-chan []byte {
	return make(chan []byte) // open, idle — avoids spurious daemon-disconnect in select
}

// startLifecycleServer starts an httptest server that calls AttachLifecycleWS
// directly (no ticket check) so tests can dial without auth plumbing.
func startLifecycleServer(t *testing.T, sess Attacher) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = AttachLifecycleWS(r.Context(), sess, c)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func dialLifecycleWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial lifecycle WS: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// readJSONFrame reads one WS text frame and unmarshals it as a map.
// Uses a fixed 3-second deadline appropriate for protofake-driven tests.
func readJSONFrame(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read WS frame: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal frame %q: %v", data, err)
	}
	return m
}

// sampleSession builds a SessionInfo with the given ID and title for tests.
func sampleSession(id, title string, status stateview.Status) proto.SessionInfo {
	return proto.SessionInfo{
		ID:      id,
		Project: "proj",
		Command: "claude",
		View: stateview.View{
			Card:   stateview.Card{Title: title},
			Status: status,
		},
	}
}

// TestGatewayLifecycle_EmitsHelloFirst verifies that the first frame sent on a
// lifecycle WebSocket has k:"h" and contains the seeded session data.
func TestGatewayLifecycle_EmitsHelloFirst(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
		Features: []string{"surface"},
	}

	m := readJSONFrame(t, c)

	if m["k"] != "h" {
		t.Errorf("first frame k = %q, want \"h\"", m["k"])
	}
	// The hello frame no longer carries a daemon-side active session id — web
	// clients own their own selection. Assert absence to lock the contract.
	if _, has := m["activeSessionID"]; has {
		t.Errorf("hello frame must not carry activeSessionID; got: %v", m["activeSessionID"])
	}
	sessions, ok := m["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Fatalf("sessions missing or empty: %v", m["sessions"])
	}
	sess0, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0] is not object: %T", sessions[0])
	}
	viewObj, ok := sess0["view"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view is not object: %T", sess0["view"])
	}
	cardObj, ok := viewObj["card"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view.card is not object: %T", viewObj["card"])
	}
	if cardObj["title"] != "T" {
		t.Errorf("sessions[0].view.card.title = %q, want \"T\"", cardObj["title"])
	}
}

// TestGatewayLifecycle_BroadcastsViewUpdate verifies that the second event is
// sent as a view-update frame (k:"v") after the hello frame.
func TestGatewayLifecycle_BroadcastsViewUpdate(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// First event → hello frame.
	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
		Features: []string{"surface"},
	}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	// Second event → view-update frame. The daemon no longer ships an active
	// session id at all — web clients own their own selection. Assert absence
	// on the view-update frame to lock the contract.
	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s2", "U", stateview.StatusIdle)},
	}
	m := readJSONFrame(t, c)

	if m["k"] != "v" {
		t.Errorf("second frame k = %q, want \"v\"", m["k"])
	}
	if _, has := m["activeSessionID"]; has {
		t.Errorf("view-update frame must not carry activeSessionID; got: %v", m["activeSessionID"])
	}
	sessions, ok := m["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Fatalf("sessions missing or empty: %v", m["sessions"])
	}
	sess0, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0] is not object: %T", sessions[0])
	}
	viewObj, ok := sess0["view"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view is not object: %T", sess0["view"])
	}
	if viewObj["status"] != "idle" {
		t.Errorf("sessions[0].view.status = %q, want \"idle\"", viewObj["status"])
	}
}

// TestGatewayLifecycle_SubscribeSeedIsNotDropped pins the ordering contract at
// the daemon/gateway boundary: a surface subscription may synchronously emit
// its initial terminal snapshot before SendSurfaceSubscribe returns.
func TestGatewayLifecycle_SubscribeSeedIsNotDropped(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString([]byte("seed"))
	fake := newFakeLifecycleAttacher()
	fake.seedOnSubscribe = &proto.EvtSurfaceOutput{
		SessionID: "s1",
		TimeSec:   0.5,
		DataB64:   encoded,
	}
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"s","reqId":"r1","sessionId":"s1","cols":120,"rows":40}`)); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	var seedFrame []any
	for range 2 {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read subscribe result: %v", err)
		}
		var frame []any
		if json.Unmarshal(data, &frame) == nil {
			seedFrame = frame
		}
	}
	if len(seedFrame) != 4 || seedFrame[1] != "o" || seedFrame[2] != encoded || seedFrame[3] != "s1" {
		t.Fatalf("frames did not contain terminal seed for s1: %v", seedFrame)
	}
	fake.mu.Lock()
	cols, rows := fake.subscribeCols, fake.subscribeRows
	fake.mu.Unlock()
	if cols != 120 || rows != 40 {
		t.Fatalf("subscribe geometry = %dx%d, want 120x40", cols, rows)
	}
}

func TestGatewayLifecycle_SubscribeRejectsMissingGeometryBeforeDaemonCall(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"s","reqId":"r1","sessionId":"s1"}`)); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read subscribe error: %v", err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["k"] != "e" || response["code"] != "invalid_argument" {
		t.Fatalf("response = %v, want invalid_argument", response)
	}
	fake.mu.Lock()
	calls := fake.subscribeCalls
	fake.mu.Unlock()
	if calls != 0 {
		t.Fatalf("daemon subscribe calls = %d, want 0", calls)
	}
}

// TestGatewayLifecycle_BroadcastsActivityViewUpdate verifies activity events
// are forwarded as a partial view-update without clobbering sessions.
func TestGatewayLifecycle_BroadcastsActivityViewUpdate(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	fake.events <- proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
		Features: []string{"surface"},
	}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	fake.events <- proto.EvtActivityEvents{
		SessionID: "s1",
		Events: []proto.ActivityEventWire{{
			Type: "mid_turn_touch", Sequence: 1, SessionID: "s1",
			Path: "src/foo.ts", ToolUseID: "tc1",
		}},
	}
	m := readJSONFrame(t, c)

	if m["k"] != "v" {
		t.Errorf("activity frame k = %q, want \"v\"", m["k"])
	}
	if m["activity_session_id"] != "s1" {
		t.Errorf("activity_session_id = %v, want \"s1\"", m["activity_session_id"])
	}
	if _, has := m["sessions"]; has {
		t.Errorf("activity-only frame must omit sessions; got %v", m["sessions"])
	}
	events, ok := m["activity_events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("activity_events: got %v", m["activity_events"])
	}
	ev0 := events[0].(map[string]any)
	if ev0["type"] != "mid_turn_touch" || ev0["tool_call_id"] != "tc1" {
		t.Errorf("activity event: got %v", ev0)
	}
}

// TestGatewayLifecycle_DaemonDisconnect verifies the 2-step close protocol
// (ADR 0011) when the daemon events channel closes on a lifecycle WS.
func TestGatewayLifecycle_DaemonDisconnect(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Simulate daemon disconnect.
	close(fake.events)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First message: control frame {"k":"c","data":"daemon-disconnected"}.
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("expected control frame before close, got: %v", err)
	}
	var msg controlMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal control frame: %v", err)
	}
	if msg.K != "c" || msg.Data != "daemon-disconnected" {
		t.Errorf("control frame = %+v, want {K:\"c\", Data:\"daemon-disconnected\"}", msg)
	}

	// Next read: StatusGoingAway typed close.
	_, _, err = c.Read(ctx)
	if err == nil {
		t.Fatal("expected close error, got nil")
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CloseError, got %T: %v", err, err)
	}
	if ce.Code != websocket.StatusGoingAway {
		t.Errorf("close code = %v, want StatusGoingAway", ce.Code)
	}
}
