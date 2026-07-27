package codexclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Structured log event names for Conn.Run's three silent-drop paths (see
// adr-20260707-codexclient-observability-log). Grepping for these strings in
// server.log is the intended first-response tool for envelope-shaped bugs.
const (
	eventDecodeError = "codexclient.decode_error"
	eventInvalidID   = "codexclient.invalid_id"
	eventPendingMiss = "codexclient.pending_miss"

	// decodeErrorRawTruncateLen bounds the "raw" attribute logged for a
	// decode_error event so a pathological oversized message can't blow up
	// log volume.
	decodeErrorRawTruncateLen = 256

	// nullIDLiteral is the wire representation of an explicit JSON "id":null
	// member, as stored verbatim by RequestID.
	nullIDLiteral = "null"

	// InternalErrorCode is JSON-RPC 2.0's reserved code for server-side
	// errors that the peer generated locally (a timeout, an internal
	// invariant break) rather than forwarded from an upstream. It is the
	// default fill for ReplyError, and for ReplyRPCError when the passed
	// error is not an *RPCError. See
	// docs/note/note-20260707-technical-jsonrpc-id-opacity.md for why
	// bytes-preserving forwarding lives alongside this fallback.
	InternalErrorCode = -32603
)

// RequestID is the JSON-RPC 2.0 "id" member, preserved as opaque wire bytes
// (a JSON string, number, or literal null) rather than decoded into a fixed
// Go numeric type. Peers are free to choose any JSON-RPC 2.0 id
// representation; preserving the raw bytes avoids losing or reformatting an
// id that this Conn did not itself mint.
type RequestID json.RawMessage

// Equal reports whether id and other carry identical wire bytes.
func (id RequestID) Equal(other RequestID) bool {
	return bytes.Equal([]byte(id), []byte(other))
}

// String returns the wire bytes verbatim, for logging/diagnostics.
func (id RequestID) String() string {
	return string(id)
}

// MarshalJSON implements json.Marshaler. It mirrors json.RawMessage: the
// wire bytes are returned verbatim, and a nil/empty id marshals to the JSON
// literal null.
func (id RequestID) MarshalJSON() ([]byte, error) {
	if len(id) == 0 {
		return []byte("null"), nil
	}
	return id, nil
}

// UnmarshalJSON implements json.Unmarshaler, storing the wire bytes verbatim
// (including the literal "null" bytes when the id is explicitly JSON null).
func (id *RequestID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return errors.New("codexclient: RequestID: UnmarshalJSON on nil pointer")
	}
	*id = append((*id)[0:0], data...)
	return nil
}

// rpcMessage is the JSON-RPC 2.0 envelope used by the Codex app-server protocol.
// ID is nil when the "id" member is absent from the wire (a Notification);
// a non-nil ID (including one wrapping the literal "null" bytes) means the
// member was present.
type rpcMessage struct {
	ID     *RequestID      `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// rpcMessageWire mirrors rpcMessage for decoding only. Its ID field is a bare
// json.RawMessage rather than *RequestID: encoding/json's special-case
// handling of JSON null for pointer-typed struct fields would otherwise
// collapse "id" absent and "id":null into the same nil value. A
// RawMessage-typed field keeps the literal "null" bytes instead, letting
// rpcMessage.UnmarshalJSON tell the two cases apart.
type rpcMessageWire struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for rpcMessage so that "id"
// absent and "id":null decode to distinct states (see rpcMessageWire).
func (m *rpcMessage) UnmarshalJSON(data []byte) error {
	var wire rpcMessageWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Method = wire.Method
	m.Params = wire.Params
	m.Result = wire.Result
	m.Error = wire.Error
	if wire.ID == nil {
		m.ID = nil
		return nil
	}
	id := RequestID(wire.ID)
	m.ID = &id
	return nil
}

// RPCError is the typed error returned by Conn.Request when the peer
// replied with a JSON-RPC 2.0 error response (as opposed to a local failure
// like a transport read error or a request-side timeout).
//
// Data holds the peer's error object bytes verbatim — including code /
// message / data as sent — so a proxy can bytes-preserve-forward the
// upstream failure downstream via ReplyRPCError without stringify-and-
// reparse loss. This is the error-object companion to the id-opacity SSOT
// documented in docs/note/note-20260707-technical-jsonrpc-id-opacity.md:
// the shim silently mangled inbound string ids until that fix, and it
// silently synthesized outbound -32603 wraps for every real upstream
// failure until this one.
type RPCError struct {
	Method string          // request method that failed
	Data   json.RawMessage // peer's error object bytes, verbatim
}

// Error implements the error interface. The formatting mirrors what the
// pre-typed-error Conn.Request used to return so log lines and existing
// substring assertions ("codexclient: <method> error: ...") keep working.
func (e *RPCError) Error() string {
	return fmt.Sprintf("codexclient: %s error: %s", e.Method, string(e.Data))
}

// ErrorObject returns the peer's error object bytes, verbatim. Callers
// forwarding upstream errors should hand these bytes to ReplyRPCError (or
// pass the *RPCError itself) rather than re-encoding e.Error() through
// ReplyError, which would collapse the upstream code/data to -32603 and
// a formatted message.
func (e *RPCError) ErrorObject() json.RawMessage { return e.Data }

// Handler receives inbound messages dispatched by Conn.Run.
type Handler interface {
	// OnNotification is called for server-initiated notifications (no reply expected).
	OnNotification(method string, params json.RawMessage)
	// OnServerRequest is called for server-initiated requests.  The handler must
	// call conn.Reply or conn.ReplyError before returning to unblock the peer.
	OnServerRequest(id RequestID, method string, params json.RawMessage)
}

// Conn is a transport-agnostic JSON-RPC framing layer for the Codex app-server
// protocol.  It can be used as either the initiating side (client role) or the
// responding side (server role); both directions use the same framing.
type Conn struct {
	t           Transport
	readTimeout time.Duration
	mu          sync.Mutex
	pending     map[int64]chan rpcMessage
	nextID      int64
}

// NewConn wraps t in a Conn.  readTimeout is applied to each client-initiated
// request; zero means 15 seconds (the historic default).
func NewConn(t Transport, readTimeout time.Duration) *Conn {
	if readTimeout <= 0 {
		readTimeout = 15 * time.Second
	}
	return &Conn{
		t:           t,
		readTimeout: readTimeout,
		pending:     make(map[int64]chan rpcMessage),
	}
}

// Run starts the read loop and dispatches messages to h until the transport
// returns an error.  It blocks until the loop exits.
//
// Three envelope shapes are malformed relative to this Conn's expectations
// (decode failure, an id this Conn cannot resolve against its own pending
// map, and a response with no matching pending request). None of them close
// the transport: each is logged via slog.Default() (see the eventXxx
// constants) and the read loop continues with the next message, per
// adr-20260707-codexclient-observability-log.
func (c *Conn) Run(ctx context.Context, h Handler) error {
	for {
		data, err := c.t.ReadMessage(ctx)
		if err != nil {
			return err
		}
		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			logDecodeError(data, err)
			continue
		}
		// Response to a pending client request.
		if msg.ID != nil && msg.Method == "" {
			c.handleResponse(*msg.ID, msg)
			continue
		}
		if msg.Method == "" {
			continue
		}
		if msg.ID != nil {
			h.OnServerRequest(*msg.ID, msg.Method, msg.Params)
		} else {
			h.OnNotification(msg.Method, msg.Params)
		}
	}
}

// handleResponse routes a response-shaped message (id present, method empty)
// either to the pending map or to a structured pending_miss/invalid_id log
// entry, per FR-005 / FR-006.
func (c *Conn) handleResponse(id RequestID, msg rpcMessage) {
	if string(id) == nullIDLiteral {
		// FR-006: method-empty + id=null is a JSON-RPC 2.0 invalid-Request
		// error response, distinct from a Notification (id member absent
		// entirely). There is no pending request an explicit null id could
		// ever resolve, so this is reported as pending_miss with method=""
		// made explicit, rather than as invalid_id.
		logPendingMiss(id, msg.Method, msg)
		return
	}
	pendingID, err := parsePendingID(id)
	if err != nil {
		logInvalidID(id, msg.Method, err)
		return
	}
	c.resolvePending(pendingID, id, msg)
}

// parsePendingID recovers the int64 key used by the pending-request map from
// a wire RequestID. Ids minted by Request (see below) are always decimal
// integers, optionally JSON-string-quoted; anything else does not match a
// pending request of this Conn's own numbering.
func parsePendingID(id RequestID) (int64, error) {
	return strconv.ParseInt(strings.Trim(string(id), `"`), 10, 64)
}

// Request sends a request and waits for the corresponding response.
func (c *Conn) Request(method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	wireID := RequestID(strconv.AppendInt(nil, id, 10))
	ch := make(chan rpcMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.writeMsg(rpcMessage{ID: &wireID, Method: method, Params: mustJSON(params)}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case msg := <-ch:
		if len(msg.Error) > 0 && string(msg.Error) != "null" {
			// Preserve the peer's error object bytes verbatim so a proxy
			// can bytes-forward via ReplyRPCError. errors.As(err,
			// **RPCError) recovers the object; err.Error() keeps its
			// historic "codexclient: <method> error: <json>" shape.
			return msg.Result, &RPCError{
				Method: method,
				Data:   append(json.RawMessage(nil), msg.Error...),
			}
		}
		return msg.Result, nil
	case <-time.After(c.readTimeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("codexclient: timeout waiting for %s", method)
	}
}

// Notify sends a notification (no response expected).
func (c *Conn) Notify(method string, params any) error {
	return c.writeMsg(rpcMessage{Method: method, Params: mustJSON(params)})
}

// Reply sends a success response to a server-initiated request.
func (c *Conn) Reply(id RequestID, result any) error {
	return c.writeMsg(rpcMessage{ID: &id, Result: mustJSON(result)})
}

// ReplyError sends a JSON-RPC 2.0 error response to a server-initiated
// request. The wire error object is always spec-compliant (v1 schema
// JSONRPCErrorError requires "code" and "message"): code defaults to
// InternalErrorCode (-32603) for locally-synthesized errors. Proxy sites
// that want to bytes-forward an upstream JSON-RPC error object should use
// ReplyRPCError, which preserves the upstream code / data verbatim.
func (c *Conn) ReplyError(id RequestID, errMsg string) error {
	return c.writeMsg(rpcMessage{ID: &id, Error: internalErrorObject(errMsg)})
}

// ReplyRPCError forwards an error to a server-initiated request. If err is
// (or wraps) an *RPCError — the case for any error returned by
// Conn.Request when the peer replied with a JSON-RPC error — the peer's
// error object bytes are echoed verbatim, preserving code / message / data
// end-to-end. Otherwise (a local timeout, an I/O error, a shim-synthesized
// error) it falls back to the ReplyError shape (code=-32603 + message).
//
// This is the error-object counterpart to bytes-preserving id forwarding
// (see docs/note/note-20260707-technical-jsonrpc-id-opacity.md): together
// they let a proxy relay peer↔peer JSON-RPC without losing either the
// caller-chosen id shape or the responder's structured failure detail.
func (c *Conn) ReplyRPCError(id RequestID, err error) error {
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && len(rpcErr.Data) > 0 {
		return c.writeMsg(rpcMessage{ID: &id, Error: append(json.RawMessage(nil), rpcErr.Data...)})
	}
	return c.writeMsg(rpcMessage{ID: &id, Error: internalErrorObject(err.Error())})
}

// internalErrorObject builds a spec-compliant JSONRPCErrorError body with
// InternalErrorCode. Shared by ReplyError and ReplyRPCError's fallback
// path so the two never drift.
func internalErrorObject(message string) json.RawMessage {
	return mustJSON(map[string]any{
		"code":    InternalErrorCode,
		"message": message,
	})
}

// Close tears down the underlying transport.
func (c *Conn) Close() error { return c.t.Close() }

func (c *Conn) resolvePending(id int64, rawID RequestID, msg rpcMessage) {
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch == nil {
		logPendingMiss(rawID, msg.Method, msg)
		return
	}
	ch <- msg
}

// logDecodeError logs FR-005's decode_error event: json.Unmarshal could not
// parse the message at all, so only the raw bytes (truncated) and the
// decode error are available.
func logDecodeError(raw []byte, err error) {
	if len(raw) > decodeErrorRawTruncateLen {
		raw = raw[:decodeErrorRawTruncateLen]
	}
	slog.Default().Warn(eventDecodeError,
		"raw", string(raw),
		"err", err.Error(),
	)
}

// logInvalidID logs FR-005's invalid_id event: the message decoded, but its
// id is not one this Conn's own numbering could ever have minted (an
// object/array id, or a numeric id that overflows int64).
func logInvalidID(id RequestID, method string, err error) {
	slog.Default().Warn(eventInvalidID,
		"raw_id", id.String(),
		"method", method,
		"err", err.Error(),
	)
}

// logPendingMiss logs the pending_miss event: the id parsed fine (or, per
// FR-006, was an explicit null) but no self-issued Request is waiting on it
// — a timeout, a duplicate reply, or a peer bug.
func logPendingMiss(id RequestID, method string, msg rpcMessage) {
	slog.Default().Warn(eventPendingMiss,
		"raw_id", id.String(),
		"method", method,
		"result_len", len(msg.Result),
		"error_len", len(msg.Error),
	)
}

func (c *Conn) writeMsg(msg rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.t.WriteMessage(context.Background(), data)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
