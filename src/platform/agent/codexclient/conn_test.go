package codexclient_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
)

// pipeTransport wires two StdioTransports back-to-back for in-process tests.
func pipeTransport() (codexclient.Transport, codexclient.Transport) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return codexclient.StdioTransport(pr1, pw2), codexclient.StdioTransport(pr2, pw1)
}

// discardWriteTransport lets writes succeed immediately and blocks reads forever.
func discardWriteTransport() codexclient.Transport {
	pr, _ := io.Pipe() // nobody writes to this end → reads block
	return codexclient.StdioTransport(pr, io.Discard)
}

func TestConn_RequestResponse(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// connB: echo params as result.
	go connB.Run(ctx, &echoHandler{conn: connB}) //nolint:errcheck
	// connA: needs a read loop to receive the response.
	go connA.Run(ctx, &noopHandler{}) //nolint:errcheck

	result, err := connA.Request("ping", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["x"] != float64(1) {
		t.Fatalf("got %v, want x=1", got)
	}
}

func TestConn_Notify(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	recv := make(chan string, 1)
	h := &notifyHandler{recv: recv}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, h) //nolint:errcheck

	if err := connA.Notify("hello", map[string]any{"msg": "world"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	select {
	case got := <-recv:
		if !strings.Contains(got, "world") {
			t.Fatalf("got %q, want world", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestConn_RequestTimeout(t *testing.T) {
	// peer accepts writes but never replies
	tr := discardWriteTransport()
	conn := codexclient.NewConn(tr, 100*time.Millisecond)
	go conn.Run(context.Background(), &noopHandler{}) //nolint:errcheck
	_, err := conn.Request("slow", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %q, want timeout", err)
	}
}

func TestConn_ReplyError(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go connB.Run(ctx, &replyErrorHandler{conn: connB, msg: "not supported"}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})                                        //nolint:errcheck

	_, err := connA.Request("bad", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("got %q, want 'not supported'", err)
	}
}

// TestConnRunLog pins AC-006: Conn.Run's three silent-drop paths (decode
// failure, an id this Conn cannot parse as its own pending numbering — both
// an object id and an int64-overflowing numeric id, per AC-006's three
// invalid-envelope cases — and a method-empty response with no matching
// pending request, including the FR-006 id=null variant) each emit a
// structured log entry via slog.Default() and, critically, none of them
// close the transport: a message following the invalid envelopes is still
// delivered to the handler. It also pins FR-006/NFR-004's requirement that
// each entry carry the raw wire bytes and method as structured attributes
// (not just an event name substring), since a grep-driven investigation
// depends on raw_id/method actually being present and correct, not merely on
// the event firing.
func TestConnRunLog(t *testing.T) {
	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prevLogger)

	ta, tb := pipeTransport()
	conn := codexclient.NewConn(ta, time.Second)
	recv := make(chan string, 1)
	h := &notifyHandler{recv: recv}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go conn.Run(ctx, h) //nolint:errcheck

	// codexclient.decode_error: not valid JSON at all.
	if err := tb.WriteMessage(ctx, []byte("not-json")); err != nil {
		t.Fatalf("WriteMessage(decode_error case): %v", err)
	}
	// codexclient.invalid_id (object id): JSON-RPC 2.0 forbids object/array
	// ids, and this one can't be parsed as this Conn's own decimal
	// pending-map numbering.
	if err := tb.WriteMessage(ctx, []byte(`{"id":{"x":1}}`)); err != nil {
		t.Fatalf("WriteMessage(invalid_id/object-id case): %v", err)
	}
	// codexclient.invalid_id (overflow numeric id): AC-006's third
	// invalid-envelope case. The id is a syntactically valid JSON number but
	// overflows int64, so strconv.ParseInt in parsePendingID fails and this
	// must be reported the same way as any other unparseable id, not
	// silently coerced or dropped without a log entry.
	const overflowID = "999999999999999999999"
	if err := tb.WriteMessage(ctx, []byte(`{"id":`+overflowID+`}`)); err != nil {
		t.Fatalf("WriteMessage(invalid_id/overflow case): %v", err)
	}
	// codexclient.pending_miss (FR-006 variant): method empty + id=null is a
	// JSON-RPC 2.0 invalid-Request error response, distinct from a
	// Notification (id member absent entirely); it is reported as
	// pending_miss with method="" explicit rather than as invalid_id.
	if err := tb.WriteMessage(ctx, []byte(`{"id":null}`)); err != nil {
		t.Fatalf("WriteMessage(pending_miss/null-id case): %v", err)
	}

	// AC-006: the transport must not be closed by any of the invalid
	// envelopes above — a subsequent valid message is still dispatched.
	if err := tb.WriteMessage(ctx, []byte(`{"method":"hello","params":{"msg":"still alive"}}`)); err != nil {
		t.Fatalf("WriteMessage(valid case): %v", err)
	}
	select {
	case got := <-recv:
		if !strings.Contains(got, "still alive") {
			t.Fatalf("notification params = %q, want to contain %q", got, "still alive")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification following the invalid envelopes")
	}

	logged := buf.String()
	lines := strings.Split(strings.TrimSpace(logged), "\n")

	// findLine returns the single log line whose "msg" is event and which
	// contains every string in extra (used to disambiguate the two
	// codexclient.invalid_id lines by their distinct raw_id), failing the
	// test if the match is missing or ambiguous.
	findLine := func(event string, extra ...string) string {
		t.Helper()
		msgField := `"msg":"` + event + `"`
		var matches []string
		for _, line := range lines {
			if !strings.Contains(line, msgField) {
				continue
			}
			ok := true
			for _, want := range extra {
				if !strings.Contains(line, want) {
					ok = false
					break
				}
			}
			if ok {
				matches = append(matches, line)
			}
		}
		if len(matches) != 1 {
			t.Fatalf("want exactly one log line for event %q (extra=%v), got %d; full log:\n%s", event, extra, len(matches), logged)
		}
		return matches[0]
	}
	// assertAttr fails the test unless line contains attr as a JSON
	// key:value pair with the given string value (json.Marshal handles
	// quoting/escaping so callers can pass raw wire text).
	assertAttr := func(t *testing.T, line, key, value string) {
		t.Helper()
		encodedValue, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal expected value for %q: %v", key, err)
		}
		want := `"` + key + `":` + string(encodedValue)
		if !strings.Contains(line, want) {
			t.Fatalf("log line missing %s; got:\n%s", want, line)
		}
	}

	decodeErrorLine := findLine("codexclient.decode_error")
	assertAttr(t, decodeErrorLine, "raw", "not-json")

	objectIDEncoded, err := json.Marshal(`{"x":1}`)
	if err != nil {
		t.Fatalf("marshal object-id disambiguator: %v", err)
	}
	objectIDLine := findLine("codexclient.invalid_id", string(objectIDEncoded))
	assertAttr(t, objectIDLine, "raw_id", `{"x":1}`)
	assertAttr(t, objectIDLine, "method", "")

	overflowLine := findLine("codexclient.invalid_id", overflowID)
	assertAttr(t, overflowLine, "raw_id", overflowID)
	assertAttr(t, overflowLine, "method", "")

	pendingMissLine := findLine("codexclient.pending_miss")
	assertAttr(t, pendingMissLine, "raw_id", "null")
	assertAttr(t, pendingMissLine, "method", "")
}

// TestStdioTransport_RoundTrip verifies newline-delimited framing.
func TestStdioTransport_RoundTrip(t *testing.T) {
	pr, pw := io.Pipe()
	trW := codexclient.StdioTransport(io.NopCloser(strings.NewReader("")), pw)
	trR := codexclient.StdioTransport(pr, io.Discard)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg := []byte(`{"method":"hello"}`)
	// Write and read must be concurrent because io.Pipe is unbuffered.
	writeErr := make(chan error, 1)
	go func() { writeErr <- trW.WriteMessage(ctx, msg) }()

	got, err := trR.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("WriteMessage: %v", werr)
	}
	if string(got) != string(msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

// TestStdioTransport_MultiMessage verifies multiple messages in sequence.
func TestStdioTransport_MultiMessage(t *testing.T) {
	pr, pw := io.Pipe()
	scanner := bufio.NewScanner(pr)
	tr := codexclient.StdioTransport(io.NopCloser(strings.NewReader("")), pw)
	ctx := context.Background()

	msgs := []string{`{"id":1}`, `{"method":"ping"}`, `{"result":"ok"}`}
	go func() {
		for _, m := range msgs {
			_ = tr.WriteMessage(ctx, []byte(m))
		}
	}()

	for _, want := range msgs {
		if !scanner.Scan() {
			t.Fatal("scanner stopped early")
		}
		if scanner.Text() != want {
			t.Fatalf("got %q, want %q", scanner.Text(), want)
		}
	}
}

// TestStdioTransport_LargeMessage verifies that a message larger than the
// bufio.Scanner default token size (64 KiB) round-trips intact. Codex/Claude
// turn events carrying diffs or file contents routinely exceed that bound.
func TestStdioTransport_LargeMessage(t *testing.T) {
	pr, pw := io.Pipe()
	trW := codexclient.StdioTransport(io.NopCloser(strings.NewReader("")), pw)
	trR := codexclient.StdioTransport(pr, io.Discard)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := strings.Repeat("x", 1<<20) // 1 MiB, well past the 64 KiB default
	params := codexclient.TurnCompletedParams("thread-1", "turn-1")
	params["padding"] = payload
	msg, err := json.Marshal(map[string]any{
		"method": "turn/completed",
		"params": params,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	writeErr := make(chan error, 1)
	go func() { writeErr <- trW.WriteMessage(ctx, msg) }()

	got, err := trR.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("WriteMessage: %v", werr)
	}
	if len(got) != len(msg) {
		t.Fatalf("got %d bytes, want %d", len(got), len(msg))
	}
}

// --- test doubles ---

type echoHandler struct{ conn *codexclient.Conn }

func (h *echoHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *echoHandler) OnServerRequest(id codexclient.RequestID, _ string, params json.RawMessage) {
	_ = h.conn.Reply(id, params)
}

type notifyHandler struct{ recv chan string }

func (h *notifyHandler) OnNotification(_ string, params json.RawMessage) {
	h.recv <- string(params)
}
func (h *notifyHandler) OnServerRequest(_ codexclient.RequestID, _ string, _ json.RawMessage) {}

type noopHandler struct{}

func (h *noopHandler) OnNotification(_ string, _ json.RawMessage)                           {}
func (h *noopHandler) OnServerRequest(_ codexclient.RequestID, _ string, _ json.RawMessage) {}

type replyErrorHandler struct {
	conn *codexclient.Conn
	msg  string
}

func (h *replyErrorHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *replyErrorHandler) OnServerRequest(id codexclient.RequestID, _ string, _ json.RawMessage) {
	_ = h.conn.ReplyError(id, h.msg)
}
