package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

// errDaemonGone is returned by writeOutbound when the daemon events channel
// closes, indicating the daemon disconnected.
var errDaemonGone = errors.New("server/api: daemon disconnected")

var lifecycleSubscriberCounter atomic.Uint64

type runtimeTerminalBarrier struct{ forwarded uint64 }

func (b *runtimeTerminalBarrier) mark(sequence uint64) {
	if sequence > b.forwarded {
		b.forwarded = sequence
	}
}

func (b *runtimeTerminalBarrier) ready(finalSequence uint64) bool {
	return b.forwarded >= finalSequence
}

func lifecycleCorrelationKey(c proto.PublicCorrelation) string {
	return fmt.Sprintf("%s/%d/%d", c.ClientInstanceID, c.ConnectionGeneration, c.ClientRevision)
}

// Attacher is the daemon-side surface AttachWS needs (proto.Client wrapper).
// Implemented by DaemonAdapter; a fake is used in gateway_terminal_test.go.
type Attacher interface {
	// SubscribeSurface starts streaming output for sessionID on this WS.
	// Returns immediately after the daemon ack and yields a chan of
	// proto.ServerEvent filtered to this session (best-effort: events
	// for other sessions may also come through if the daemon-side filter
	// is coarse — the gateway re-filters).
	SubscribeSurface(ctx context.Context, sessionID string, cols, rows uint16) (<-chan proto.ServerEvent, error)
	UnsubscribeSurface(ctx context.Context, sessionID string) error
	// SendSurfaceSubscribe forwards a CmdSurfaceSubscribe to the daemon
	// WITHOUT registering a new event subscriber on the DaemonClient.
	// Used by AttachLifecycleWS, which already holds a single event channel
	// shared by every multiplexed session.
	SendSurfaceSubscribe(ctx context.Context, sessionID, subscriberID string, cols, rows uint16) error
	SendSurfaceUnsubscribe(ctx context.Context, sessionID, subscriberID string) error
	WriteRaw(ctx context.Context, sessionID string, data []byte) error
	Resize(ctx context.Context, sessionID string, cols, rows uint16) error
	// SubscribeLifecycle subscribes to daemon-side lifecycle events
	// (sessions-changed) and returns a channel of ServerEvent.
	// The returned channel is closed on disconnect.
	SubscribeLifecycle(ctx context.Context) (<-chan proto.ServerEvent, error)
	// PushChannelFor returns pre-encoded WS control frames paired with the
	// events channel returned by SubscribeLifecycle (server-initiated severance).
	PushChannelFor(eventsCh <-chan proto.ServerEvent) <-chan []byte
}

// LifecycleDesiredSender is the v2 control-plane seam. It is optional on
// legacy test attachers so the browser and new server can roll as one unit
// without reintroducing imperative gateway reconciliation.
type LifecycleDesiredSender interface {
	SendLifecycleDesired(context.Context, proto.CmdLifecycleDesired, string) error
}

// DaemonAdapter implements Attacher on top of DaemonClient.
type DaemonAdapter struct {
	d *DaemonClient
}

// NewDaemonAdapter wraps a DaemonClient as an Attacher.
func NewDaemonAdapter(d *DaemonClient) *DaemonAdapter { return &DaemonAdapter{d: d} }

// SubscribeSurface sends CmdSurfaceSubscribe and returns a per-call event
// channel. The channel is auto-unregistered from the DaemonClient fan-out
// when ctx is cancelled (browser disconnect / WS handler return).
//
// Register BEFORE send: any event the daemon emits as a direct consequence
// of CmdSurfaceSubscribe — notably the snapshot output that termvt sends
// to a brand-new subscriber — must arrive at a registered fan-out channel,
// not into the empty subscriber map. The subscribe-then-send order would
// drop the initial screen state silently.
func (a *DaemonAdapter) SubscribeSurface(ctx context.Context, sid string, cols, rows uint16) (<-chan proto.ServerEvent, error) {
	ch := a.d.SubscribeEvents(ctx)
	if _, err := a.d.SendCommand(ctx, proto.CmdSurfaceSubscribe{SessionID: sid, Cols: cols, Rows: rows}); err != nil {
		return nil, err
	}
	return ch, nil
}

// UnsubscribeSurface sends CmdSurfaceUnsubscribe to the daemon.
func (a *DaemonAdapter) UnsubscribeSurface(ctx context.Context, sid string) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceUnsubscribe{SessionID: sid})
	return err
}

// SendSurfaceSubscribe forwards CmdSurfaceSubscribe to the daemon without
// allocating a fresh DaemonClient subscriber. Used by AttachLifecycleWS,
// which multiplexes subscribe requests over its single lifecycle event channel.
func (a *DaemonAdapter) SendSurfaceSubscribe(ctx context.Context, sid, subscriberID string, cols, rows uint16) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceSubscribe{SessionID: sid, SubscriberID: subscriberID, Cols: cols, Rows: rows})
	return err
}

func (a *DaemonAdapter) SendSurfaceUnsubscribe(ctx context.Context, sid, subscriberID string) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceUnsubscribe{SessionID: sid, SubscriberID: subscriberID})
	return err
}

func (a *DaemonAdapter) SendLifecycleDesired(ctx context.Context, cmd proto.CmdLifecycleDesired, subscriberID string) error {
	cmd.SubscriberID = subscriberID
	_, err := a.d.SendCommand(ctx, cmd)
	return err
}

// WriteRaw sends CmdSurfaceWriteRaw to the daemon.
func (a *DaemonAdapter) WriteRaw(ctx context.Context, sid string, data []byte) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceWriteRaw{SessionID: sid, Data: data})
	return err
}

// Resize sends CmdSurfaceResize to the daemon.
func (a *DaemonAdapter) Resize(ctx context.Context, sid string, cols, rows uint16) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceResize{SessionID: sid, Cols: cols, Rows: rows})
	return err
}

// SubscribeLifecycle sends CmdSubscribe for sessions-changed, session-file-line,
// agent-notification, and surface-output events (the lifecycle WS multiplexes
// surface output for any session the browser subscribed to via {k:"s"}). The
// returned per-call event channel is auto-unregistered when ctx is cancelled.
//
// Register BEFORE send: the very first sessions-changed event arrives as a
// direct daemon-side reaction to CmdSubscribe (see runtime.proto_bridge).
// Registering after the send loses that hello-seeding payload to an empty
// subscriber map and leaves the browser without a "h" frame forever.
func (a *DaemonAdapter) SubscribeLifecycle(ctx context.Context) (<-chan proto.ServerEvent, error) {
	ch := a.d.SubscribeEvents(ctx)
	filters := []string{
		proto.EvtNameSessionsChanged,
		proto.EvtNameSessionFileLine,
		proto.EvtNameAgentNotification,
		proto.EvtNameActivityEvents,
		proto.EvtNameSurfaceOutput,
		proto.EvtNameApprovalRequested,
		proto.EvtNameApprovalResolved,
		proto.EvtNameQuestionRequested,
		proto.EvtNameQuestionResolved,
		proto.EvtNameLifecycleOutcome,
		proto.EvtNameLifecycleOutput,
		proto.EvtNameLifecycleDiagnostic,
	}
	if _, err := a.d.SendCommand(ctx, proto.CmdSubscribe{Filters: filters}); err != nil {
		return nil, err
	}
	return ch, nil
}

func (a *DaemonAdapter) PushChannelFor(eventsCh <-chan proto.ServerEvent) <-chan []byte {
	return a.d.PushChannelFor(eventsCh)
}

// writeTypedClose sends a WebSocket StatusGoingAway typed close frame.
func writeTypedClose(c *websocket.Conn, reason string) {
	_ = c.Close(websocket.StatusGoingAway, reason)
}

// AttachWS bridges one WebSocket connection to a session surface. It streams
// output events to the client (writeOutbound) and forwards client input/resize
// (readInbound goroutine). Returns when the connection or daemon closes.
func AttachWS(ctx context.Context, sess Attacher, sessionID string, c *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cols, rows, err := readInitialGeometry(ctx, c)
	if err != nil {
		writeTypedClose(c, "invalid-geometry")
		return err
	}
	ch, err := sess.SubscribeSurface(ctx, sessionID, cols, rows)
	if err != nil {
		var eb *proto.ErrorBody
		if errors.As(err, &eb) {
			_ = c.Write(ctx, websocket.MessageText, controlFrame(0, string(eb.Code)))
		}
		writeTypedClose(c, "subscribe-failed")
		return err
	}
	defer func() { _ = sess.UnsubscribeSurface(context.Background(), sessionID) }()

	go func() { readInbound(ctx, sess, sessionID, c); cancel() }()
	return writeOutbound(ctx, sessionID, c, ch)
}

func readInitialGeometry(ctx context.Context, c *websocket.Conn) (uint16, uint16, error) {
	_, data, err := c.Read(ctx)
	if err != nil {
		return 0, 0, err
	}
	var msg inbound
	if err := json.Unmarshal(data, &msg); err != nil || msg.K != "r" {
		return 0, 0, errors.New("server/api: first terminal frame must declare geometry")
	}
	if msg.Cols <= 0 || msg.Rows <= 0 || state.SizeHintRejectReason(msg.Cols, msg.Rows) != "" {
		return 0, 0, errors.New("server/api: invalid initial terminal geometry")
	}
	return uint16(msg.Cols), uint16(msg.Rows), nil
}

// helloFrame is the first server→browser frame for a lifecycle WebSocket.
// It seeds the browser with the current sessions / features so the React
// store can render the initial view before any subsequent view-update
// arrives. The web client owns its own active-session-per-tab, so no active
// session id is shipped.
type helloFrame struct {
	K                string              `json:"k"` // always "h"
	Sessions         []proto.SessionInfo `json:"sessions"`
	Features         []string            `json:"features"`
	ServerTime       int64               `json:"serverTime"`
	ClientInstanceID string              `json:"clientInstanceId,omitempty"`
	// ProtocolVersion is the bundled capability axis skeleton (FR-P1-03).
	// Same-build clients match this constant and skip per-capability negotiation.
	ProtocolVersion string   `json:"protocolVersion,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

// ProtocolVersion is the Phase 0/1 wire contract version advertised on hello.
const ProtocolVersion = "1.0.0-phase01"

// BundledCapabilities is the capability set co-shipped with this daemon build.
var BundledCapabilities = []string{
	"approval.respond",
	"question.respond",
	"sessions.view_update",
	"surface.subscribe",
}

// encodeHelloFrame encodes EvtSessionsChanged as the initial hello frame.
// nil slices are replaced with empty slices so the browser always gets arrays.
func encodeHelloFrame(sc proto.EvtSessionsChanged, serverTime int64, clientInstanceID string) []byte {
	sessions := sc.Sessions
	if sessions == nil {
		sessions = []proto.SessionInfo{}
	}
	features := sc.Features
	if features == nil {
		features = []string{}
	}
	caps := append([]string(nil), BundledCapabilities...)
	h := helloFrame{
		K:                "h",
		Sessions:         sessions,
		Features:         features,
		ServerTime:       serverTime,
		ClientInstanceID: clientInstanceID,
		ProtocolVersion:  ProtocolVersion,
		Capabilities:     caps,
	}
	b, err := json.Marshal(h)
	if err != nil {
		slog.Error("server/api: encode hello failed", "err", err)
		return nil
	}
	return b
}

// AttachLifecycleWS bridges one WebSocket connection to daemon lifecycle
// events. Used when the client connects without a ?session= query param.
// The single WebSocket is multiplexed: it carries lifecycle frames
// (k:"h" hello, k:"v" view-update, k:"tt" / k:"et" tail, k:"n" notification)
// and per-session surface frames (asciicast output array) for any session the
// browser has subscribed to via inbound k:"s" frames. Inbound k:"u" frames
// unsubscribe. k:"i"/"r" frames forward input and resize for a specific
// sessionId. Sends a 2-step close (ADR 0011) on daemon disconnect.
//
// The read goroutine is required not only to dispatch inbound frames but to
// drain the coder/websocket control frames (pings) — without it, the browser
// keep-alive times out and forcibly closes the connection.
func AttachLifecycleWS(ctx context.Context, sess Attacher, c *websocket.Conn, clientInstanceID string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := sess.SubscribeLifecycle(ctx)
	if err != nil {
		writeTypedClose(c, "lifecycle-subscribe-failed")
		return err
	}
	pushCh := sess.PushChannelFor(ch)
	subs := newLifecycleSubSet()
	writeMu := &sync.Mutex{}
	subscriberID := "web-" + strconv.FormatUint(lifecycleSubscriberCounter.Add(1), 10)
	ownerID := "owner-" + randomOpaque24()
	defer func() {
		// Cleanup: unsubscribe daemon-side for every session this WS held open.
		for _, id := range subs.drain() {
			_ = sess.SendSurfaceUnsubscribe(context.Background(), id, subscriberID)
		}
	}()

	go func() {
		readLifecycleInbound(ctx, sess, c, subs, subscriberID, ownerID, clientInstanceID, writeMu)
		cancel()
	}()

	helloSent := false
	barriers := make(map[string]*runtimeTerminalBarrier)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-pushCh:
			if !ok {
				_ = writeLifecycleFrameLocked(ctx, c, writeMu, controlFrame(0, "daemon-disconnected"))
				writeTypedClose(c, "daemon-disconnected")
				return errDaemonGone
			}
			if err := writeLifecycleFrameLocked(ctx, c, writeMu, frame); err != nil {
				return err
			}
		case ev, ok := <-ch:
			if !ok {
				_ = writeLifecycleFrameLocked(ctx, c, writeMu, controlFrame(0, "daemon-disconnected"))
				writeTypedClose(c, "daemon-disconnected")
				return errDaemonGone
			}
			frames, nextHello, ok := lifecycleFrames(ev, helloSent, clientInstanceID, subscriberID, ownerID, subs, barriers)
			helloSent = nextHello
			if !ok {
				continue
			}
			for _, frame := range frames {
				if frame != nil {
					if err := writeLifecycleFrameLocked(ctx, c, writeMu, frame); err != nil {
						return err
					}
				}
			}
		}
	}
}

func lifecycleFrames(ev proto.ServerEvent, helloSent bool, clientInstanceID, subscriberID, ownerID string, subs *lifecycleSubSet, barriers map[string]*runtimeTerminalBarrier) ([][]byte, bool, bool) {
	switch e := ev.(type) {
	case proto.EvtSessionsChanged:
		if helloSent {
			return [][]byte{encodeServerEvent(e)}, true, true
		}
		return [][]byte{encodeHelloFrame(e, time.Now().Unix(), clientInstanceID)}, true, true
	case proto.EvtSessionFileLine, proto.EvtAgentNotification, proto.EvtActivityEvents,
		proto.EvtApprovalRequested, proto.EvtApprovalResolved,
		proto.EvtQuestionRequested, proto.EvtQuestionResolved,
		proto.EvtLifecycleDiagnostic:
		return [][]byte{encodeServerEvent(e)}, helloSent, true
	case proto.EvtLifecycleOutput:
		barrier := lifecycleBarrier(barriers, e.Correlation)
		barrier.mark(e.Sequence)
		return [][]byte{encodeServerEvent(e)}, helloSent, true
	case proto.EvtLifecycleOutcome:
		barrier := lifecycleBarrier(barriers, e.Correlation)
		frames := make([][]byte, 0, 2)
		if !barrier.ready(e.FinalSeq) {
			frames = append(frames, encodeServerEvent(proto.EvtLifecycleDiagnostic{LifecycleDiagnostic: proto.LifecycleDiagnostic{
				Correlation: e.Correlation, Watermark: barrier.forwarded, Unknown: true,
			}}))
		}
		return append(frames, encodeServerEvent(e)), helloSent, true
	case proto.EvtSurfaceOutput:
		if (e.SubscriberID != subscriberID && e.SubscriberID != ownerID) || !subs.contains(e.SessionID) {
			return nil, helloSent, false
		}
		return [][]byte{encodeServerEvent(e)}, helloSent, true
	default:
		return nil, helloSent, false
	}
}

func lifecycleBarrier(barriers map[string]*runtimeTerminalBarrier, c proto.PublicCorrelation) *runtimeTerminalBarrier {
	key := lifecycleCorrelationKey(c)
	barrier := barriers[key]
	if barrier == nil {
		barrier = &runtimeTerminalBarrier{}
		barriers[key] = barrier
	}
	return barrier
}

// unwrapProtoError extracts (code, message) from a proto.ErrorBody, falling
// back to ("internal", err.Error()) for opaque errors.
func unwrapProtoError(err error) (string, string) {
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		return string(eb.Code), eb.Message
	}
	return "internal", err.Error()
}

// writeRespOKFrame writes {k:"r", reqId:...} (best-effort: errors logged).
func writeRespOKFrame(ctx context.Context, c *websocket.Conn, reqID string) {
	if reqID == "" {
		return
	}
	frame, err := json.Marshal(respOKFrame{K: "r", ReqID: reqID})
	if err != nil {
		slog.Error("server/api: encode resp-ok frame", "err", err)
		return
	}
	if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
		slog.Warn("server/api: write resp-ok frame", "err", err)
	}
}

// writeRespErrFrame writes {k:"e", reqId, code, message} (best-effort).
func writeRespErrFrame(ctx context.Context, c *websocket.Conn, reqID, code, message string) {
	if reqID == "" {
		return
	}
	frame, err := json.Marshal(respErrFrame{
		K: "e", ReqID: reqID, Code: code, Message: message,
	})
	if err != nil {
		slog.Error("server/api: encode resp-err frame", "err", err)
		return
	}
	if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
		slog.Warn("server/api: write resp-err frame", "err", err)
	}
}

// respOKFrame is the success response to a subscribe / unsubscribe command.
type respOKFrame struct {
	K     string `json:"k"` // always "r"
	ReqID string `json:"reqId"`
}

// respErrFrame is the error response to a subscribe / unsubscribe command.
type respErrFrame struct {
	K       string `json:"k"` // always "e"
	ReqID   string `json:"reqId"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeOutbound reads proto.ServerEvent values from ch and encodes them as WS
// frames. On daemon disconnect (ch closed) it sends the 2-step close defined
// in ADR 0011: control frame then StatusGoingAway typed close.
func writeOutbound(ctx context.Context, sessionID string, c *websocket.Conn, ch <-chan proto.ServerEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				// Daemon disconnected: 2-step close (ADR 0011).
				_ = c.Write(ctx, websocket.MessageText, controlFrame(0, "daemon-disconnected"))
				writeTypedClose(c, "daemon-disconnected")
				return errDaemonGone
			}
			// Filter: only forward surface-scoped events belonging to this
			// session. Any other event type (notably EvtSessionsChanged, which
			// is part of the lifecycle stream) must be dropped here so that
			// terminal-only WS clients do not receive lifecycle traffic.
			switch e := ev.(type) {
			case proto.EvtSurfaceOutput:
				if e.SessionID != sessionID {
					continue
				}
			case proto.EvtSessionFileLine:
				if e.SessionID != sessionID {
					continue
				}
			case proto.EvtAgentNotification:
				if e.SessionID != sessionID {
					continue
				}
			default:
				// Not a surface-scoped event (e.g. EvtSessionsChanged). The
				// AttachWS path is the per-session terminal stream; everything
				// else is owned by AttachLifecycleWS.
				continue
			}
			frame := encodeServerEvent(ev)
			if frame == nil {
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return err
			}
		}
	}
}

// readInbound forwards client messages (input, resize) to the session until
// the connection or context closes. Errors are logged at warn level and cause
// the function to return; the caller goroutine then invokes cancel().
func readInbound(ctx context.Context, sess Attacher, sessionID string, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		applyInboundProto(ctx, sess, sessionID, data)
	}
}

// applyInboundProto decodes a raw browser frame and dispatches to sess.
// "i" → WriteRaw; "r" (positive cols+rows) → Resize. Unknown kinds are
// silently dropped.
func applyInboundProto(ctx context.Context, sess Attacher, sessionID string, data []byte) {
	var in inbound
	if json.Unmarshal(data, &in) != nil {
		return
	}
	switch in.K {
	case "i":
		if err := sess.WriteRaw(ctx, sessionID, []byte(in.D)); err != nil {
			slog.Warn("server/api: write raw to session", "err", err)
		}
	case "r":
		tryResize(ctx, sess, sessionID, in.Cols, in.Rows, "resize session")
	}
}

// tryResize validates browser resize hints before uint16 narrowing. Non-positive
// dimensions are silently dropped (existing WS behavior). Out-of-range values
// emit a structured warn log and are dropped without calling Resize.
func tryResize(ctx context.Context, sess Attacher, sessionID string, cols, rows int, op string) {
	if cols <= 0 || rows <= 0 {
		return
	}
	if reason := state.SizeHintRejectReason(cols, rows); reason != "" {
		slog.Warn("server/api: resize dropped",
			"session_id", sessionID,
			"cols", cols,
			"rows", rows,
			"reason", reason,
		)
		return
	}
	if err := sess.Resize(ctx, sessionID, uint16(cols), uint16(rows)); err != nil {
		slog.Warn("server/api: "+op, "err", err, "session_id", sessionID)
	}
}
