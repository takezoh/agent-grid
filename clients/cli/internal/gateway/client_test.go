package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-protocol-admission, contract-attach-transport-topology
// (unit-transport-admission).

// fakeDoer answers requests from a table and counts every one it saw, so a test
// can assert that no request was made at all — which is the only way to check a
// gate rather than a convention.
type fakeDoer struct {
	requests atomic.Int64
	seen     []*http.Request
	respond  func(request *http.Request) (*http.Response, error)
}

func (doer *fakeDoer) Do(request *http.Request) (*http.Response, error) {
	doer.requests.Add(1)
	doer.seen = append(doer.seen, request)
	return doer.respond(request)
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

func usableServer() clientconfig.Server {
	return clientconfig.Server{
		Index:       0,
		ID:          "srv_test",
		DisplayName: "one",
		Enabled:     true,
		URL:         "https://gateway.test",
		Auth:        clientconfig.Auth{Mode: "bearer", Token: "s3cr3t-bearer-token"},
	}
}

// TestMismatchedServerPerformsNoOperationalWork is ver-mismatch-blocks-operational-work.
//
// The counterexample this rejects is the tempting one: notice the mismatch,
// report it, and carry on because the call site "knows" it already failed. Then
// one path forgets, and a build with a different wire contract starts issuing
// requests whose failures look like ordinary server errors. So the check is not
// "did Admit return an error" — it is "did anything reach the network
// afterwards", which is why the fake counts requests.
func TestMismatchedServerPerformsNoOperationalWork(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	doer.respond = func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/capabilities" {
			return jsonResponse(http.StatusOK, capabilitiesResponse{ProtocolVersion: "protocol-999"})
		}
		t.Errorf("an operational request reached the network after a mismatch: %s", request.URL.Path)
		return jsonResponse(http.StatusOK, map[string]any{})
	}
	client, err := New(usableServer(), doer, refusingDialer(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	admitErr := client.Admit(context.Background())
	if !errors.Is(admitErr, ErrIncompatibleProtocol) {
		t.Fatalf("Admit error = %v, want ErrIncompatibleProtocol", admitErr)
	}
	if client.Admitted() {
		t.Error("a mismatched server must never report itself admitted")
	}
	after := doer.requests.Load()

	for name, call := range map[string]func() error{
		"GetJSON":  func() error { return client.GetJSON(context.Background(), "/api/sessions", &struct{}{}) },
		"PostJSON": func() error { return client.PostJSON(context.Background(), "/api/sessions", map[string]any{}, nil) },
		"Delete":   func() error { return client.Delete(context.Background(), "/api/sessions/s1") },
		"Dial": func() error {
			_, _, dialErr := client.Dial(context.Background())
			return dialErr
		},
	} {
		callErr := call()
		if !errors.Is(callErr, ErrIncompatibleProtocol) {
			t.Errorf("%s after mismatch = %v, want ErrIncompatibleProtocol", name, callErr)
		}
	}
	if got := doer.requests.Load(); got != after {
		t.Errorf("%d request(s) reached the network after the mismatch; want 0", got-after)
	}
}

// TestMismatchOnAnOperationalRouteIsAlsoTerminal covers the server that is
// replaced while this process runs: the mismatch first shows up as a 426 on an
// ordinary route, not on discovery.
func TestMismatchOnAnOperationalRouteIsAlsoTerminal(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	doer.respond = func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/capabilities" {
			return jsonResponse(http.StatusOK, capabilitiesResponse{ProtocolVersion: ProtocolVersion})
		}
		return jsonResponse(http.StatusUpgradeRequired, map[string]string{
			"code": "incompatible_protocol", "expectedVersion": "protocol-4", "actualVersion": "protocol-4",
		})
	}
	client, err := New(usableServer(), doer, refusingDialer(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Admit(context.Background()); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if err := client.GetJSON(context.Background(), "/api/sessions", &struct{}{}); !errors.Is(err, ErrIncompatibleProtocol) {
		t.Fatalf("first operational call = %v, want ErrIncompatibleProtocol", err)
	}
	before := doer.requests.Load()
	if err := client.GetJSON(context.Background(), "/api/sessions", &struct{}{}); !errors.Is(err, ErrIncompatibleProtocol) {
		t.Fatalf("second operational call = %v, want ErrIncompatibleProtocol", err)
	}
	if got := doer.requests.Load(); got != before {
		t.Errorf("the second call reached the network; the mismatch must be remembered")
	}
}

// TestEveryOperationalRequestCarriesCredentialAndVersion is the positive half of
// admission: what a request must always carry.
func TestEveryOperationalRequestCarriesCredentialAndVersion(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	doer.respond = func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/capabilities" {
			return jsonResponse(http.StatusOK, capabilitiesResponse{ProtocolVersion: ProtocolVersion})
		}
		if request.URL.Path == "/api/ws-ticket" {
			return jsonResponse(http.StatusOK, ticketResponse{Ticket: "tkt", ClientInstanceID: "ci"})
		}
		return jsonResponse(http.StatusOK, map[string]any{})
	}
	server := usableServer()
	client, err := New(server, doer, func(context.Context, string) (Socket, error) {
		return nil, errors.New("dial refused")
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = client.Admit(context.Background())
	_ = client.GetJSON(context.Background(), "/api/sessions", &struct{}{})
	_ = client.PostJSON(context.Background(), "/api/sessions", map[string]any{}, nil)
	_ = client.Delete(context.Background(), "/api/sessions/s1")
	_, _, _ = client.Dial(context.Background())

	if len(doer.seen) == 0 {
		t.Fatal("no requests were made")
	}
	for _, request := range doer.seen {
		if got := request.Header.Get(ProtocolVersionHeader); got != ProtocolVersion {
			t.Errorf("%s %s carries version %q, want %q",
				request.Method, request.URL.Path, got, ProtocolVersion)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+server.Auth.Token {
			t.Errorf("%s %s carries authorization %q, want the bearer credential",
				request.Method, request.URL.Path, got)
		}
		if strings.Contains(request.URL.String(), server.Auth.Token) {
			t.Errorf("%s %s puts the bearer credential in the url", request.Method, request.URL.Path)
		}
	}
}

// TestSocketIsOpenedOnlyThroughAFreshlyMintedTicket pins the two-step flow and
// the rule that the bearer credential never reaches a URL.
func TestSocketIsOpenedOnlyThroughAFreshlyMintedTicket(t *testing.T) {
	t.Parallel()

	var minted atomic.Int64
	doer := &fakeDoer{}
	doer.respond = func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/capabilities":
			return jsonResponse(http.StatusOK, capabilitiesResponse{ProtocolVersion: ProtocolVersion})
		case "/api/ws-ticket":
			count := minted.Add(1)
			return jsonResponse(http.StatusOK, ticketResponse{
				Ticket:           "ticket-" + string(rune('0'+count)),
				ClientInstanceID: "ci",
			})
		}
		return jsonResponse(http.StatusOK, map[string]any{})
	}

	var endpoints []string
	server := usableServer()
	client, err := New(server, doer, func(_ context.Context, endpoint string) (Socket, error) {
		endpoints = append(endpoints, endpoint)
		return nil, errors.New("dial refused by the test")
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Admit(context.Background()); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	_, _, _ = client.Dial(context.Background())
	_, _, _ = client.Dial(context.Background())

	if minted.Load() != 2 {
		t.Errorf("%d tickets minted for 2 dials; each dial needs its own", minted.Load())
	}
	if len(endpoints) != 2 || endpoints[0] == endpoints[1] {
		t.Errorf("the two dials reused a ticket: %v", endpoints)
	}
	for _, endpoint := range endpoints {
		if strings.Contains(endpoint, server.Auth.Token) {
			t.Errorf("the bearer credential reached the socket url: %s", endpoint)
		}
	}
}

// TestConfigurationInvalidEntryHasNoTransportAtAll is the CLI half of
// contract-config-entry-admission: an unusable entry must never acquire a
// reachability observation, and the only way to guarantee that is for no
// transport to exist for it.
func TestConfigurationInvalidEntryHasNoTransportAtAll(t *testing.T) {
	t.Parallel()

	invalid := usableServer()
	invalid.Violation = &clientconfig.Violation{
		Rule: clientconfig.RuleDisplayNameCharset, Detail: "leading dash",
	}
	doer := &fakeDoer{respond: func(*http.Request) (*http.Response, error) {
		t.Error("an unusable entry was probed")
		return jsonResponse(http.StatusOK, map[string]any{})
	}}
	if _, err := New(invalid, doer, refusingDialer(t)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("New(unusable) = %v, want ErrInvalidRequest", err)
	}
	if doer.requests.Load() != 0 {
		t.Error("building a transport for an unusable entry touched the network")
	}
}

// TestAttachNeverUsesPerSessionRoute is ver-attach-topology-no-per-session-route.
//
// Two halves, because one alone is escapable. The behavioural half proves the
// endpoint this build produces carries no session parameter; the source half
// proves the route cannot be reintroduced somewhere the behavioural test does
// not reach.
func TestAttachNeverUsesPerSessionRoute(t *testing.T) {
	t.Parallel()

	client := &Client{server: usableServer()}
	endpoint, err := client.socketEndpoint("a-ticket")
	if err != nil {
		t.Fatalf("socketEndpoint: %v", err)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	if parsed.Path != "/ws" {
		t.Errorf("socket path = %q, want /ws", parsed.Path)
	}
	if parsed.Query().Has("session") {
		t.Errorf("the attach endpoint carries a session parameter: %s", endpoint)
	}
	if got := parsed.Query().Get("ticket"); got != "a-ticket" {
		t.Errorf("ticket = %q, want the minted value", got)
	}
	if got := parsed.Query().Get("protocolVersion"); got != ProtocolVersion {
		t.Errorf("protocolVersion = %q, want %q", got, ProtocolVersion)
	}

	for _, offending := range moduleSourcesContaining(t, `"session"`, "?session=") {
		t.Errorf("%s reintroduces the per-session attach route", offending)
	}
}

func refusingDialer(t *testing.T) Dialer {
	t.Helper()
	return func(context.Context, string) (Socket, error) {
		t.Error("a dial was attempted where none should be")
		return nil, errors.New("dial refused")
	}
}
