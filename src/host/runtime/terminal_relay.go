package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/termvt"
)

// SurfaceBackend is the subset of PtyBackend that TerminalRelay depends on.
// It is extracted as an interface so tests can inject a fake implementation
// without starting a real pty.
type SurfaceBackend interface {
	SubscribeSurface(frameID string, cols, rows int) (int, <-chan termvt.Event, error)
	UnsubscribeSurface(frameID string, id int) error
	WriteSurface(frameID string, data []byte) error
	ResizeSurface(frameID string, cols, rows int) error
}

type leasedSurfaceBackend interface {
	AcquireSurface(context.Context, string, int, int) (SurfaceLease, error)
}

type identifiableSurfaceLease interface {
	ID() int
}

// bufferedSurfaceBackend optionally accepts a caller-specified subscriber
// buffer size. Production backends may ignore this seam and fall back to their
// default contract; tests use it to drive overflow deterministically.
type bufferedSurfaceBackend interface {
	SubscribeSurfaceWithBuffer(frameID string, cols, rows, buffer int) (int, <-chan termvt.Event, error)
}

// surfaceKey is the map key for one logical browser subscription. SubscriberID
// distinguishes lifecycle WebSockets sharing the same daemon ConnID.
type surfaceKey struct {
	connID       state.ConnID
	sessionID    state.SessionID
	subscriberID state.SubscriberID
}

// surfaceSub holds the live state of one fan-out goroutine.
type surfaceSub struct {
	frameID string
	cols    int
	rows    int
	subID   int           // termvt subscriber id returned by SubscribeSurface
	cancel  chan struct{} // closed to stop the fan-out goroutine early
	seq     uint64        // next Sequence value to emit (subscribe-scoped, resets on re-subscribe)
	relay   chan internalBroadcastSurface
	release func() error
}

// TerminalRelay manages per-(ConnID, SessionID, SubscriberID) subscriptions to termvt
// sessions and fans EventOutput chunks out as internalBroadcastSurface events
// on the runtime event loop. It is a reducer-bypass goroutine in the same
// spirit as FileRelay.
//
// Public API (4 methods):
//   - Subscribe / Unsubscribe manage the fan-out goroutine lifecycle.
//   - Write / Resize forward raw input and resize requests to the backend.
//   - Close tears everything down.
type TerminalRelay struct {
	backend SurfaceBackend
	// send posts an internal event onto the runtime event loop. TerminalRelay
	// holds only this bound function (not *Runtime) so its fan-out goroutines
	// cannot touch loop-owned state directly. Returns true on accept, false on
	// drop — a false return triggers per-subscription severance.
	send    func(internalEvent) bool
	sendNow func(internalEvent)
	startTS time.Time // base for TimeSec computation

	mu   sync.Mutex
	subs map[surfaceKey]*surfaceSub

	subscriberBuffer   int
	severanceThreshold int
}

const defaultTerminalRelaySubscriberBuffer = 256

type TerminalRelayOption func(*TerminalRelay)

// WithTerminalRelaySubscriberBuffer overrides the backend subscriber buffer
// TerminalRelay requests when the backend supports it. Non-positive values are
// ignored so the production default remains unchanged.
func WithTerminalRelaySubscriberBuffer(size int) TerminalRelayOption {
	return func(tr *TerminalRelay) {
		if size > 0 {
			tr.subscriberBuffer = size
		}
	}
}

// WithSeveranceThreshold overrides the per-subscription relay backlog limit
// before a subscription is severed. Non-positive values are ignored.
func WithSeveranceThreshold(size int) TerminalRelayOption {
	return func(tr *TerminalRelay) {
		if size > 0 {
			tr.severanceThreshold = size
		}
	}
}

// NewTerminalRelay creates a TerminalRelay that forwards surface events via send.
// send is typically rt.enqueueInternal, bound at construction time.
func NewTerminalRelay(
	b SurfaceBackend,
	send func(internalEvent) bool,
	sendNow func(internalEvent),
	opts ...TerminalRelayOption,
) *TerminalRelay {
	tr := &TerminalRelay{
		backend:            b,
		send:               send,
		sendNow:            sendNow,
		startTS:            time.Now(),
		subs:               make(map[surfaceKey]*surfaceSub),
		subscriberBuffer:   defaultTerminalRelaySubscriberBuffer,
		severanceThreshold: defaultSeveranceThreshold,
	}
	for _, opt := range opts {
		opt(tr)
	}
	return tr
}

// Subscribe starts a fan-out goroutine for (connID, sessionID) on frameID.
// If a subscription already exists for that key it is a no-op (idempotent).
func (tr *TerminalRelay) Subscribe(connID state.ConnID, sessionID state.SessionID, frameID string, cols, rows int) error {
	return tr.SubscribeOwned(connID, sessionID, "", frameID, cols, rows)
}

func (tr *TerminalRelay) SubscribeOwned(
	connID state.ConnID,
	sessionID state.SessionID,
	subscriberID state.SubscriberID,
	frameID string,
	cols, rows int,
) error {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	if _, exists := tr.subs[key]; exists {
		tr.mu.Unlock()
		return nil
	}
	tr.mu.Unlock()

	id, ch, release, err := tr.subscribeSurface(frameID, cols, rows)
	if err != nil {
		return err
	}

	sub := &surfaceSub{
		frameID: frameID,
		cols:    cols,
		rows:    rows,
		subID:   id,
		cancel:  make(chan struct{}),
		relay:   make(chan internalBroadcastSurface, tr.severanceThreshold),
		release: release,
	}

	tr.mu.Lock()
	// Double-check after acquiring lock (another goroutine could have raced us).
	if _, exists := tr.subs[key]; exists {
		tr.mu.Unlock()
		_ = release()
		return nil
	}
	tr.subs[key] = sub
	tr.mu.Unlock()

	go tr.relayForward(key, sub)
	go tr.fanOut(key, sub, ch)
	return nil
}

// RebindOwned makes one logical browser subscription follow frameID. The
// session-level subscription key is stable while a session's head frame can
// change, so retaining a subscription to a different frame would split the
// terminal: output would come from the old frame while input is routed to the
// new head. Detach the old source before opening the new one so no output is
// accepted from the wrong frame while the replacement surface is starting.
func (tr *TerminalRelay) RebindOwned(
	connID state.ConnID,
	sessionID state.SessionID,
	subscriberID state.SubscriberID,
	frameID string,
	cols, rows int,
) error {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	old, exists := tr.subs[key]
	if exists && old.frameID == frameID && old.cols == cols && old.rows == rows {
		tr.mu.Unlock()
		return nil
	}
	if exists {
		delete(tr.subs, key)
	}
	tr.mu.Unlock()

	if exists {
		close(old.cancel)
		_ = old.release()
	}
	return tr.SubscribeOwned(connID, sessionID, subscriberID, frameID, cols, rows)
}

func (tr *TerminalRelay) subscribeSurface(frameID string, cols, rows int) (int, <-chan termvt.Event, func() error, error) {
	if backend, ok := tr.backend.(leasedSurfaceBackend); ok {
		lease, err := backend.AcquireSurface(context.Background(), frameID, cols, rows)
		if err != nil {
			return 0, nil, nil, err
		}
		id := 0
		if identifiable, ok := lease.(identifiableSurfaceLease); ok {
			id = identifiable.ID()
		}
		return id, lease.Events(), lease.Release, nil
	}
	if backend, ok := tr.backend.(bufferedSurfaceBackend); ok {
		id, events, err := backend.SubscribeSurfaceWithBuffer(frameID, cols, rows, tr.subscriberBuffer)
		return id, events, func() error { return tr.backend.UnsubscribeSurface(frameID, id) }, err
	}
	id, events, err := tr.backend.SubscribeSurface(frameID, cols, rows)
	return id, events, func() error { return tr.backend.UnsubscribeSurface(frameID, id) }, err
}

// Unsubscribe stops the fan-out goroutine for (connID, sessionID) and
// releases the termvt subscriber. Idempotent — safe to call multiple times.
func (tr *TerminalRelay) Unsubscribe(connID state.ConnID, sessionID state.SessionID) {
	tr.UnsubscribeOwned(connID, sessionID, "")
}

func (tr *TerminalRelay) UnsubscribeOwned(connID state.ConnID, sessionID state.SessionID, subscriberID state.SubscriberID) {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	sub, ok := tr.subs[key]
	if !ok {
		tr.mu.Unlock()
		return
	}
	delete(tr.subs, key)
	tr.mu.Unlock()

	close(sub.cancel)
	_ = sub.release()
}

func (tr *TerminalRelay) shouldApplySlowClose(
	connID state.ConnID,
	sessionID state.SessionID,
	frameID string,
	subID int,
	subscriberIDs ...state.SubscriberID,
) bool {
	var subscriberID state.SubscriberID
	if len(subscriberIDs) > 0 {
		subscriberID = subscriberIDs[0]
	}
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	sub, ok := tr.subs[key]
	if !ok {
		return true
	}
	return sub.frameID == frameID && sub.subID == subID
}

func (tr *TerminalRelay) isCurrentOwnedSubscription(
	connID state.ConnID,
	sessionID state.SessionID,
	subscriberID state.SubscriberID,
	frameID string,
	subID int,
) bool {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	sub, ok := tr.subs[key]
	return ok && sub.frameID == frameID && sub.subID == subID
}

func (tr *TerminalRelay) hasSubscription(connID state.ConnID, sessionID state.SessionID) bool {
	return tr.hasOwnedSubscription(connID, sessionID, "")
}

func (tr *TerminalRelay) isActiveSub(key surfaceKey, sub *surfaceSub) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	cur, ok := tr.subs[key]
	return ok && cur == sub
}

func (tr *TerminalRelay) hasOwnedSubscription(connID state.ConnID, sessionID state.SessionID, subscriberID state.SubscriberID) bool {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	_, ok := tr.subs[key]
	return ok
}

// SeverOwned stops one subscription due to shared-hop backpressure. Idempotent
// for unknown keys. Other hops call this instead of reimplementing severance.
func (tr *TerminalRelay) SeverOwned(connID state.ConnID, sessionID state.SessionID, subscriberID state.SubscriberID) {
	key := surfaceKey{connID: connID, sessionID: sessionID, subscriberID: subscriberID}

	tr.mu.Lock()
	sub, ok := tr.subs[key]
	if !ok {
		tr.mu.Unlock()
		return
	}
	delete(tr.subs, key)
	tr.mu.Unlock()

	close(sub.cancel)
	_ = sub.release()
	tr.sendNow(internalSurfaceClosed{
		ConnID:       key.connID,
		SessionID:    key.sessionID,
		SubscriberID: key.subscriberID,
		FrameID:      sub.frameID,
		SubID:        sub.subID,
	})
}

// Write forwards raw bytes to the frame's pty. No carriage return is appended;
// the caller (xterm.js via the web gateway) is responsible for proper encoding.
func (tr *TerminalRelay) Write(frameID string, data []byte) error {
	return tr.backend.WriteSurface(frameID, data)
}

// Resize forwards a terminal resize to the frame's pty and VT emulator.
func (tr *TerminalRelay) Resize(frameID string, cols, rows int) error {
	return tr.backend.ResizeSurface(frameID, cols, rows)
}

// Close unsubscribes all active subscriptions and shuts down TerminalRelay.
func (tr *TerminalRelay) Close() {
	tr.mu.Lock()
	keys := make([]surfaceKey, 0, len(tr.subs))
	for k := range tr.subs {
		keys = append(keys, k)
	}
	tr.mu.Unlock()

	for _, key := range keys {
		tr.UnsubscribeOwned(key.connID, key.sessionID, key.subscriberID)
	}
}

// relayForward drains a subscription's dedicated relay buffer onto the shared
// internal channel. A drop on the shared hop severs only this subscription.
func (tr *TerminalRelay) relayForward(key surfaceKey, sub *surfaceSub) {
	backlog := 0
	for {
		select {
		case <-sub.cancel:
			return
		case ev := <-sub.relay:
			if !tr.isActiveSub(key, sub) {
				return
			}
			if tr.send(ev) {
				if backlog > 0 {
					backlog--
				}
				continue
			}
			backlog++
			if backlog >= tr.severanceThreshold {
				tr.SeverOwned(key.connID, key.sessionID, key.subscriberID)
				return
			}
		}
	}
}

// fanOut runs in a dedicated goroutine per subscription. It receives termvt
// events from ch, copies EventOutput payloads into internalBroadcastSurface,
// and enqueues them on the per-subscription relay buffer. When the channel is
// closed (slow-close by termvt on process exit) it emits one internalSurfaceClosed
// and exits. When cancel is closed (Unsubscribe / Close) it exits immediately.
func (tr *TerminalRelay) fanOut(key surfaceKey, sub *surfaceSub, ch <-chan termvt.Event) {
	for {
		select {
		case <-sub.cancel:
			return
		case ev, ok := <-ch:
			if !ok {
				tr.mu.Lock()
				if cur, ok := tr.subs[key]; ok && cur == sub {
					delete(tr.subs, key)
				}
				tr.mu.Unlock()
				tr.sendNow(internalSurfaceClosed{
					ConnID:       key.connID,
					SessionID:    key.sessionID,
					SubscriberID: key.subscriberID,
					FrameID:      sub.frameID,
					SubID:        sub.subID,
				})
				return
			}
			if ev.Kind != termvt.EventOutput {
				continue
			}
			data := make([]byte, len(ev.Data))
			copy(data, ev.Data)

			if !tr.isActiveSub(key, sub) {
				return
			}

			seq := sub.seq
			sub.seq++

			select {
			case sub.relay <- internalBroadcastSurface{
				ConnID:       key.connID,
				SessionID:    key.sessionID,
				SubscriberID: key.subscriberID,
				FrameID:      sub.frameID,
				SubID:        sub.subID,
				Data:         data,
				TimeSec:      time.Since(tr.startTS).Seconds(),
				Sequence:     seq,
			}:
			default:
				tr.SeverOwned(key.connID, key.sessionID, key.subscriberID)
				return
			}
		}
	}
}

// === Internal event types ===

// internalBroadcastSurface is enqueued by TerminalRelay when it receives an
// EventOutput chunk. The event loop routes it to the single ConnID that
// subscribed so that EvtSurfaceOutput is streamed over the wire.
type internalBroadcastSurface struct {
	ConnID       state.ConnID
	SessionID    state.SessionID
	SubscriberID state.SubscriberID
	FrameID      string
	SubID        int
	Data         []byte
	TimeSec      float64
	Sequence     uint64
}

func (internalBroadcastSurface) isInternalEvent() {}

// internalSurfaceClosed is enqueued by TerminalRelay when termvt closes the
// subscriber channel (slow-close on process exit). The event loop uses it to
// remove the entry from state.SurfaceSubs so the client knows the stream ended.
type internalSurfaceClosed struct {
	ConnID       state.ConnID
	SessionID    state.SessionID
	SubscriberID state.SubscriberID
	FrameID      string
	SubID        int
}

func (internalSurfaceClosed) isInternalEvent() {}
