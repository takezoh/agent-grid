package runtime

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

// broadcastSurfaceOutput fans a chunk of frame surface output to all ConnIDs subscribed
// to e.SessionID via State.SurfaceSubs. Each ConnID gets its own EvtSurfaceOutput
// message so the per-subscriber Sequence is handled by TerminalRelay (the
// internalBroadcastSurface path). This function is used when the effect comes
// from the reducer side (EffBroadcastSurfaceOutput).
func (r *Runtime) broadcastSurfaceOutput(e state.EffBroadcastSurfaceOutput) {
	ev := proto.EvtSurfaceOutput{
		SessionID: string(e.SessionID),
		TimeSec:   e.TimeSec,
		DataB64:   base64.StdEncoding.EncodeToString(e.Data),
		Sequence:  0, // sequence managed by TerminalRelay fan-out goroutines
	}
	// Iterate outer map (ConnID → set of SessionIDs) to find which connections
	// are subscribed to this session.
	for connID, subscriptions := range r.state.SurfaceSubs {
		for subscription := range subscriptions {
			if subscription.SessionID != e.SessionID {
				continue
			}
			ev.SubscriberID = string(subscription.SubscriberID)
			wire, err := proto.EncodeEvent(ev)
			if err != nil {
				slog.Error("runtime: encode surface output failed", "err", err)
				continue
			}
			r.queueWireToConn(connID, wire, proto.EvtNameSurfaceOutput, subscriptionKey(
				connID, subscription.SessionID, subscription.SubscriberID,
			))
		}
	}
}

// broadcastSurfaceFromInternal delivers a single internalBroadcastSurface to
// exactly one ConnID's outbox. Called from dispatchInternal when the TerminalRelay
// fan-out goroutine enqueues a new chunk.
func (r *Runtime) broadcastSurfaceFromInternal(ev internalBroadcastSurface) {
	out := proto.EvtSurfaceOutput{
		SessionID:    string(ev.SessionID),
		SubscriberID: string(ev.SubscriberID),
		TimeSec:      ev.TimeSec,
		DataB64:      base64.StdEncoding.EncodeToString(ev.Data),
		Sequence:     ev.Sequence,
	}
	wire, err := proto.EncodeEvent(out)
	if err != nil {
		slog.Error("runtime: encode surface output (internal) failed", "err", err)
		return
	}
	r.queueWireToConn(ev.ConnID, wire, proto.EvtNameSurfaceOutput, subscriptionKey(
		ev.ConnID, ev.SessionID, ev.SubscriberID,
	))
	if correlation, ok := r.lifecycleCorrelation(ev); ok {
		r.lifecycleTelemetry.Publish(TelemetryRecord{Correlation: correlation,
			Watermark: ev.Sequence, Sequence: ev.Sequence, Digest: surfaceDigest(ev.Data)})
	}
}

func (r *Runtime) emitLifecycleTelemetry(record TelemetryRecord) {
	if record.Correlation.ClientInstanceID == "" {
		return
	}
	output := proto.EvtLifecycleOutput{LifecycleOutput: proto.LifecycleOutput{
		Correlation: record.Correlation,
		Sequence:    record.Sequence,
		Digest:      record.Digest,
	}}
	if lifecycleWire, encodeErr := proto.EncodeEvent(output); encodeErr == nil {
		r.broadcastWire(lifecycleWire, proto.EvtNameLifecycleOutput)
	}
	if record.DropCount > 0 || record.Unknown {
		diagnostic := proto.EvtLifecycleDiagnostic{LifecycleDiagnostic: proto.LifecycleDiagnostic{
			Correlation: record.Correlation,
			Watermark:   record.Watermark,
			DropCount:   record.DropCount,
			Unknown:     record.Unknown,
		}}
		if diagnosticWire, encodeErr := proto.EncodeEvent(diagnostic); encodeErr == nil {
			r.broadcastWire(diagnosticWire, proto.EvtNameLifecycleDiagnostic)
		}
	}
}

func surfaceDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *Runtime) lifecycleCorrelation(ev internalBroadcastSurface) (proto.PublicCorrelation, bool) {
	for _, binding := range r.lifecycleBindings {
		if binding.connID == ev.ConnID && binding.sessionID == ev.SessionID && binding.subscriberID == ev.SubscriberID {
			return binding.correlation, true
		}
	}
	return proto.PublicCorrelation{}, false
}

// broadcastPromptEvent delivers EvtPromptEvent to all ConnIDs subscribed to
// the session that owns e.FrameID. FrameID → SessionID is resolved by scanning
// state.Sessions.
func (r *Runtime) broadcastPromptEvent(e state.EffBroadcastPromptEvent) {
	sessionID := r.sessionIDForFrame(e.FrameID)
	if sessionID == "" {
		slog.Warn("runtime: prompt event: no session for frame", "frame", e.FrameID)
		return
	}
	ev := proto.EvtPromptEvent{
		FrameID:  string(e.FrameID),
		Phase:    e.Phase,
		ExitCode: e.ExitCode,
		NowRFC:   time.Now().UTC().Format(time.RFC3339),
	}
	wire, err := proto.EncodeEvent(ev)
	if err != nil {
		slog.Error("runtime: encode prompt event failed", "err", err)
		return
	}
	for connID, sessions := range r.state.SurfaceSubs {
		for subscription := range sessions {
			if subscription.SessionID == sessionID {
				r.queueWireToConn(connID, wire, proto.EvtNamePromptEvent, subscriptionKey(
					connID, subscription.SessionID, subscription.SubscriberID,
				))
				break
			}
		}
	}
}

// sessionIDForFrame resolves the SessionID that owns frameID by scanning state.Sessions.
// Returns "" if no session contains the frame.
func (r *Runtime) sessionIDForFrame(frameID state.FrameID) state.SessionID {
	for sessID, sess := range r.state.Sessions {
		for _, fr := range sess.Frames {
			if fr.ID == frameID {
				return sessID
			}
		}
	}
	return ""
}

// queueWireToConn enqueues raw wire bytes on a specific ConnID's outbox lane.
// Interactive surface events sever the attributed subscription on overflow.
func (r *Runtime) queueWireToConn(connID state.ConnID, wire []byte, eventName string, sub SubscriptionKey) {
	cc, ok := r.conns[connID]
	if !ok {
		return
	}
	r.queueWireLane(cc, wire, isInteractiveWireEvent(eventName), sub)
}
