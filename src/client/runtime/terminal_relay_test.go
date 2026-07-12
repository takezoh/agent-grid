package runtime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/termvt"
)

// fakeSurfaceBackend is a test double for SurfaceBackend. Each call to
// SubscribeSurface allocates a new subscriber id and a fresh buffered channel.
// Tests can push events into the channel via Send, or close it via Close to
// simulate termvt slow-close.
type fakeSurfaceBackend struct {
	mu      sync.Mutex
	nextID  atomic.Int32
	subs    map[int]fakeSurfaceSub // id → subscription
	written []writeCall
	resized []resizeCall

	defaultBuffer int
}

type fakeSurfaceSub struct {
	frameID string
	ch      chan termvt.Event
}

type writeCall struct {
	frameID string
	data    []byte
}

type resizeCall struct {
	frameID string
	cols    int
	rows    int
}

func newFakeSurfaceBackend() *fakeSurfaceBackend {
	return &fakeSurfaceBackend{
		subs:          make(map[int]fakeSurfaceSub),
		defaultBuffer: 32,
	}
}

func (f *fakeSurfaceBackend) SubscribeSurface(frameID string) (int, <-chan termvt.Event, error) {
	return f.SubscribeSurfaceWithBuffer(frameID, f.defaultBuffer)
}

func (f *fakeSurfaceBackend) SubscribeSurfaceWithBuffer(
	frameID string,
	buffer int,
) (int, <-chan termvt.Event, error) {
	id := int(f.nextID.Add(1))
	if buffer <= 0 {
		buffer = f.defaultBuffer
	}
	ch := make(chan termvt.Event, buffer)
	f.mu.Lock()
	f.subs[id] = fakeSurfaceSub{frameID: frameID, ch: ch}
	f.mu.Unlock()
	return id, ch, nil
}

func (f *fakeSurfaceBackend) UnsubscribeSurface(frameID string, id int) error {
	f.mu.Lock()
	sub, ok := f.subs[id]
	if ok && sub.frameID == frameID {
		delete(f.subs, id)
	}
	f.mu.Unlock()
	return nil
}

func (f *fakeSurfaceBackend) WriteSurface(frameID string, data []byte) error {
	f.mu.Lock()
	f.written = append(f.written, writeCall{frameID: frameID, data: data})
	f.mu.Unlock()
	return nil
}

func (f *fakeSurfaceBackend) ResizeSurface(frameID string, cols, rows int) error {
	f.mu.Lock()
	f.resized = append(f.resized, resizeCall{frameID: frameID, cols: cols, rows: rows})
	f.mu.Unlock()
	return nil
}

// Send pushes ev into the channel for the given subscriber id.
func (f *fakeSurfaceBackend) Send(id int, ev termvt.Event) {
	f.mu.Lock()
	sub, ok := f.subs[id]
	f.mu.Unlock()
	if ok {
		select {
		case sub.ch <- ev:
		default:
			f.CloseID(id)
		}
	}
}

// Broadcast pushes ev to every subscriber on frameID. Subscribers whose
// buffers are full are severed, mirroring termvt's fan-out contract.
func (f *fakeSurfaceBackend) Broadcast(frameID string, ev termvt.Event) {
	f.mu.Lock()
	ids := make([]int, 0, len(f.subs))
	for id, sub := range f.subs {
		if sub.frameID == frameID {
			ids = append(ids, id)
		}
	}
	f.mu.Unlock()
	for _, id := range ids {
		f.Send(id, ev)
	}
}

// CloseID closes the channel for the given subscriber id, simulating termvt
// slow-close on process exit.
func (f *fakeSurfaceBackend) CloseID(id int) {
	f.mu.Lock()
	sub, ok := f.subs[id]
	if ok {
		delete(f.subs, id)
	}
	f.mu.Unlock()
	if ok {
		close(sub.ch)
	}
}

// --- helpers ---

// collectEvents drains the send channel until timeout or n events are received.
func collectEvents(t *testing.T, ch <-chan internalEvent, n int, timeout time.Duration) []internalEvent {
	t.Helper()
	var out []internalEvent
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}

// collectUntilSurfaceClosed reads events until an internalSurfaceClosed
// arrives (inclusive) or the timeout expires.
func collectUntilSurfaceClosed(t *testing.T, ch <-chan internalEvent, timeout time.Duration) []internalEvent {
	t.Helper()
	var out []internalEvent
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
			if _, ok := ev.(internalSurfaceClosed); ok {
				return out
			}
		case <-deadline:
			t.Fatalf("timeout waiting for internalSurfaceClosed; got %d events", len(out))
			return out
		}
	}
}

// newTestTerminalRelay wires up a TerminalRelay whose send function posts to
// the returned channel, making test assertions straightforward.
func newTestTerminalRelay(t *testing.T, b SurfaceBackend) (*TerminalRelay, <-chan internalEvent) {
	t.Helper()
	ch := make(chan internalEvent, 64)
	send := func(ev internalEvent) bool { ch <- ev; return true }
	sendNow := func(ev internalEvent) { ch <- ev }
	return NewTerminalRelay(b, send, sendNow), ch
}

func newTestTerminalRelayWithOptions(
	t *testing.T,
	b SurfaceBackend,
	send func(internalEvent) bool,
	sendNow func(internalEvent),
	opts ...TerminalRelayOption,
) *TerminalRelay {
	t.Helper()
	return NewTerminalRelay(b, send, sendNow, opts...)
}

const (
	conn1 = state.ConnID(1)
	conn2 = state.ConnID(2)
	sess1 = state.SessionID("sess-1")
	sess2 = state.SessionID("sess-2")
)

// TestTerminalRelay_SnapshotSequenceZero: first EventOutput gets Sequence == 0
// and Data is preserved correctly.
func TestTerminalRelay_SnapshotSequenceZero(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	id := int(b.nextID.Load())
	payload := []byte("hello snapshot")
	b.Send(id, termvt.Event{Kind: termvt.EventOutput, Data: payload})

	got := collectEvents(t, events, 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	bs, ok := got[0].(internalBroadcastSurface)
	if !ok {
		t.Fatalf("expected internalBroadcastSurface, got %T", got[0])
	}
	if bs.Sequence != 0 {
		t.Errorf("Sequence = %d, want 0", bs.Sequence)
	}
	if string(bs.Data) != string(payload) {
		t.Errorf("Data = %q, want %q", bs.Data, payload)
	}
	if bs.ConnID != conn1 {
		t.Errorf("ConnID = %v, want %v", bs.ConnID, conn1)
	}
	if bs.SessionID != sess1 {
		t.Errorf("SessionID = %v, want %v", bs.SessionID, sess1)
	}
}

// TestTerminalRelay_SequenceMonotonic: Sequence increments 0,1,2,3 across
// four consecutive EventOutput events on the same subscription.
func TestTerminalRelay_SequenceMonotonic(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id := int(b.nextID.Load())

	for i := 0; i < 4; i++ {
		b.Send(id, termvt.Event{Kind: termvt.EventOutput, Data: []byte{byte(i)}})
	}

	got := collectEvents(t, events, 4, time.Second)
	if len(got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got))
	}
	for i, ev := range got {
		bs, ok := ev.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("event[%d]: expected internalBroadcastSurface, got %T", i, ev)
		}
		if bs.Sequence != uint64(i) {
			t.Errorf("event[%d]: Sequence = %d, want %d", i, bs.Sequence, i)
		}
	}
}

// TestTerminalRelay_SubscribeRestartsSequence: Unsubscribe then re-Subscribe
// on the same (ConnID, SessionID) resets Sequence to 0 (ADR 0010).
func TestTerminalRelay_SubscribeRestartsSequence(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id1 := int(b.nextID.Load())
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("a")})
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("b")})

	// Wait for both events to be delivered before unsubscribing.
	got := collectEvents(t, events, 2, time.Second)
	if len(got) != 2 {
		t.Fatalf("expected 2 events before Unsubscribe, got %d", len(got))
	}

	tr.Unsubscribe(conn1, sess1)

	// Re-subscribe — Sequence must restart from 0.
	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("re-Subscribe: %v", err)
	}
	id2 := int(b.nextID.Load())
	b.Send(id2, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c")})

	got2 := collectEvents(t, events, 1, time.Second)
	if len(got2) != 1 {
		t.Fatalf("expected 1 event after re-Subscribe, got %d", len(got2))
	}
	bs, ok := got2[0].(internalBroadcastSurface)
	if !ok {
		t.Fatalf("expected internalBroadcastSurface, got %T", got2[0])
	}
	if bs.Sequence != 0 {
		t.Errorf("Sequence after re-Subscribe = %d, want 0", bs.Sequence)
	}
}

func TestTerminalRelay_LogicalBrowserOwnersHaveIndependentSubscriptions(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.SubscribeOwned(conn1, sess1, "browser-a", "%1"); err != nil {
		t.Fatalf("SubscribeOwned browser-a: %v", err)
	}
	if err := tr.SubscribeOwned(conn1, sess1, "browser-b", "%1"); err != nil {
		t.Fatalf("SubscribeOwned browser-b: %v", err)
	}
	if got := b.nextID.Load(); got != 2 {
		t.Fatalf("backend subscriptions = %d, want independent seed source for each browser", got)
	}

	tr.UnsubscribeOwned(conn1, sess1, "browser-a")
	if !tr.hasOwnedSubscription(conn1, sess1, "browser-b") {
		t.Fatal("unsubscribing browser A stopped browser B")
	}
}

// TestTerminalRelay_SlowCloseEmitsClosedEvent: when the backend closes the
// channel (process exit), exactly one internalSurfaceClosed is emitted.
func TestTerminalRelay_SlowCloseEmitsClosedEvent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id := int(b.nextID.Load())

	b.CloseID(id)

	got := collectEvents(t, events, 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	sc, ok := got[0].(internalSurfaceClosed)
	if !ok {
		t.Fatalf("expected internalSurfaceClosed, got %T", got[0])
	}
	if sc.ConnID != conn1 || sc.SessionID != sess1 {
		t.Errorf("closed event = {%v, %v}, want {%v, %v}", sc.ConnID, sc.SessionID, conn1, sess1)
	}
	if sc.SubID != id {
		t.Errorf("closed event SubID = %d, want %d", sc.SubID, id)
	}

	// No additional events should arrive.
	extra := collectEvents(t, events, 1, 50*time.Millisecond)
	if len(extra) != 0 {
		t.Errorf("unexpected extra events: %v", extra)
	}
}

func TestTerminalRelay_SlowCloseAllowsResubscribeBeforeClosedEventIsDelivered(t *testing.T) {
	b := newFakeSurfaceBackend()
	events := make(chan internalEvent, 8)
	allowCloseDelivery := make(chan struct{})
	closeStarted := make(chan struct{})
	var closeStartedOnce atomic.Bool

	send := func(ev internalEvent) bool {
		events <- ev
		return true
	}
	sendNow := func(ev internalEvent) {
		if _, ok := ev.(internalSurfaceClosed); ok {
			if closeStartedOnce.CompareAndSwap(false, true) {
				close(closeStarted)
			}
			<-allowCloseDelivery
		}
		events <- ev
	}

	tr := newTestTerminalRelayWithOptions(t, b, send, sendNow)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id := int(b.nextID.Load())

	b.CloseID(id)

	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for blocked internalSurfaceClosed send")
	}

	tr.mu.Lock()
	_, stillPresent := tr.subs[surfaceKey{connID: conn1, sessionID: sess1}]
	tr.mu.Unlock()
	if stillPresent {
		t.Fatal("local subscription remained after slow-close started")
	}

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("re-Subscribe: %v", err)
	}
	id2 := int(b.nextID.Load())
	if id2 == id {
		t.Fatalf("subscriber id did not advance after re-Subscribe: old=%d new=%d", id, id2)
	}

	b.Send(id2, termvt.Event{Kind: termvt.EventOutput, Data: []byte("r")})
	gotOutput := collectEvents(t, events, 1, time.Second)
	if len(gotOutput) != 1 {
		t.Fatalf("expected 1 output event, got %d", len(gotOutput))
	}
	bs, ok := gotOutput[0].(internalBroadcastSurface)
	if !ok {
		t.Fatalf("expected internalBroadcastSurface, got %T", gotOutput[0])
	}
	if bs.Sequence != 0 || string(bs.Data) != "r" {
		t.Fatalf("re-subscribed output = %+v, want sequence 0 data r", bs)
	}

	close(allowCloseDelivery)

	got := collectEvents(t, events, 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	sc, ok := got[0].(internalSurfaceClosed)
	if !ok {
		t.Fatalf("expected internalSurfaceClosed, got %T", got[0])
	}
	if sc.ConnID != conn1 || sc.SessionID != sess1 {
		t.Fatalf("closed event = {%v, %v}, want {%v, %v}", sc.ConnID, sc.SessionID, conn1, sess1)
	}
	if sc.SubID != id {
		t.Fatalf("closed event SubID = %d, want %d", sc.SubID, id)
	}
}

// TestTerminalRelay_Write: Write forwards data to backend.WriteSurface.
func TestTerminalRelay_Write(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	data := []byte("input bytes")
	if err := tr.Write("%1", data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.written) != 1 {
		t.Fatalf("expected 1 WriteSurface call, got %d", len(b.written))
	}
	if b.written[0].frameID != "%1" {
		t.Errorf("frameID = %q, want %%1", b.written[0].frameID)
	}
	if string(b.written[0].data) != string(data) {
		t.Errorf("data = %q, want %q", b.written[0].data, data)
	}
}

// TestTerminalRelay_Resize: Resize forwards dimensions to backend.ResizeSurface.
func TestTerminalRelay_Resize(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Resize("%1", 80, 24); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.resized) != 1 {
		t.Fatalf("expected 1 ResizeSurface call, got %d", len(b.resized))
	}
	rc := b.resized[0]
	if rc.frameID != "%1" || rc.cols != 80 || rc.rows != 24 {
		t.Errorf("resize = {%q, %d, %d}, want {%%1, 80, 24}", rc.frameID, rc.cols, rc.rows)
	}
}

// TestTerminalRelay_UnsubscribeIdempotent: calling Unsubscribe twice must not
// panic or produce errors.
func TestTerminalRelay_UnsubscribeIdempotent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	tr.Unsubscribe(conn1, sess1)
	tr.Unsubscribe(conn1, sess1) // should not panic
}

// TestTerminalRelay_TwoConnsIndependent: two ConnIDs subscribing to the same
// frameID get independent termvt subscriber ids and independent Sequence counters.
func TestTerminalRelay_TwoConnsIndependent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe conn1: %v", err)
	}
	id1 := int(b.nextID.Load())

	if err := tr.Subscribe(conn2, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe conn2: %v", err)
	}
	id2 := int(b.nextID.Load())

	if id1 == id2 {
		t.Fatalf("expected different subscriber ids, both got %d", id1)
	}

	// Send two events to conn1's sub and one to conn2's sub.
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c1-a")})
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c1-b")})
	b.Send(id2, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c2-a")})

	got := collectEvents(t, events, 3, time.Second)
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}

	// Bucket by ConnID.
	seqByConn := map[state.ConnID][]uint64{}
	for _, ev := range got {
		bs, ok := ev.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("expected internalBroadcastSurface, got %T", ev)
		}
		seqByConn[bs.ConnID] = append(seqByConn[bs.ConnID], bs.Sequence)
	}

	if len(seqByConn[conn1]) != 2 {
		t.Errorf("conn1 events = %d, want 2", len(seqByConn[conn1]))
	}
	if len(seqByConn[conn2]) != 1 {
		t.Errorf("conn2 events = %d, want 1", len(seqByConn[conn2]))
	}
	// conn2 must start at Sequence 0, not carry over conn1's counter.
	if seqByConn[conn2][0] != 0 {
		t.Errorf("conn2 first Sequence = %d, want 0", seqByConn[conn2][0])
	}
}

type relayEventRouter struct {
	mu       sync.Mutex
	channels map[surfaceKey]chan internalEvent
}

func newRelayEventRouter() *relayEventRouter {
	return &relayEventRouter{
		channels: make(map[surfaceKey]chan internalEvent),
	}
}

func (r *relayEventRouter) send(ev internalEvent) bool {
	key, ok := relayEventKey(ev)
	if !ok {
		return false
	}

	r.mu.Lock()
	ch := r.channels[key]
	r.mu.Unlock()
	if ch == nil {
		return false
	}

	ch <- ev
	return true
}

func (r *relayEventRouter) setChannel(key surfaceKey, ch chan internalEvent) {
	r.mu.Lock()
	r.channels[key] = ch
	r.mu.Unlock()
}

func relayEventKey(ev internalEvent) (surfaceKey, bool) {
	switch v := ev.(type) {
	case internalBroadcastSurface:
		return surfaceKey{connID: v.ConnID, sessionID: v.SessionID}, true
	case internalSurfaceClosed:
		return surfaceKey{connID: v.ConnID, sessionID: v.SessionID}, true
	default:
		return surfaceKey{}, false
	}
}

func awaitEvent(
	t *testing.T,
	ch <-chan internalEvent,
	timeout time.Duration,
) internalEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timeout waiting for event")
		return nil
	}
}

func TestTerminalRelay_SeversSlowSubscriberWithoutBlockingOthers(t *testing.T) {
	b := newFakeSurfaceBackend()
	router := newRelayEventRouter()
	tr := newTestTerminalRelayWithOptions(
		t,
		b,
		router.send,
		func(ev internalEvent) { _ = router.send(ev) },
		WithTerminalRelaySubscriberBuffer(1),
		WithSeveranceThreshold(1),
	)
	defer tr.Close()

	blockedKey := surfaceKey{connID: conn1, sessionID: sess1}
	fastSameSessionKey := surfaceKey{connID: conn2, sessionID: sess1}
	otherSessionKey := surfaceKey{connID: conn1, sessionID: sess2}

	blockedCh := make(chan internalEvent)
	blockedDrain := make(chan internalEvent, 8)
	fastSameSessionCh := make(chan internalEvent, 8)
	otherSessionCh := make(chan internalEvent, 8)
	router.setChannel(blockedKey, blockedCh)
	router.setChannel(fastSameSessionKey, fastSameSessionCh)
	router.setChannel(otherSessionKey, otherSessionCh)

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe blocked key: %v", err)
	}
	if err := tr.Subscribe(conn2, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe fast same-session key: %v", err)
	}
	if err := tr.Subscribe(conn1, sess2, "%2"); err != nil {
		t.Fatalf("Subscribe other-session key: %v", err)
	}

	for i := 0; i < 3; i++ {
		b.Broadcast("%1", termvt.Event{Kind: termvt.EventOutput, Data: []byte{byte('a' + i)}})
		b.Broadcast("%2", termvt.Event{Kind: termvt.EventOutput, Data: []byte{byte('x' + i)}})

		evSame := awaitEvent(t, fastSameSessionCh, time.Second)
		bsSame, ok := evSame.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("same-session event[%d] type = %T, want internalBroadcastSurface", i, evSame)
		}
		if bsSame.Sequence != uint64(i) {
			t.Fatalf("same-session event[%d] sequence = %d, want %d", i, bsSame.Sequence, i)
		}
		if want := []byte{byte('a' + i)}; string(bsSame.Data) != string(want) {
			t.Fatalf("same-session event[%d] data = %q, want %q", i, bsSame.Data, want)
		}

		evOther := awaitEvent(t, otherSessionCh, time.Second)
		bsOther, ok := evOther.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("other-session event[%d] type = %T, want internalBroadcastSurface", i, evOther)
		}
		if bsOther.Sequence != uint64(i) {
			t.Fatalf("other-session event[%d] sequence = %d, want %d", i, bsOther.Sequence, i)
		}
		if want := []byte{byte('x' + i)}; string(bsOther.Data) != string(want) {
			t.Fatalf("other-session event[%d] data = %q, want %q", i, bsOther.Data, want)
		}
	}

	go func() {
		for ev := range blockedCh {
			blockedDrain <- ev
		}
	}()

	// How many broadcasts squeeze through before severance is scheduling
	// dependent: the fanOut goroutine may or may not have pulled the first
	// event out of the size-1 subscriber channel before the next broadcast
	// finds it full. The contract is only that the slow subscriber is severed
	// after an in-order prefix of the stream, never that it sees a fixed count.
	gotBlocked := collectUntilSurfaceClosed(t, blockedDrain, time.Second)
	if _, ok := gotBlocked[len(gotBlocked)-1].(internalSurfaceClosed); !ok {
		t.Fatalf("last blocked event type = %T, want internalSurfaceClosed", gotBlocked[len(gotBlocked)-1])
	}
	prefix := gotBlocked[:len(gotBlocked)-1]
	if len(prefix) < 1 || len(prefix) > 2 {
		t.Fatalf("blocked key broadcasts before severance = %d, want 1 or 2", len(prefix))
	}
	for i, ev := range prefix {
		bs, ok := ev.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("blocked event[%d] type = %T, want internalBroadcastSurface", i, ev)
		}
		if bs.Sequence != uint64(i) {
			t.Fatalf("blocked event[%d] sequence = %d, want %d", i, bs.Sequence, i)
		}
		if want := []byte{byte('a' + i)}; string(bs.Data) != string(want) {
			t.Fatalf("blocked event[%d] data = %q, want %q", i, bs.Data, want)
		}
	}
	tr.Unsubscribe(conn1, sess1)

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("re-Subscribe blocked key: %v", err)
	}
	b.Broadcast("%1", termvt.Event{Kind: termvt.EventOutput, Data: []byte("r")})

	ev := awaitEvent(t, blockedDrain, time.Second)
	bs, ok := ev.(internalBroadcastSurface)
	if !ok {
		t.Fatalf("re-subscribed event type = %T, want internalBroadcastSurface", ev)
	}
	if bs.Sequence != 0 {
		t.Fatalf("re-subscribed sequence = %d, want 0", bs.Sequence)
	}
	if string(bs.Data) != "r" {
		t.Fatalf("re-subscribed data = %q, want %q", bs.Data, "r")
	}
}
