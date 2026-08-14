package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/takezoh/agent-grid/services/host/proto"
)

// Contract: contract-transport-close-taxonomy
// (adr-20260814-transport-close-reason-taxonomy).

// TestTicketAndVersionRejectionsCarryTheSameReasonVocabulary is
// ver-pre-upgrade-rejections-carry-the-same-vocabulary.
//
// A rejection that happens before websocket.Accept cannot carry a close
// reason, so without a body a client sees only a failed handshake — the same
// thing it sees when the network drops. This fails an implementation that
// answers a rejected ticket with the plain-text `unauthorized` it used to.
func TestTicketAndVersionRejectionsCarryTheSameReasonVocabulary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		request func(t *testing.T) *http.Request
		handler func(t *testing.T) http.Handler
		status  int
		want    CloseReason
	}{
		{
			name: "rejected ws ticket",
			request: func(*testing.T) *http.Request {
				return protocolRequest(http.MethodGet, "/ws?ticket=never-minted", nil)
			},
			handler: func(t *testing.T) http.Handler { return NewMux(healthyDaemonClient(t), "secret") },
			status:  http.StatusUnauthorized,
			want:    CloseUnauthenticated,
		},
		{
			name: "rejected bearer token",
			request: func(*testing.T) *http.Request {
				return protocolRequest(http.MethodGet, "/api/sessions", nil)
			},
			handler: func(t *testing.T) http.Handler { return NewMux(healthyDaemonClient(t), "secret") },
			status:  http.StatusUnauthorized,
			want:    CloseUnauthenticated,
		},
		{
			name: "mismatched protocol version on the upgrade",
			request: func(*testing.T) *http.Request {
				r := protocolRequest(http.MethodGet, "/ws?ticket=never-minted", nil)
				r.Header.Set(ProtocolVersionHeader, "protocol-from-another-build")
				return r
			},
			handler: func(t *testing.T) http.Handler { return NewMux(healthyDaemonClient(t), "secret") },
			status:  http.StatusUpgradeRequired,
			want:    CloseIncompatibleProtocol,
		},
		{
			name: "mismatched protocol version on the REST API",
			request: func(*testing.T) *http.Request {
				r := protocolRequest(http.MethodGet, "/api/sessions", nil)
				r.Header.Set("Authorization", "Bearer secret")
				r.Header.Set(ProtocolVersionHeader, "protocol-from-another-build")
				return r
			},
			handler: func(t *testing.T) http.Handler { return NewMux(healthyDaemonClient(t), "secret") },
			status:  http.StatusUpgradeRequired,
			want:    CloseIncompatibleProtocol,
		},
		{
			name: "backend down before the upgrade",
			request: func(*testing.T) *http.Request {
				return protocolRequest(http.MethodGet, "/ws?ticket=never-minted", nil)
			},
			handler: func(t *testing.T) http.Handler { return NewMux(unhealthyDaemonClient(t), "secret") },
			status:  http.StatusServiceUnavailable,
			want:    CloseBackendUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			tc.handler(t).ServeHTTP(w, tc.request(t))

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%q", w.Code, tc.status, w.Body.String())
			}
			var body struct {
				Code CloseReason `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("rejection body is not JSON — a client has nothing typed to read: %q", w.Body.String())
			}
			if body.Code != tc.want {
				t.Errorf("rejection code = %q, want %q", body.Code, tc.want)
			}
			if !isPublishedCloseReason(body.Code) {
				t.Errorf("rejection code %q is outside the published vocabulary", body.Code)
			}
		})
	}
}

func isPublishedCloseReason(reason CloseReason) bool {
	return slices.Contains(PublishedCloseReasons, reason)
}

// healthyDaemonClient dials over an in-memory pipe, so Health() is true
// without binding a socket. The far end never answers, which is fine: these
// rejections all happen before any daemon RPC.
func healthyDaemonClient(t *testing.T) *DaemonClient {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { _ = serverConn.Close() })
	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return proto.DialConn(clientConn), nil
	}, testMinDelay, testMaxDelay)
	t.Cleanup(d.Close)
	if !d.Health() {
		t.Fatal("daemon client did not come up healthy")
	}
	return d
}

func unhealthyDaemonClient(t *testing.T) *DaemonClient {
	t.Helper()
	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return nil, net.ErrClosed
	}, testMinDelay, testMaxDelay)
	t.Cleanup(d.Close)
	if d.Health() {
		t.Fatal("daemon client unexpectedly healthy")
	}
	return d
}
