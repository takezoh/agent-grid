package proto

// RespSurfaceUnsubscribed is a server-initiated push response emitted when the
// daemon severs a surface subscription (backpressure or termvt slow-close).
// It rides the existing surface.unsubscribe command name on the wire envelope
// with an empty req_id so gateways can deliver a browser-observable signal
// without adding a new ServerEvent type.
type RespSurfaceUnsubscribed struct {
	SessionID    string `json:"session_id"`
	SubscriberID string `json:"subscriber_id,omitempty"`
}

func (RespSurfaceUnsubscribed) isResponse() {}