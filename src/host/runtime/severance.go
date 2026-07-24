package runtime

import (
	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

const defaultSeveranceThreshold = 64

// SubscriptionKey identifies one (ConnID, SessionID, SubscriberID) surface
// subscription for backlog attribution across shared IPC hops.
type SubscriptionKey struct {
	ConnID       state.ConnID
	SessionID    state.SessionID
	SubscriberID state.SubscriberID
}

func subscriptionKey(connID state.ConnID, sessionID state.SessionID, subscriberID state.SubscriberID) SubscriptionKey {
	return SubscriptionKey{ConnID: connID, SessionID: sessionID, SubscriberID: subscriberID}
}

// SeveranceGate centralises the backlog threshold used by every hop that
// applies sever-not-drop to surface-output subscriptions (FR-008).
type SeveranceGate struct {
	threshold int
}

func NewSeveranceGate(threshold int) *SeveranceGate {
	if threshold <= 0 {
		threshold = defaultSeveranceThreshold
	}
	return &SeveranceGate{threshold: threshold}
}

func (g *SeveranceGate) Threshold() int { return g.threshold }

func (g *SeveranceGate) OverThreshold(backlog int) bool {
	return backlog >= g.threshold
}

func isInteractiveWireEvent(eventName string) bool {
	switch eventName {
	case proto.EvtNameSurfaceOutput, proto.EvtNamePromptEvent:
		return true
	default:
		return false
	}
}

func isInteractiveInternal(ev internalEvent) bool {
	switch ev.(type) {
	case internalBroadcastSurface, internalSurfaceClosed, internalLifecycleDesired,
		internalLifecycleEffectResult, internalLifecycleDeadline, internalLifecycleExpiry:
		return true
	default:
		return false
	}
}

func isLifecycleInternal(ev internalEvent) bool {
	switch ev.(type) {
	case internalLifecycleDesired, internalLifecycleEffectResult,
		internalLifecycleDeadline, internalLifecycleExpiry, internalLifecycleTelemetry:
		return true
	default:
		return false
	}
}

func (r *Runtime) severSurfaceSubscription(key SubscriptionKey) {
	if r.terminalRelay == nil {
		return
	}
	r.terminalRelay.SeverOwned(key.ConnID, key.SessionID, key.SubscriberID)
}

func subscriptionKeyFromInternal(ev internalEvent) (SubscriptionKey, bool) {
	switch e := ev.(type) {
	case internalBroadcastSurface:
		return subscriptionKey(e.ConnID, e.SessionID, e.SubscriberID), true
	case internalSurfaceClosed:
		return subscriptionKey(e.ConnID, e.SessionID, e.SubscriberID), true
	default:
		return SubscriptionKey{}, false
	}
}
