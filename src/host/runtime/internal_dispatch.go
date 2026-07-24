package runtime

import (
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/takezoh/agent-grid/host/proto"
	rsubsystem "github.com/takezoh/agent-grid/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/pathmap"
)

// internalEvent is the closed sum of runtime-internal lifecycle
// events that bypass state.Reduce. Used for connection accept /
// teardown, where the runtime owns mutable state (the conns map)
// the reducer can't see.
type internalEvent interface {
	isInternalEvent()
}

const lifecycleMailboxSize = 16

// connOpen is enqueued by the accept loop after Accept returns.
type connOpen struct {
	conn net.Conn
}

func (connOpen) isInternalEvent() {}

// connClose is enqueued by connReader after the socket EOFs.
type connClose struct {
	id state.ConnID
}

func (connClose) isInternalEvent() {}

type internalLifecycleDesired struct {
	connID state.ConnID
	reqID  string
	cmd    proto.CmdLifecycleDesired
}

func (internalLifecycleDesired) isInternalEvent() {}

type internalLifecycleEffectResult struct {
	connID state.ConnID
	cmd    proto.CmdLifecycleDesired
	err    error
}

func (internalLifecycleEffectResult) isInternalEvent() {}

type internalLifecycleDeadline struct {
	connID      state.ConnID
	correlation proto.PublicCorrelation
}

func (internalLifecycleDeadline) isInternalEvent() {}

type internalLifecycleExpiry struct {
	connID      state.ConnID
	correlation proto.PublicCorrelation
}

func (internalLifecycleExpiry) isInternalEvent() {}

type internalLifecycleTelemetry struct{ record TelemetryRecord }

func (internalLifecycleTelemetry) isInternalEvent() {}

// internalSetRelay is enqueued by SetRelay to wire a FileRelay into the loop.
type internalSetRelay struct {
	relay *FileRelay
}

func (internalSetRelay) isInternalEvent() {}

// internalBroadcastWire is enqueued by FileRelay so broadcastWire runs
// on the event loop, never on the sweep goroutine (which would race
// the loop's conns / Subscribers maps).
type internalBroadcastWire struct {
	wire      []byte
	eventName string
}

func (internalBroadcastWire) isInternalEvent() {}

// internalStartRestoredTaps is enqueued by StartTapsForRestoredFrames to
// attach frame taps to frames that were restored from the snapshot (warm
// or cold start) — bypasses the reducer because Reduce is only invoked
// by user-driven events, and restored frames never go through
// EvFrameSpawned.
type internalStartRestoredTaps struct{}

func (internalStartRestoredTaps) isInternalEvent() {}

type internalBarrier struct {
	drained chan bool
}

func (internalBarrier) isInternalEvent() {}

// internalSpawnComplete is enqueued by the spawn goroutine after a window has
// been launched. The goroutine performs all slow I/O and carries the resulting
// per-frame handles back as data; the event loop is the sole writer that stores
// them into loop-owned maps (handleSpawnComplete), keeping spawn off the
// single-writer state without any direct map writes from the goroutine.
type internalSpawnComplete struct {
	effect           state.EffSpawnFrame
	subsystemID      state.SubsystemID
	sub              rsubsystem.Subsystem
	cleanup          func() error
	token            string         // empty when the frame exposes no frame-messaging authority
	mounts           pathmap.Mounts // nil for non-container frames
	containerSockDir string         // raw ContainerSockDir from WrappedLaunch; empty for host frames
	bindResult       rsubsystem.BindResult
}

func (internalSpawnComplete) isInternalEvent() {}

// drainInteractiveInternal empties the interactive internal lane before the
// loop services bulk traffic, preventing surface-output bursts from starving
// unrelated control-plane work (FR-006/FR-007).
func (r *Runtime) drainInteractiveInternal() {
	for {
		select {
		case iev := <-r.internalChInteractive:
			r.dispatchInternal(iev)
		default:
			return
		}
	}
}

func (r *Runtime) drainLifecycleInternal() {
	for {
		select {
		case ev := <-r.lifecycleCh:
			r.dispatchInternal(ev)
		default:
			return
		}
	}
}

// dispatchInternal handles runtime-internal events.
func (r *Runtime) dispatchInternal(ev internalEvent) {
	switch e := ev.(type) {
	case connOpen:
		r.handleConnOpen(e.conn)
	case connClose:
		r.handleConnClose(e.id)
	case internalLifecycleDesired:
		r.handleLifecycleDesired(e)
	case internalLifecycleEffectResult:
		r.handleLifecycleEffectResult(e)
	case internalLifecycleDeadline:
		r.handleLifecycleDeadline(e)
	case internalLifecycleExpiry:
		r.handleLifecycleExpiry(e)
	case internalLifecycleTelemetry:
		r.emitLifecycleTelemetry(e.record)
	case internalBroadcastWire:
		r.broadcastWire(e.wire, e.eventName)
	case internalSetRelay:
		r.relay = e.relay
		r.syncRelayWatches()
	case internalStartRestoredTaps:
		_ = e
		r.startRestoredTaps()
	case internalBarrier:
		e.drained <- r.quiesced()
	case internalSpawnComplete:
		r.handleSpawnComplete(e)
	case internalFrameListRequest:
		e.reply <- r.frameMessagingList(e.source)
	case internalFrameReadRequest:
		e.reply <- r.frameMessagingRead(e.source, e.peerFrameID)
	case internalFrameSendRequest:
		e.reply <- r.frameMessagingSend(e.source, e.targetFrameID, e.topic, e.body, e.priority)
	case internalFrameReplyRequest:
		e.reply <- r.frameMessagingReply(e.source, e.messageID, e.body, e.finalAnswer, e.resolution, e.confidence)
	case internalFrameListByThreadRequest:
		e.reply <- r.frameMessagingListByThread(e.sessionID, e.threadID)
	case internalFrameReadByThreadRequest:
		e.reply <- r.frameMessagingReadByThread(e.sessionID, e.threadID, e.peerFrameID)
	case internalFrameSendByThreadRequest:
		e.reply <- r.frameMessagingSendByThread(e.sessionID, e.threadID, e.targetFrameID, e.topic, e.body, e.priority)
	case internalFrameReplyByThreadRequest:
		e.reply <- r.frameMessagingReplyByThread(e.sessionID, e.threadID, e.messageID, e.body, e.finalAnswer, e.resolution, e.confidence)
	case internalBroadcastSurface:
		if r.terminalRelay == nil || !r.terminalRelay.isCurrentOwnedSubscription(
			e.ConnID, e.SessionID, e.SubscriberID, e.FrameID, e.SubID,
		) {
			return
		}
		r.broadcastSurfaceFromInternal(e)
	case internalSurfaceClosed:
		if r.terminalRelay != nil && !r.terminalRelay.shouldApplySlowClose(e.ConnID, e.SessionID, e.FrameID, e.SubID, e.SubscriberID) {
			return
		}
		r.dispatch(state.EvCmdSurfaceUnsubscribe{ConnID: e.ConnID, ReqID: "", SessionID: e.SessionID, SubscriberID: e.SubscriberID})
	}
}

// startRestoredTaps attaches a frame tap to each restored root frame.
// Non-root frames don't get taps because their driver state isn't
// displayed in the UI. Called from the event loop so r.taps is
// guaranteed to be initialised and state.Sessions is accessed under
// the loop's single-writer discipline.
func (r *Runtime) startRestoredTaps() {
	if r.taps == nil {
		return
	}
	for _, sess := range r.state.Sessions {
		if len(sess.Frames) == 0 {
			continue
		}
		frameID := sess.Frames[0].ID
		r.taps.start(frameID, string(frameID), r)
	}
}

// enqueueInternal posts an internal event onto the runtime's
// internal channel. Non-blocking; drops silently on a full channel.
// Returns true when the event was accepted by the channel, false on drop.
//
// Drops are logged at Debug (not Warn) because the daemon's own log file is
// streamed back to clients via FileRelay. Every Warn would be written to
// server.log, the FileRelay watcher would observe the write, the sweep would
// enqueue an internalBroadcastWire, and if internalCh is already saturated
// the enqueue would drop and emit ANOTHER Warn — a self-sustaining feedback
// loop that wedges the daemon (observed: 39MB of identical lines at 10/s).
// Debug-level drop messages stay out of the default log stream and break the
// cycle. Callers that genuinely cannot tolerate a drop (e.g. spawn completion)
// use sendSpawnComplete, which blocks rather than dropping. Callers that can
// recover from a drop by retrying (e.g. FileRelay.sweep) check the return
// value and roll back state to retry on the next tick.
//
// Every drop is also counted per-event-type via internalDrops so saturation
// causes can be attributed via InternalDropStats.
func (r *Runtime) enqueueInternal(ev internalEvent) bool {
	if isLifecycleInternal(ev) {
		select {
		case r.lifecycleCh <- ev:
			return true
		default:
			if r.internalDrops != nil {
				r.internalDrops.inc(internalEventName(ev))
			}
			return false
		}
	}
	ch := r.internalChBulk
	if isInteractiveInternal(ev) {
		ch = r.internalChInteractive
	}
	select {
	case ch <- ev:
		return true
	default:
		if key, ok := subscriptionKeyFromInternal(ev); ok {
			r.severSurfaceSubscription(key)
			return false
		}
		name := internalEventName(ev)
		if r.internalDrops != nil {
			r.internalDrops.inc(name)
		}
		slog.Debug("runtime: internal channel full, dropping", "type", name)
		return false
	}
}

// Internal event name constants. Shared by internalEventName (for log/metrics
// labels) and newInternalDropCounter (for the pre-populated counter map).
// Keeping them as const makes the catalogue greppable.
const (
	internalEventConnOpen           = "conn-open"
	internalEventConnClose          = "conn-close"
	internalEventLifecycleDesired   = "lifecycle-desired"
	internalEventLifecycleResult    = "lifecycle-result"
	internalEventLifecycleDeadline  = "lifecycle-deadline"
	internalEventLifecycleExpiry    = "lifecycle-expiry"
	internalEventLifecycleTelemetry = "lifecycle-telemetry"
	internalEventBroadcastWire      = "broadcast-wire"
	internalEventBroadcastSurface   = "broadcast-surface"
	internalEventSurfaceClosed      = "surface-closed"
	internalEventSetRelay           = "set-relay"
	internalEventStartRestoredTaps  = "start-restored-taps"
	internalEventSpawnComplete      = "spawn-complete"
	internalEventFrameList          = "frame-list"
	internalEventFrameRead          = "frame-read"
	internalEventFrameSend          = "frame-send"
	internalEventFrameReply         = "frame-reply"
	internalEventFrameListByThread  = "frame-list-by-thread"
	internalEventFrameReadByThread  = "frame-read-by-thread"
	internalEventFrameSendByThread  = "frame-send-by-thread"
	internalEventFrameReplyByThread = "frame-reply-by-thread"
	internalEventUnknown            = "unknown"
)

// internalEventName returns a short identifier for an internal event, used
// for diagnostic logs (Debug level) and drop counter labels.
func internalEventName(ev internalEvent) string {
	switch ev.(type) {
	case connOpen:
		return internalEventConnOpen
	case connClose:
		return internalEventConnClose
	case internalLifecycleDesired:
		return internalEventLifecycleDesired
	case internalLifecycleEffectResult:
		return internalEventLifecycleResult
	case internalLifecycleDeadline:
		return internalEventLifecycleDeadline
	case internalLifecycleExpiry:
		return internalEventLifecycleExpiry
	case internalLifecycleTelemetry:
		return internalEventLifecycleTelemetry
	case internalBroadcastWire:
		return internalEventBroadcastWire
	case internalBroadcastSurface:
		return internalEventBroadcastSurface
	case internalSurfaceClosed:
		return internalEventSurfaceClosed
	case internalSetRelay:
		return internalEventSetRelay
	case internalStartRestoredTaps:
		return internalEventStartRestoredTaps
	case internalSpawnComplete:
		return internalEventSpawnComplete
	case internalFrameListRequest:
		return internalEventFrameList
	case internalFrameReadRequest:
		return internalEventFrameRead
	case internalFrameSendRequest:
		return internalEventFrameSend
	case internalFrameReplyRequest:
		return internalEventFrameReply
	case internalFrameListByThreadRequest:
		return internalEventFrameListByThread
	case internalFrameReadByThreadRequest:
		return internalEventFrameReadByThread
	case internalFrameSendByThreadRequest:
		return internalEventFrameSendByThread
	case internalFrameReplyByThreadRequest:
		return internalEventFrameReplyByThread
	default:
		return internalEventUnknown
	}
}

// internalDropCounter tracks per-event-type silent drops from enqueueInternal.
// Pre-populated at construction so the hot path uses an existing *atomic.Uint64
// (no map writes, no locks). Unknown event types fall back to the "unknown"
// bucket.
type internalDropCounter struct {
	byType map[string]*atomic.Uint64
}

func newInternalDropCounter() *internalDropCounter {
	names := []string{
		internalEventConnOpen,
		internalEventConnClose,
		internalEventLifecycleDesired,
		internalEventLifecycleResult,
		internalEventLifecycleDeadline,
		internalEventLifecycleExpiry,
		internalEventLifecycleTelemetry,
		internalEventBroadcastWire,
		internalEventBroadcastSurface,
		internalEventSurfaceClosed,
		internalEventSetRelay,
		internalEventStartRestoredTaps,
		internalEventSpawnComplete,
		internalEventFrameList,
		internalEventFrameRead,
		internalEventFrameSend,
		internalEventFrameReply,
		internalEventFrameListByThread,
		internalEventFrameReadByThread,
		internalEventFrameSendByThread,
		internalEventFrameReplyByThread,
		internalEventUnknown,
	}
	m := make(map[string]*atomic.Uint64, len(names))
	for _, n := range names {
		m[n] = new(atomic.Uint64)
	}
	return &internalDropCounter{byType: m}
}

func (c *internalDropCounter) inc(name string) {
	if v, ok := c.byType[name]; ok {
		v.Add(1)
		return
	}
	c.byType[internalEventUnknown].Add(1)
}

// snapshot returns the current per-event-type drop counts. Only non-zero
// buckets are included so the caller can spot active producers quickly.
func (c *internalDropCounter) snapshot() map[string]uint64 {
	out := make(map[string]uint64, len(c.byType))
	for k, v := range c.byType {
		if n := v.Load(); n > 0 {
			out[k] = n
		}
	}
	return out
}

// sendSpawnComplete delivers a spawn-completion event to the loop. Unlike
// enqueueInternal it must NOT drop: handleSpawnComplete is the sole writer of
// the subsystem/cleanup maps and container registry for the frame, so losing
// this event would leak the already-launched subsystem, frame, container
// token and cleanup closure with no recovery path. Blocks until the loop
// accepts it or the daemon shuts down (r.done), so it never leaks a goroutine.
func (r *Runtime) sendSpawnComplete(ev internalEvent) {
	select {
	case r.internalChBulk <- ev:
	case <-r.done:
	}
}

func (r *Runtime) sendInternalNow(ev internalEvent) {
	if isLifecycleInternal(ev) {
		select {
		case r.lifecycleCh <- ev:
		case <-r.done:
		}
		return
	}
	ch := r.internalChBulk
	if isInteractiveInternal(ev) {
		ch = r.internalChInteractive
	}
	select {
	case ch <- ev:
	case <-r.done:
	}
}
