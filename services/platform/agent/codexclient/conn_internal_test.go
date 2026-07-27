package codexclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"testing"
	"time"
)

// TestReplyError_WireShape pins the JSON-RPC 2.0 spec-compliance invariant on
// self-generated (non-forwarded) error responses: ReplyError MUST include a
// numeric "code" alongside "message" so the reply matches JSONRPCErrorError
// (v1 schema `required: ["code", "message"]`). Without "code", codex-cli
// 0.142.5 rejects the whole envelope as "data did not match any variant of
// untagged enum JSONRPCMessage", which is what took down the frame-messaging
// shim's thread/start path.
func TestReplyError_WireShape(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	conn := NewConn(StdioTransport(pr1, pw2), time.Second)
	peer := StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go conn.Run(ctx, noopInternalHandler{}) //nolint:errcheck

	id := RequestID(`"call-1"`)
	writeErr := make(chan error, 1)
	go func() { writeErr <- conn.ReplyError(id, "not supported") }()

	data, err := peer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("ReplyError: %v", werr)
	}
	var got struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code    *int64  `json:"code"`
			Message *string `json:"message"`
			Data    any     `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal reply: %v (raw=%s)", err, data)
	}
	if string(got.ID) != string(id) {
		t.Fatalf("id = %s, want %s", got.ID, id)
	}
	if got.Error.Code == nil {
		t.Fatalf("error.code missing; wire=%s (v1 JSONRPCErrorError requires 'code')", data)
	}
	if got.Error.Message == nil {
		t.Fatalf("error.message missing; wire=%s", data)
	}
	if *got.Error.Message != "not supported" {
		t.Fatalf("error.message = %q, want %q", *got.Error.Message, "not supported")
	}
}

// TestRequest_ReturnsRPCError pins the typed-error contract: when the peer
// returns a JSON-RPC error response, Request must return an error that
// unwraps to *RPCError, and ErrorObject() must expose the peer's error
// object bytes verbatim so proxy sites can bytes-preserve-forward without
// stringify-and-reparse loss.
func TestRequest_ReturnsRPCError(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	connSelf := NewConn(StdioTransport(pr1, pw2), time.Second)
	peer := StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go connSelf.Run(ctx, noopInternalHandler{}) //nolint:errcheck

	type reqResult struct {
		res json.RawMessage
		err error
	}
	resultCh := make(chan reqResult, 1)
	go func() {
		res, err := connSelf.Request("bad", map[string]any{})
		resultCh <- reqResult{res: res, err: err}
	}()

	data, err := peer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var msg rpcMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal request: %v", err)
	}
	if msg.ID == nil {
		t.Fatal("request has no id")
	}
	// The peer's error object carries a non-default code and a data field so
	// bytes-preservation is observable (a reconstructed error object would
	// drop or rename these).
	upstreamErr := `{"code":-32602,"message":"invalid params","data":{"field":"cwd"}}`
	reply := `{"id":` + msg.ID.String() + `,"error":` + upstreamErr + `}`
	if err := peer.WriteMessage(ctx, []byte(reply)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err == nil {
			t.Fatal("Request returned nil error, want *RPCError")
		}
		var rpcErr *RPCError
		if !errors.As(got.err, &rpcErr) {
			t.Fatalf("Request error = %T (%v), want *RPCError", got.err, got.err)
		}
		if rpcErr.Method != "bad" {
			t.Fatalf("RPCError.Method = %q, want %q", rpcErr.Method, "bad")
		}
		if string(rpcErr.ErrorObject()) != upstreamErr {
			t.Fatalf("RPCError.ErrorObject() = %s, want %s (bytes-preserving)", rpcErr.ErrorObject(), upstreamErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Request to resolve")
	}
}

// TestReplyRPCError_ForwardsUpstreamErrorBytes pins the bytes-preserving
// error-forwarding invariant that mirrors the id-opacity SSOT
// (note-20260707-technical-jsonrpc-id-opacity): a proxy that received an
// *RPCError from Request() must be able to forward the exact upstream error
// object bytes back downstream — code / message / data preserved verbatim.
// Without this, downstream peers (codex-cli 0.142.5) that inspect the
// numeric code lose the upstream's real failure reason to a synthetic
// -32603 wrap.
func TestReplyRPCError_ForwardsUpstreamErrorBytes(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	conn := NewConn(StdioTransport(pr1, pw2), time.Second)
	peer := StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go conn.Run(ctx, noopInternalHandler{}) //nolint:errcheck

	upstreamErrBytes := json.RawMessage(`{"code":-32000,"message":"upstream-said-so","data":{"detail":"specific"}}`)
	rpcErr := &RPCError{Method: "thread/start", Data: upstreamErrBytes}

	id := RequestID(`"call-2"`)
	writeErr := make(chan error, 1)
	go func() { writeErr <- conn.ReplyRPCError(id, rpcErr) }()

	data, err := peer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("ReplyRPCError: %v", werr)
	}
	var got struct {
		ID    json.RawMessage `json:"id"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal reply: %v (raw=%s)", err, data)
	}
	if string(got.ID) != string(id) {
		t.Fatalf("id = %s, want %s", got.ID, id)
	}
	if string(got.Error) != string(upstreamErrBytes) {
		t.Fatalf("error object mismatch (bytes-preserving forwarding failed):\n got: %s\nwant: %s", got.Error, upstreamErrBytes)
	}
}

// TestReplyRPCError_LocalErrorGetsInternalCode pins the fallback contract:
// when ReplyRPCError receives a non-*RPCError (a local timeout, an I/O
// error, anything the proxy synthesized itself), it must still emit a
// spec-compliant JSONRPCErrorError with code=-32603 (Internal error) plus
// the error's message text — the same shape a direct ReplyError call would
// produce. This lets shim proxy sites use ReplyRPCError uniformly for
// every err returned by upstream.Request without a type-switch dance.
func TestReplyRPCError_LocalErrorGetsInternalCode(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	conn := NewConn(StdioTransport(pr1, pw2), time.Second)
	peer := StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go conn.Run(ctx, noopInternalHandler{}) //nolint:errcheck

	id := RequestID(`"call-3"`)
	writeErr := make(chan error, 1)
	go func() { writeErr <- conn.ReplyRPCError(id, errors.New("timeout waiting for thread/start")) }()

	data, err := peer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("ReplyRPCError: %v", werr)
	}
	var got struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code    *int64  `json:"code"`
			Message *string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal reply: %v (raw=%s)", err, data)
	}
	if got.Error.Code == nil || *got.Error.Code != -32603 {
		t.Fatalf("error.code = %v, want -32603 (Internal error); wire=%s", got.Error.Code, data)
	}
	if got.Error.Message == nil || *got.Error.Message != "timeout waiting for thread/start" {
		t.Fatalf("error.message = %v, want %q; wire=%s", got.Error.Message, "timeout waiting for thread/start", data)
	}
}

// TestRpcMessage_IDRoundTrip pins AC-002: the JSON-RPC 2.0 "id" member
// round-trips as opaque wire bytes for all four shapes a peer may send, and
// an explicit JSON literal null is distinguished from the member being
// absent (a Notification).
func TestRpcMessage_IDRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		wire    string
		wantNil bool // true: rpcMessage.ID must be nil (id absent)
		wantID  string
	}{
		{name: "string id", wire: `{"id":"abc","method":"m"}`, wantID: `"abc"`},
		{name: "number id", wire: `{"id":42,"method":"m"}`, wantID: `42`},
		{name: "explicit null id", wire: `{"id":null,"method":"m"}`, wantID: `null`},
		{name: "absent id (notification)", wire: `{"method":"m"}`, wantNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var msg rpcMessage
			if err := json.Unmarshal([]byte(tc.wire), &msg); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if tc.wantNil {
				if msg.ID != nil {
					t.Fatalf("ID = %q, want nil (absent)", msg.ID.String())
				}
				return
			}
			if msg.ID == nil {
				t.Fatalf("ID = nil, want %q", tc.wantID)
			}
			if got := msg.ID.String(); got != tc.wantID {
				t.Fatalf("ID = %q, want %q", got, tc.wantID)
			}

			// Re-marshal and confirm the "id" wire bytes are preserved
			// verbatim through a full round trip.
			out, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var back rpcMessage
			if err := json.Unmarshal(out, &back); err != nil {
				t.Fatalf("re-Unmarshal: %v", err)
			}
			if back.ID == nil || !back.ID.Equal(*msg.ID) {
				t.Fatalf("round-trip ID mismatch: got %v, want %v", back.ID, msg.ID)
			}
		})
	}
}

type noopInternalHandler struct{}

func (noopInternalHandler) OnNotification(_ string, _ json.RawMessage)               {}
func (noopInternalHandler) OnServerRequest(_ RequestID, _ string, _ json.RawMessage) {}

// TestConn_RequestResolvesViaInt64PendingMap pins AC-003: Request mints a
// self-issued decimal-string wire id, but the pending-response map it
// resolves through remains keyed by int64 (the SSOT) rather than by the
// wire bytes; resolution normalizes a (possibly quoted) decimal wire id back
// to that int64 key via parsePendingID.
func TestConn_RequestResolvesViaInt64PendingMap(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	connSelf := NewConn(StdioTransport(pr1, pw2), time.Second)
	peer := StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go connSelf.Run(ctx, noopInternalHandler{}) //nolint:errcheck

	type reqResult struct {
		res json.RawMessage
		err error
	}
	resultCh := make(chan reqResult, 1)
	go func() {
		res, err := connSelf.Request("ping", map[string]any{"x": 1})
		resultCh <- reqResult{res: res, err: err}
	}()

	// Read the outgoing request straight off the wire: the id must be a
	// decimal string, per Conn.Request's self-issued numbering.
	data, err := peer.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var msg rpcMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal request: %v", err)
	}
	if msg.ID == nil {
		t.Fatal("request has no id")
	}
	n, err := strconv.ParseInt(msg.ID.String(), 10, 64)
	if err != nil {
		t.Fatalf("wire id %q is not a decimal int64: %v", msg.ID.String(), err)
	}

	// The pending map is keyed by that int64 (the SSOT), not by the wire
	// bytes themselves.
	connSelf.mu.Lock()
	_, ok := connSelf.pending[n]
	connSelf.mu.Unlock()
	if !ok {
		t.Fatalf("pending map missing int64 key %d derived from wire id %q", n, msg.ID.String())
	}

	// Reply using a JSON-string-quoted form of the same decimal id, proving
	// resolution normalizes via parsePendingID (strconv.ParseInt after
	// trimming surrounding quotes) rather than requiring exact byte match.
	quotedID := RequestID(`"` + msg.ID.String() + `"`)
	reply := rpcMessage{ID: &quotedID, Result: mustJSON(map[string]any{"ok": true})}
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("Marshal reply: %v", err)
	}
	if err := peer.WriteMessage(ctx, replyBytes); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("Request error: %v", got.err)
		}
		var payload map[string]any
		if err := json.Unmarshal(got.res, &payload); err != nil {
			t.Fatalf("Unmarshal result: %v", err)
		}
		if payload["ok"] != true {
			t.Fatalf("result = %v, want ok=true", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Request to resolve")
	}

	connSelf.mu.Lock()
	_, stillPending := connSelf.pending[n]
	connSelf.mu.Unlock()
	if stillPending {
		t.Fatalf("pending map still holds key %d after resolution", n)
	}
}
