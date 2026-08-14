package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/services/host/proto"
)

// Contract: contract-transport-close-taxonomy
// (adr-20260814-transport-close-reason-taxonomy).
//
// Before this contract the gateway had one close code, four ad-hoc reason
// strings no schema modelled, and most departures unwound into an abrupt close
// carrying nothing — so a draining server, a rejected credential and a pulled
// cable were mutually indistinguishable. These tests pin that every departure
// now names itself, on both sides of the WebSocket upgrade.

// TestEveryServerInitiatedCloseCarriesAPublishedReason is
// ver-every-server-initiated-close-carries-a-published-reason.
//
// It fails an implementation that leaves any departure unwinding into an
// abrupt close with no reason — which is what graceful shutdown and every
// write-error path did before.
func TestEveryServerInitiatedCloseCarriesAPublishedReason(t *testing.T) {
	t.Parallel()

	refusal := func(code proto.ErrCode) error {
		return &proto.ErrorBody{Code: code, Message: string(code)}
	}

	cases := []struct {
		name string
		// depart drives the socket to the departure under test. It returns
		// after the server has been told to leave, not after it has left.
		depart func(t *testing.T, h *departureHarness)
		// sendGeometry is false for the malformed-opening-frame case, which
		// must not send a valid first frame.
		sendGeometry bool
		lifecycle    bool
		subscribeErr error
		want         CloseReason
		// wantLegacy pins the pre-taxonomy `data` payload that shipped clients
		// still match on, where one existed.
		wantLegacy string
	}{
		{
			name:         "graceful shutdown",
			sendGeometry: true,
			depart:       func(_ *testing.T, h *departureHarness) { h.cancel() },
			want:         CloseGoingAway,
		},
		{
			name:         "backend loss",
			sendGeometry: true,
			depart:       func(_ *testing.T, h *departureHarness) { close(h.terminal.events) },
			want:         CloseBackendUnavailable,
			wantLegacy:   legacyDaemonGoneData,
		},
		{
			name:         "capacity refusal",
			sendGeometry: true,
			subscribeErr: refusal(proto.ErrResourceExhausted),
			depart:       func(*testing.T, *departureHarness) {},
			want:         CloseCapacity,
			wantLegacy:   string(proto.ErrResourceExhausted),
		},
		{
			name:         "unattachable target",
			sendGeometry: true,
			subscribeErr: refusal(proto.ErrNotFound),
			depart:       func(*testing.T, *departureHarness) {},
			want:         CloseInvalidRequest,
			wantLegacy:   string(proto.ErrNotFound),
		},
		{
			name:         "opaque server failure",
			sendGeometry: true,
			subscribeErr: errors.New("subsystem exploded"),
			depart:       func(*testing.T, *departureHarness) {},
			want:         CloseInternalError,
		},
		{
			name:         "malformed opening frame",
			sendGeometry: false,
			depart: func(t *testing.T, h *departureHarness) {
				h.write(t, `{"k":"i","d":"no geometry here"}`)
			},
			want: CloseInvalidRequest,
		},
		{
			name:      "lifecycle graceful shutdown",
			lifecycle: true,
			depart:    func(_ *testing.T, h *departureHarness) { h.cancel() },
			want:      CloseGoingAway,
		},
		{
			name:       "lifecycle backend loss",
			lifecycle:  true,
			depart:     func(_ *testing.T, h *departureHarness) { close(h.lifecycle.events) },
			want:       CloseBackendUnavailable,
			wantLegacy: legacyDaemonGoneData,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := startDepartureHarness(t, tc.lifecycle, tc.subscribeErr)
			if tc.sendGeometry {
				h.write(t, `{"k":"r","cols":80,"rows":24}`)
			}
			if tc.sendGeometry && tc.subscribeErr == nil {
				h.waitForAttach(t)
			}
			tc.depart(t, h)

			got := h.observeDeparture(t)
			if got.closeReason != string(tc.want) {
				t.Errorf("close frame reason = %q, want %q", got.closeReason, tc.want)
			}
			if got.closeCode != websocket.StatusGoingAway {
				t.Errorf("close code = %v, want %v — the taxonomy lives in the reason slot",
					got.closeCode, websocket.StatusGoingAway)
			}
			if got.controlReason != string(tc.want) {
				t.Errorf("departure control frame reason = %q, want %q — the two-step close must carry the same reason",
					got.controlReason, tc.want)
			}
			if got.controlLegacy != tc.wantLegacy {
				t.Errorf("departure control frame data = %q, want %q — the pre-taxonomy payload shipped clients match on must survive",
					got.controlLegacy, tc.wantLegacy)
			}
		})
	}
}

// departureHarness runs one attach handler over a real socket with a
// connection context the test can cancel, which is how graceful shutdown is
// reproduced: the process draining is the request context ending.
type departureHarness struct {
	terminal  *fakeSessionAttacher
	lifecycle *fakeLifecycleAttacher
	conn      *websocket.Conn
	cancel    context.CancelFunc
}

func startDepartureHarness(t *testing.T, lifecycle bool, subscribeErr error) *departureHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	h := &departureHarness{cancel: cancel}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		if lifecycle {
			_ = AttachLifecycleWS(ctx, h.lifecycle, c, "test-ci")
			return
		}
		_ = AttachWS(ctx, h.terminal, "s1", c)
	})

	if lifecycle {
		h.lifecycle = newFakeLifecycleAttacher()
		h.lifecycle.subscribeErr = subscribeErr
	} else {
		h.terminal = newFakeAttacher()
		h.terminal.subscribeErr = subscribeErr
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()
	c, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	h.conn = c
	return h
}

// waitForAttach blocks until the server has subscribed to the session. The
// contract is about a client that is attached: cancelling while the opening
// frame is still in flight is a handshake that never completed, not a
// departure from an attachment, and racing it would test the wrong thing.
func (h *departureHarness) waitForAttach(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.terminal.mu.Lock()
		attached := h.terminal.subscribeCall != nil
		h.terminal.mu.Unlock()
		if attached {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("server never subscribed to the session")
}

func (h *departureHarness) write(t *testing.T, frame string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		t.Fatalf("write %s: %v", frame, err)
	}
}

// departureObservation is everything a client can learn about why it lost the
// socket, read only through the public contract.
type departureObservation struct {
	closeCode     websocket.StatusCode
	closeReason   string
	controlReason string
	controlLegacy string
}

// observeDeparture drains frames until the socket ends, then reports the close
// frame and the departure control frame that preceded it. A socket that ends
// without a close frame is the failure this verification exists to catch.
func (h *departureHarness) observeDeparture(t *testing.T) departureObservation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	obs := departureObservation{}
	for range 64 {
		_, data, err := h.conn.Read(ctx)
		if err != nil {
			var ce websocket.CloseError
			if !errors.As(err, &ce) {
				t.Fatalf("socket ended with no close frame — the client cannot attribute this departure: %v", err)
			}
			obs.closeCode = ce.Code
			obs.closeReason = ce.Reason
			return obs
		}
		var frame struct {
			K      string `json:"k"`
			Reason string `json:"reason"`
			Data   string `json:"data"`
		}
		if json.Unmarshal(data, &frame) == nil && frame.K == "c" && frame.Reason != "" {
			obs.controlReason = frame.Reason
			obs.controlLegacy = frame.Data
		}
	}
	t.Fatal("socket never departed")
	return obs
}
