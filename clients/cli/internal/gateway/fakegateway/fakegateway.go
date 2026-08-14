// Package fakegateway is the fake half of the gateway boundary's registered
// triple (test-harness/dependencies.json).
//
// It answers the same REST routes and speaks the same frames as the real
// gateway, in process and without a listener. Its shapes come from the Phase 0
// captures under internal/gateway/testdata, which is what stops it drifting
// into a convenient dialect that only this client understands: a fake that
// answers a question the real server would refuse makes every test built on it
// meaningless in exactly the way that is hardest to notice.
package fakegateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
)

// Fake is one server. Every field is a decision a real server makes, exposed so
// a test states it rather than arranging for it.
type Fake struct {
	mu sync.Mutex

	// ProtocolVersion is what /api/capabilities answers. Setting it to
	// something other than gateway.ProtocolVersion produces a real mismatch.
	ProtocolVersion string
	// Ticket is the value /api/ws-ticket mints. Empty mints an empty ticket,
	// which is the server-side fault the CLI must not blame on a credential.
	Ticket string
	// ClientInstanceID is the identity the mint hands back.
	ClientInstanceID string
	// Sessions is the session list the hello frame carries.
	Sessions []gateway.SessionInfo
	// Features, Capabilities and ServerTime are the three hello fields the CLI
	// reads none of — they exist because the real server always sends them, and
	// a fake whose hello frame is missing keys is a fake that would hide a
	// decoder which broke on their presence. TestE2E_FakeVsRealGatewayAdmission
	// caught exactly that. Their values are deterministic placeholders; the
	// contract under test is that the key is there and carries the right JSON
	// type, not what a particular build happens to advertise.
	Features     []string
	Capabilities []string
	ServerTime   int64
	// RejectTicketMint answers the mint route with the published
	// unauthenticated rejection body.
	RejectTicketMint bool
	// RefuseDial fails the WebSocket dial itself.
	RefuseDial error
	// SubscribeCode, when set, refuses every subscription with this daemon
	// error code — "resource_exhausted" being the capacity refusal.
	SubscribeCode string
	// SubscribeOutcomes is the sequence of lifecycle outcome statuses to answer
	// a subscription with. Empty means a single "applied".
	SubscribeOutcomes []string
	// Snapshot is the output payload delivered immediately on a successful
	// subscription — the session's current screen.
	Snapshot []byte
	// CreatedSession is the body POST /api/sessions returns.
	CreatedSession map[string]any
	// CreateStatus overrides the creation status code.
	CreateStatus int
	// SessionConfig is the body GET /api/session-config returns.
	SessionConfig map[string]any
	// DeleteStatus is the status DELETE /api/sessions/{id} returns.
	DeleteStatus int
	// DeleteCode is the published rejection code a failing delete carries.
	DeleteCode gateway.CloseReason

	// TicketMints counts how many tickets were minted, so a test can assert
	// that each dial minted its own.
	TicketMints int
	// Deletes records every session identifier a termination was requested for.
	// It is what proves no other path emits one.
	Deletes []string

	sockets []*Socket
}

// fixedServerTime is the epoch-millisecond instant every fake hello frame
// carries: 2026-08-14T00:00:00Z.
const fixedServerTime int64 = 1786752000000

// New builds a fake that behaves like a healthy server.
func New(sessions ...gateway.SessionInfo) *Fake {
	return &Fake{
		ProtocolVersion:  gateway.ProtocolVersion,
		Ticket:           "ticket-1",
		ClientInstanceID: "ci-1",
		Sessions:         sessions,
		DeleteStatus:     http.StatusNoContent,
		// A fixed instant rather than the clock: a fake that changed its hello
		// frame between two runs would make every byte-level assertion over it
		// flaky for a reason that has nothing to do with the code under test.
		ServerTime: fixedServerTime,
	}
}

// Doer returns the HTTP seam.
func (fake *Fake) Doer() gateway.Doer { return doerFunc(fake.serve) }

// Dialer returns the WebSocket seam.
func (fake *Fake) Dialer() gateway.Dialer {
	return func(_ context.Context, endpoint string) (gateway.Socket, error) {
		fake.mu.Lock()
		refuse := fake.RefuseDial
		fake.mu.Unlock()
		if refuse != nil {
			return nil, refuse
		}
		if !strings.Contains(endpoint, "ticket=") {
			return nil, errors.New("dial without a ticket")
		}
		socket := newSocket(fake)
		fake.mu.Lock()
		fake.sockets = append(fake.sockets, socket)
		fake.mu.Unlock()
		socket.pushHello()
		return socket, nil
	}
}

// Sockets returns every socket that was opened, in order.
func (fake *Fake) Sockets() []*Socket {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]*Socket(nil), fake.sockets...)
}

type doerFunc func(*http.Request) (*http.Response, error)

func (do doerFunc) Do(request *http.Request) (*http.Response, error) { return do(request) }

func (fake *Fake) serve(request *http.Request) (*http.Response, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	switch {
	case request.URL.Path == "/api/capabilities":
		return jsonResponse(http.StatusOK, map[string]any{
			"protocolVersion": fake.ProtocolVersion,
		})
	case request.URL.Path == "/api/ws-ticket":
		if fake.RejectTicketMint {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"code":    string(gateway.ReasonUnauthenticated),
				"message": "missing, unknown, already-used or expired ticket",
			})
		}
		fake.TicketMints++
		return jsonResponse(http.StatusOK, map[string]any{
			"ticket":             fmt.Sprintf("%s-%d", fake.Ticket, fake.TicketMints),
			"client_instance_id": fake.ClientInstanceID,
		})
	case request.URL.Path == "/api/session-config":
		body := fake.SessionConfig
		if body == nil {
			body = map[string]any{
				"push_commands": []string{}, "base_command_profiles": []any{},
				"project_roots": []string{}, "project_paths": []string{}, "projects": []any{},
			}
		}
		return jsonResponse(http.StatusOK, body)
	case request.Method == http.MethodPost && request.URL.Path == "/api/sessions":
		status := fake.CreateStatus
		if status == 0 {
			status = http.StatusCreated
		}
		body := fake.CreatedSession
		if body == nil {
			body = map[string]any{"id": "s_created", "project": "/repo", "command": "claude"}
		}
		return jsonResponse(status, body)
	case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/api/sessions/"):
		id := strings.TrimPrefix(request.URL.Path, "/api/sessions/")
		fake.Deletes = append(fake.Deletes, id)
		if fake.DeleteStatus >= 200 && fake.DeleteStatus < 300 {
			return jsonResponse(fake.DeleteStatus, map[string]any{})
		}
		code := fake.DeleteCode
		if code == "" {
			code = gateway.ReasonInvalidRequest
		}
		return jsonResponse(fake.DeleteStatus, map[string]any{
			"code": string(code), "message": "the server refused the termination",
		})
	}
	return jsonResponse(http.StatusNotFound, map[string]any{
		"code": string(gateway.ReasonInvalidRequest), "message": "no such route",
	})
}

func jsonResponse(status int, body any) (*http.Response, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(encoded))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// Socket is one open lifecycle connection.
//
// Its whole job is to make every server behaviour a method: a test says
// "deliver this output", "refuse for capacity", "go away saying why", "go away
// saying nothing", rather than arranging conditions and hoping.
type Socket struct {
	fake *Fake

	mu       sync.Mutex
	written  [][]byte
	closed   bool
	reason   gateway.CloseReason
	incoming chan []byte
	done     chan struct{}
}

func newSocket(fake *Fake) *Socket {
	return &Socket{
		fake:     fake,
		incoming: make(chan []byte, 64),
		done:     make(chan struct{}),
		reason:   gateway.ReasonUnattributed,
	}
}

func (socket *Socket) pushHello() {
	socket.fake.mu.Lock()
	sessions := socket.fake.Sessions
	features := socket.fake.Features
	capabilities := socket.fake.Capabilities
	serverTime := socket.fake.ServerTime
	socket.fake.mu.Unlock()
	// The real server replaces nil slices with empty ones so a client always
	// receives arrays. A fake that emitted JSON null here would be a laxer
	// counterparty than the thing it stands for.
	if features == nil {
		features = []string{}
	}
	if capabilities == nil {
		capabilities = []string{}
	}
	frame, _ := json.Marshal(map[string]any{
		"k":                "h",
		"sessions":         sessions,
		"protocolVersion":  gateway.ProtocolVersion,
		"clientInstanceId": socket.fake.ClientInstanceID,
		"features":         features,
		"capabilities":     capabilities,
		"serverTime":       serverTime,
	})
	socket.Push(frame)
}

// Read yields the next frame, or the error that ended the socket.
func (socket *Socket) Read(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case frame, ok := <-socket.incoming:
		if !ok {
			return nil, io.EOF
		}
		return frame, nil
	case <-socket.done:
		return nil, io.EOF
	}
}

// Write records an outbound frame and answers it the way the real gateway
// would.
func (socket *Socket) Write(_ context.Context, frame []byte) error {
	socket.mu.Lock()
	if socket.closed {
		socket.mu.Unlock()
		return errors.New("write on a closed socket")
	}
	socket.written = append(socket.written, append([]byte(nil), frame...))
	socket.mu.Unlock()
	socket.answer(frame)
	return nil
}

// answer replies to a desired publication exactly as handleLifecycleDesired
// does: a request envelope first, then the daemon's authoritative outcome.
func (socket *Socket) answer(frame []byte) {
	var inbound struct {
		Kind        string `json:"k"`
		RequestID   string `json:"reqId"`
		SessionID   string `json:"sessionId"`
		Desired     bool   `json:"desired"`
		Correlation struct {
			ClientRevision uint64 `json:"clientRevision"`
		} `json:"correlation"`
	}
	if json.Unmarshal(frame, &inbound) != nil || inbound.Kind != "ld" {
		return
	}
	socket.fake.mu.Lock()
	code := socket.fake.SubscribeCode
	outcomes := append([]string(nil), socket.fake.SubscribeOutcomes...)
	snapshot := socket.fake.Snapshot
	socket.fake.mu.Unlock()

	if !inbound.Desired {
		socket.pushResponseOK(inbound.RequestID)
		return
	}
	if code != "" {
		socket.pushResponseError(inbound.RequestID, code, "the server refused the subscription")
		return
	}
	socket.pushResponseOK(inbound.RequestID)
	if len(outcomes) == 0 {
		outcomes = []string{"applied"}
	}
	for _, status := range outcomes {
		socket.pushOutcome(inbound.Correlation.ClientRevision, status)
	}
	if snapshot != nil {
		socket.PushOutput(inbound.SessionID, snapshot)
	}
}

func (socket *Socket) pushResponseOK(requestID string) {
	frame, _ := json.Marshal(map[string]any{"k": "r", "reqId": requestID})
	socket.Push(frame)
}

func (socket *Socket) pushResponseError(requestID, code, message string) {
	frame, _ := json.Marshal(map[string]any{
		"k": "e", "reqId": requestID, "code": code, "message": message,
	})
	socket.Push(frame)
}

func (socket *Socket) pushOutcome(revision uint64, status string) {
	frame, _ := json.Marshal(map[string]any{
		"k": "lo",
		"correlation": map[string]any{
			"clientInstanceID":     socket.fake.ClientInstanceID,
			"connectionGeneration": 1, "clientRevision": revision,
		},
		"status": status,
	})
	socket.Push(frame)
}

// PushOutput delivers surface output for one session, in the asciicast shape
// the real gateway uses — including the fourth element that routes it.
func (socket *Socket) PushOutput(sessionID string, data []byte) {
	frame, _ := json.Marshal([]any{
		0.0, "o", base64.StdEncoding.EncodeToString(data), sessionID,
	})
	socket.Push(frame)
}

// PushViewUpdate delivers a session-list snapshot, which is how a session end
// reaches an attached client.
func (socket *Socket) PushViewUpdate(sessions []gateway.SessionInfo) {
	frame, _ := json.Marshal(map[string]any{"k": "v", "sessions": sessions})
	socket.Push(frame)
}

// Push delivers a raw frame.
func (socket *Socket) Push(frame []byte) {
	select {
	case socket.incoming <- frame:
	case <-socket.done:
	}
}

// Depart ends the connection the way the real gateway does: a control frame
// naming the reason while the socket is still writable, then the close.
func (socket *Socket) Depart(reason gateway.CloseReason) {
	frame, _ := json.Marshal(map[string]any{"k": "c", "reason": string(reason)})
	socket.Push(frame)
	socket.mu.Lock()
	socket.reason = reason
	socket.mu.Unlock()
	socket.finish()
}

// DropAbruptly ends the connection carrying nothing at all — a pulled cable, a
// killed process, a proxy timing out. It is the case the client must present as
// an unattributed loss rather than guessing it into a named cause.
func (socket *Socket) DropAbruptly() {
	socket.mu.Lock()
	socket.reason = gateway.ReasonUnattributed
	socket.mu.Unlock()
	socket.finish()
}

func (socket *Socket) finish() {
	socket.mu.Lock()
	defer socket.mu.Unlock()
	if socket.closed {
		return
	}
	socket.closed = true
	close(socket.done)
}

// Close ends the socket from the client side.
func (socket *Socket) Close() error {
	socket.finish()
	return nil
}

// CloseReason reports what the peer said on the way out.
func (socket *Socket) CloseReason() gateway.CloseReason {
	socket.mu.Lock()
	defer socket.mu.Unlock()
	return socket.reason
}

// Written returns every frame the client sent, in order.
func (socket *Socket) Written() [][]byte {
	socket.mu.Lock()
	defer socket.mu.Unlock()
	return append([][]byte(nil), socket.written...)
}
