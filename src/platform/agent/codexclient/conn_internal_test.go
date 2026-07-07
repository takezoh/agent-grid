package codexclient

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"
)

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
