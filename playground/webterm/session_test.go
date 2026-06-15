package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// drainUntil reads frames from ch until pred matches or the deadline elapses.
func drainUntil(t *testing.T, ch <-chan []byte, pred func([]byte) bool, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before match")
			}
			if pred(f) {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for frame")
		}
	}
}

// isOutputContaining reports whether b is an asciicast output frame whose data
// contains sub.
func isOutputContaining(b []byte, sub string) bool {
	var arr []any
	if json.Unmarshal(b, &arr) != nil || len(arr) != 3 {
		return false
	}
	code, _ := arr[1].(string)
	data, _ := arr[2].(string)
	return code == "o" && strings.Contains(data, sub)
}

func isControl(b []byte, kind string, code int, dataSub string) bool {
	var m controlMsg
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	return m.K == kind && m.Code == code && strings.Contains(m.Data, dataSub)
}

func TestSessionEchoesInput(t *testing.T) {
	s, err := NewSession([]string{"cat"}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	s.WriteInput([]byte("ping-123\n"))
	drainUntil(t, ch, func(b []byte) bool { return isOutputContaining(b, "ping-123") }, 3*time.Second)
}

func TestSessionCapturesOSC9(t *testing.T) {
	// printf emits an OSC 9 desktop-notification sequence; the server must
	// surface it as a structured control event rather than raw bytes.
	s, err := NewSession([]string{"bash", "-c", `printf '\033]9;hello-notif\a'; sleep 0.3`}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	drainUntil(t, ch, func(b []byte) bool { return isControl(b, "osc", 9, "hello-notif") }, 3*time.Second)
}

func TestSessionCapturesOSC133Prompt(t *testing.T) {
	s, err := NewSession([]string{"bash", "-c", `printf '\033]133;A\a'; sleep 0.3`}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	drainUntil(t, ch, func(b []byte) bool { return isControl(b, "prompt", 133, "A") }, 3*time.Second)
}

func TestSessionCapturesTitle(t *testing.T) {
	s, err := NewSession([]string{"bash", "-c", `printf '\033]0;my-title\a'; sleep 0.3`}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	drainUntil(t, ch, func(b []byte) bool { return isControl(b, "title", 0, "my-title") }, 3*time.Second)
}

func TestSessionReattachSnapshotFirst(t *testing.T) {
	s, err := NewSession([]string{"sleep", "1"}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	select {
	case f := <-ch:
		var arr []any
		if json.Unmarshal(f, &arr) != nil || len(arr) != 3 || arr[1] != "o" {
			t.Fatalf("first frame is not an output snapshot: %s", f)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot frame")
	}
}

func TestSessionResize(t *testing.T) {
	s, err := NewSession([]string{"sleep", "1"}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if s.cols != 100 || s.rows != 30 {
		t.Fatalf("resize not applied: got %dx%d", s.cols, s.rows)
	}
}

// TestWSEndToEnd exercises the full http → websocket → pty → emulator → frame
// path: dial the ws endpoint, send input, and read the echoed output frame.
func TestWSEndToEnd(t *testing.T) {
	s, err := NewSession([]string{"cat"}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) { serveWS(s, w, r) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"i","d":"wshello\n"}`)); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if isOutputContaining(data, "wshello") {
			return
		}
	}
}
