package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// client.go is one configured server's transport. Everything the CLI sends to a
// server goes through it, which is what makes admission a gate rather than a
// convention: a mismatched build cannot reach an operational route by taking a
// different path, because there is no other path.

// requestTimeout bounds a single REST call. Attach holds its socket open
// indefinitely and is not bounded by this.
const requestTimeout = 10 * time.Second

// Doer is the HTTP seam. A fake substitutes for it in tests; production passes
// an *http.Client.
type Doer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Socket is the subset of a WebSocket connection this module uses. Keeping it
// this narrow is what lets the fake gateway drive every disconnect shape
// without a real listener.
type Socket interface {
	// Read returns the next text frame, or the error that ended the socket.
	Read(ctx context.Context) ([]byte, error)
	// Write sends one text frame.
	Write(ctx context.Context, frame []byte) error
	// Close ends the connection from this side.
	Close() error
	// CloseReason reports what the peer said when it closed, or
	// ReasonUnattributed when it said nothing. A client that guesses here is
	// exactly what contract-transport-close-taxonomy exists to prevent.
	CloseReason() CloseReason
}

// Dialer opens a Socket. It is a seam for the same reason Doer is.
type Dialer func(ctx context.Context, endpoint string) (Socket, error)

// Client is the transport for one configured server entry.
type Client struct {
	server clientconfig.Server
	doer   Doer
	dialer Dialer

	mu sync.Mutex
	// terminal is set once a server is known to be permanently unusable for
	// this process — today only a protocol mismatch. Once set, every
	// operational call returns it without touching the network, which is what
	// "prevents any further operational request or socket frame" means
	// mechanically rather than as a promise about call sites.
	terminal *TransportError
	admitted bool
}

// New builds a client for one server entry.
//
// A configuration-invalid entry must never be probed, so it is refused here
// rather than at each call site: the contract says such an entry can never
// acquire a reachability observation, and the only way to guarantee that is for
// no transport to exist for it.
func New(server clientconfig.Server, doer Doer, dialer Dialer) (*Client, error) {
	if server.Admission() != clientconfig.AdmissionUsable {
		return nil, &TransportError{
			Reason: ReasonInvalidRequest,
			Kind:   ErrInvalidRequest,
			Detail: "the configured entry is not usable and is never connected to",
		}
	}
	if doer == nil {
		doer = &http.Client{Timeout: requestTimeout}
	}
	if dialer == nil {
		dialer = dialWebSocket
	}
	return &Client{server: server, doer: doer, dialer: dialer}, nil
}

// Server returns the configuration entry this client speaks for.
func (client *Client) Server() clientconfig.Server { return client.server }

// secrets are the values that must never reach a diagnostic.
func (client *Client) secrets() []string {
	if client.server.Auth.Mode == "bearer" {
		return []string{client.server.Auth.Token}
	}
	return nil
}

// capabilitiesResponse is the version-discovery body. It is the only route that
// is not version-gated, which is what makes discovery possible at all.
type capabilitiesResponse struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// Admit performs version discovery and records a mismatch as terminal.
//
// It is fail-closed in both directions: a server that answers with a different
// version is terminal, and a server that cannot be asked at all is a transport
// failure rather than an assumed match.
func (client *Client) Admit(ctx context.Context) error {
	if failure := client.gate(); failure != nil {
		return failure
	}
	response, err := client.do(ctx, http.MethodGet, "/api/capabilities", nil)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return client.classifyStatus(response)
	}
	var body capabilitiesResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return wrapTransport(ReasonBackendUnavailable, err, client.secrets()...)
	}
	if body.ProtocolVersion != ProtocolVersion {
		return client.markTerminal(protocolMismatch(body.ProtocolVersion))
	}
	client.mu.Lock()
	client.admitted = true
	client.mu.Unlock()
	return nil
}

// Admitted reports whether version discovery has succeeded for this server.
func (client *Client) Admitted() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.admitted
}

func (client *Client) gate() *TransportError {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.terminal
}

func (client *Client) markTerminal(failure *TransportError) *TransportError {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.terminal == nil {
		client.terminal = failure
	}
	client.admitted = false
	return client.terminal
}

// do issues one REST request with the credential and the compiled protocol
// version attached. Every operational call in this module goes through it, so
// "every operational request carries both" is a property of one function.
func (client *Client) do(
	ctx context.Context, method, path string, body []byte,
) (*http.Response, error) {
	if failure := client.gate(); failure != nil {
		return nil, failure
	}
	endpoint := strings.TrimRight(client.server.URL, "/") + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, wrapTransport(ReasonInvalidRequest, err, client.secrets()...)
	}
	request.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	applyCredential(request, client.server.Auth)

	response, err := client.doer.Do(request)
	if err != nil {
		return nil, wrapTransport(ReasonBackendUnavailable, err, client.secrets()...)
	}
	// A version rejection can arrive on any operational route, not only on
	// discovery — a server can be replaced while this process runs. Recording
	// it here means the next call is refused locally rather than repeating a
	// request that cannot succeed.
	if response.StatusCode == http.StatusUpgradeRequired {
		actual := decodeVersionRejection(response)
		_ = response.Body.Close()
		return nil, client.markTerminal(protocolMismatch(actual))
	}
	return response, nil
}

// decodeVersionRejection reads the server's stated version out of a 426 body.
// The value is sanitized before it can reach any output.
func decodeVersionRejection(response *http.Response) string {
	var body struct {
		ActualVersion string `json:"actualVersion"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&body) != nil {
		return ""
	}
	return body.ActualVersion
}

// rejectionBody is the pre-upgrade rejection shape, which carries the same
// closed vocabulary as a close frame.
type rejectionBody struct {
	Code    CloseReason `json:"code"`
	Message string      `json:"message"`
}

// classifyStatus turns a non-2xx response into a TransportError, preferring the
// reason the server published over anything derived from the status code.
func (client *Client) classifyStatus(response *http.Response) *TransportError {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	var body rejectionBody
	if json.Unmarshal(raw, &body) == nil && body.Code.IsPublished() {
		return FromCloseReason(body.Code, redact(body.Message, client.secrets()...))
	}
	return FromCloseReason(reasonForStatus(response.StatusCode), "")
}

// reasonForStatus is the fallback for a server that answered an error without
// the published vocabulary — an older build, or a proxy in between. It maps to
// the same vocabulary so the CLI has exactly one thing to classify on.
func reasonForStatus(status int) CloseReason {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ReasonUnauthenticated
	case status == http.StatusUpgradeRequired:
		return ReasonIncompatibleProtocol
	case status == http.StatusNotFound:
		return ReasonInvalidRequest
	case status == http.StatusTooManyRequests:
		return ReasonCapacity
	case status == http.StatusServiceUnavailable || status == http.StatusBadGateway ||
		status == http.StatusGatewayTimeout:
		return ReasonBackendUnavailable
	case status >= 500:
		return ReasonInternalError
	default:
		return ReasonInvalidRequest
	}
}

// GetJSON performs an operational GET and decodes the body into target.
func (client *Client) GetJSON(ctx context.Context, path string, target any) error {
	return client.requestJSON(ctx, http.MethodGet, path, nil, target)
}

// PostJSON performs an operational POST and decodes the body into target.
// target may be nil when the response carries nothing worth reading.
func (client *Client) PostJSON(ctx context.Context, path string, body, target any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return wrapTransport(ReasonInvalidRequest, err)
	}
	return client.requestJSON(ctx, http.MethodPost, path, encoded, target)
}

// Delete performs an operational DELETE and reports whether the server applied
// it. It decodes no body: a termination's only success signal is the
// acknowledgement itself.
func (client *Client) Delete(ctx context.Context, path string) error {
	return client.requestJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (client *Client) requestJSON(
	ctx context.Context, method, path string, body []byte, target any,
) error {
	response, err := client.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if failure := client.classifyStatus(response); failure != nil {
		return failure
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return wrapTransport(ReasonBackendUnavailable, err, client.secrets()...)
	}
	return nil
}

// Dial opens the lifecycle-multiplexed socket, minting a fresh ticket first.
// It returns the client instance identity the server assigned, which the attach
// runtime carries in every lifecycle correlation.
func (client *Client) Dial(ctx context.Context) (Socket, string, error) {
	if failure := client.gate(); failure != nil {
		return nil, "", failure
	}
	minted, err := client.mintTicket(ctx)
	if err != nil {
		return nil, "", err
	}
	endpoint, err := client.socketEndpoint(minted.Ticket)
	if err != nil {
		return nil, "", err
	}
	socket, err := client.dialer(ctx, endpoint)
	if err != nil {
		// The endpoint carries the ticket, and dial errors embed the endpoint.
		// Redacting with the ticket as a secret is not enough on its own — the
		// URL itself is stripped — but both are passed so a partial match
		// cannot survive either.
		return nil, "", wrapTransport(ReasonBackendUnavailable, err,
			append(client.secrets(), minted.Ticket)...)
	}
	return socket, minted.ClientInstanceID, nil
}

// NewWebSocketDialer builds the production Dialer over a specific HTTP client,
// so the socket and the REST calls can share one transport policy — trust
// store, proxy and timeouts included.
//
// Without this the two halves disagree. New takes a Doer for REST but installed
// a dialer that always used http.DefaultClient, so a server reachable over REST
// through a private certificate authority or a proxy would admit successfully
// and then fail to attach, with nothing in the failure naming the asymmetry.
//
// A nil client means http.DefaultClient, which is the behaviour New still
// installs when no dialer is supplied.
func NewWebSocketDialer(client *http.Client) Dialer {
	return func(ctx context.Context, endpoint string) (Socket, error) {
		var options *websocket.DialOptions
		if client != nil {
			options = &websocket.DialOptions{HTTPClient: client}
		}
		connection, _, err := websocket.Dial(ctx, endpoint, options)
		if err != nil {
			return nil, err
		}
		// The lifecycle socket carries terminal snapshots, which are far larger
		// than the library's 32KiB default.
		connection.SetReadLimit(8 << 20)
		return &websocketSocket{conn: connection}, nil
	}
}

// dialWebSocket is the production Dialer over the default HTTP client.
func dialWebSocket(ctx context.Context, endpoint string) (Socket, error) {
	return NewWebSocketDialer(nil)(ctx, endpoint)
}

// websocketSocket adapts coder/websocket to Socket and remembers what the peer
// said on the way out.
type websocketSocket struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	reason CloseReason
}

func (socket *websocketSocket) Read(ctx context.Context) ([]byte, error) {
	_, data, err := socket.conn.Read(ctx)
	if err != nil {
		socket.recordCloseReason(err)
		return nil, err
	}
	return data, nil
}

func (socket *websocketSocket) Write(ctx context.Context, frame []byte) error {
	return socket.conn.Write(ctx, websocket.MessageText, frame)
}

func (socket *websocketSocket) Close() error {
	return socket.conn.Close(websocket.StatusNormalClosure, "")
}

func (socket *websocketSocket) CloseReason() CloseReason {
	socket.mu.Lock()
	defer socket.mu.Unlock()
	if socket.reason == "" {
		return ReasonUnattributed
	}
	return socket.reason
}

// recordCloseReason reads the close frame's reason slot. Anything outside the
// published vocabulary — including an abrupt close, which carries no frame at
// all — stays unattributed rather than being coerced into a named cause.
func (socket *websocketSocket) recordCloseReason(err error) {
	var status websocket.CloseError
	if !errors.As(err, &status) {
		return
	}
	reason := CloseReason(status.Reason)
	if !reason.IsPublished() {
		return
	}
	socket.mu.Lock()
	socket.reason = reason
	socket.mu.Unlock()
}
