// Package testsupport hosts deterministic harness fakes that are production
// source (not *_test.go) but excluded from the external-boundary inventory by
// path convention (/testsupport/).
package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// FleetHealthProbeEvidence is the credential-free probe evidence hosts rely on
// for capabilities → ticket → hello admission.
type FleetHealthProbeEvidence struct {
	ProtocolVersion  string
	TicketNonEmpty   bool
	HelloKind        string
	HelloHasSessions bool
	// Scenario labels divergence surfaces that FakeVsReal must keep aligned.
	Scenario string
	// FailedPhase is the first phase that rejected the probe:
	// "" (success), "capabilities", "ticket", or "hello".
	// Distinguishes empty_ticket (ticket) from daemon_disconnect (hello).
	FailedPhase string
	// FailureClass is a stable token for phase-specific structured evidence
	// (empty_ticket, hello_timeout, hello_read, …). Empty on success.
	FailureClass string
}

// FleetHealthProbeStep is one scripted phase outcome used by the fake gateway
// and by FakeVsRealFleetHealth* to drive comparable evidence.
type FleetHealthProbeStep struct {
	// Name is a stable scenario id (happy, protocol_mismatch, empty_ticket,
	// daemon_disconnect, recovery).
	Name string
	// ProtocolVersion returned by /api/capabilities (empty → omit field).
	ProtocolVersion string
	// CapabilitiesStatus overrides HTTP status for capabilities (0 → 200).
	CapabilitiesStatus int
	// Ticket is the ws-ticket body. Empty string with TicketStatus 200 yields empty_ticket.
	Ticket string
	// TicketStatus overrides HTTP status for ticket (0 → 200).
	TicketStatus int
	// HelloKind is the first WS frame kind ("h" or "v"). Empty means no hello
	// (daemon disconnect / silence).
	HelloKind string
	// HelloSessions, when non-nil, is serialized as the sessions array (may be empty).
	HelloSessions []any
	// CloseAfterHello closes the socket immediately after writing hello.
	CloseAfterHello bool
}

// FleetHealthProbeScriptName is the registry/shared_input marker retained for
// harnesspolicy inventory matching (docs/adr test-harness-dependency-strategies).
const FleetHealthProbeScriptName = "fleetHealthProbeScript"

// FleetHealthProbeScript is the shared scripted evidence catalog that drives
// both the fake gateway and FakeVsRealFleetHealth fidelity. It is not a marker
// string — each step is executable input that shapes probe evidence.
// Alias note: tests and FakeVsReal still refer to fleetHealthProbeScript by name.
var FleetHealthProbeScript = []FleetHealthProbeStep{
	{
		Name:            "happy",
		ProtocolVersion: "protocol-3",
		Ticket:          "fleet-health-scripted-ticket",
		HelloKind:       "h",
		HelloSessions:   []any{},
	},
	{
		// Host-classified mismatch: capabilities report an older protocol;
		// ticket/hello remain reachable so real-binary and fake share evidence.
		Name:            "protocol_mismatch",
		ProtocolVersion: "protocol-1",
		Ticket:          "unused",
		HelloKind:       "h",
		HelloSessions:   []any{},
	},
	{
		Name:            "empty_ticket",
		ProtocolVersion: "protocol-3",
		Ticket:          "",
		HelloKind:       "",
	},
	{
		Name:            "daemon_disconnect",
		ProtocolVersion: "protocol-3",
		Ticket:          "fleet-health-disconnect-ticket",
		HelloKind:       "", // open then silence / no hello
		CloseAfterHello: false,
	},
	{
		Name:            "recovery",
		ProtocolVersion: "protocol-3",
		Ticket:          "fleet-health-recovery-ticket",
		HelloKind:       "h",
		HelloSessions:   []any{},
	},
}

// ExpectedEvidenceForStep derives the comparable evidence a correct probe of
// the named script step must produce (or the error class for mismatch paths).
func ExpectedEvidenceForStep(step FleetHealthProbeStep) FleetHealthProbeEvidence {
	ev := FleetHealthProbeEvidence{
		ProtocolVersion: step.ProtocolVersion,
		Scenario:        step.Name,
	}
	if step.Ticket != "" && (step.TicketStatus == 0 || step.TicketStatus == http.StatusOK) {
		ev.TicketNonEmpty = true
	}
	if step.HelloKind != "" {
		ev.HelloKind = step.HelloKind
		ev.HelloHasSessions = step.HelloSessions != nil
	}
	return ev
}

// FleetHealthScriptByName returns a script step by name.
func FleetHealthScriptByName(name string) (FleetHealthProbeStep, bool) {
	for _, step := range FleetHealthProbeScript {
		if step.Name == name {
			return step, true
		}
	}
	return FleetHealthProbeStep{}, false
}

// FleetHealthGatewayFake serves a deterministic gateway surface for fleet health probes.
type FleetHealthGatewayFake struct {
	URL      string
	server   *httptest.Server
	mu       sync.Mutex
	step     FleetHealthProbeStep
	requests atomic.Int64
}

// NewFleetHealthGatewayFake starts an httptest gateway that answers the three
// fleet-health probe phases with the happy-path scripted evidence.
func NewFleetHealthGatewayFake() *FleetHealthGatewayFake {
	return NewFleetHealthGatewayFakeForStep(mustHappyStep())
}

// NewFleetHealthGatewayFakeForStep starts a fake gateway driven by one script step.
func NewFleetHealthGatewayFakeForStep(step FleetHealthProbeStep) *FleetHealthGatewayFake {
	fake := &FleetHealthGatewayFake{step: step}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		fake.requests.Add(1)
		fake.mu.Lock()
		current := fake.step
		fake.mu.Unlock()
		status := current.CapabilitiesStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			return
		}
		body := map[string]any{"capabilities": []string{"fleet.health"}}
		if current.ProtocolVersion != "" {
			body["protocolVersion"] = current.ProtocolVersion
		}
		_ = json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		fake.requests.Add(1)
		fake.mu.Lock()
		current := fake.step
		fake.mu.Unlock()
		status := current.TicketStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"ticket":             current.Ticket,
			"client_instance_id": "ci-fleet-health",
		})
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		fake.requests.Add(1)
		fake.mu.Lock()
		current := fake.step
		fake.mu.Unlock()
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.CloseNow() }()
		if current.HelloKind == "" {
			// daemon_disconnect: accept then stay silent until the probe cancels
			// (hello timeout). Do not close early — early close yields hello_read
			// while the real AG_E2E_FLEET_HEALTH_FAULT seam yields hello_timeout.
			<-r.Context().Done()
			return
		}
		payload, _ := json.Marshal(map[string]any{
			"k":                current.HelloKind,
			"sessions":         current.HelloSessions,
			"protocolVersion":  current.ProtocolVersion,
			"clientInstanceId": "ci-fleet-health",
		})
		_ = connection.Write(r.Context(), websocket.MessageText, payload)
		if current.CloseAfterHello {
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(50 * time.Millisecond):
		}
	})
	server := httptest.NewServer(mux)
	fake.URL = server.URL
	fake.server = server
	return fake
}

func mustHappyStep() FleetHealthProbeStep {
	step, ok := FleetHealthScriptByName("happy")
	if !ok {
		panic("fleet health script missing happy step")
	}
	return step
}

// SetStep switches the scripted evidence for subsequent probes (recovery path).
func (fake *FleetHealthGatewayFake) SetStep(step FleetHealthProbeStep) {
	fake.mu.Lock()
	fake.step = step
	fake.mu.Unlock()
}

// Close shuts down the fake gateway.
func (fake *FleetHealthGatewayFake) Close() {
	if fake != nil && fake.server != nil {
		fake.server.Close()
	}
}

// ProbeFleetHealthGateway runs the host-equivalent capabilities → ticket → hello
// sequence and returns comparable evidence. It never returns credential material
// in the evidence struct. On failure it still returns partial phase evidence so
// empty_ticket and daemon_disconnect remain distinguishable.
func ProbeFleetHealthGateway(ctx context.Context, baseURL string) (FleetHealthProbeEvidence, error) {
	var evidence FleetHealthProbeEvidence
	baseURL = strings.TrimRight(baseURL, "/")

	if err := probeCapabilitiesPhase(ctx, baseURL, &evidence); err != nil {
		return evidence, err
	}
	ticket, ticketVersion, err := probeTicketPhase(ctx, baseURL, &evidence)
	if err != nil {
		return evidence, err
	}
	if err := probeHelloPhase(ctx, baseURL, ticket, ticketVersion, &evidence); err != nil {
		return evidence, err
	}
	return evidence, nil
}

func probeCapabilitiesPhase(ctx context.Context, baseURL string, evidence *FleetHealthProbeEvidence) error {
	capReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/capabilities", nil)
	if err != nil {
		evidence.FailedPhase = "capabilities"
		evidence.FailureClass = "capabilities_request"
		return fmt.Errorf("capabilities request: %w", err)
	}
	capResp, err := http.DefaultClient.Do(capReq)
	if err != nil {
		evidence.FailedPhase = "capabilities"
		evidence.FailureClass = "capabilities_dial"
		return fmt.Errorf("capabilities dial: %w", err)
	}
	defer capResp.Body.Close()
	if capResp.StatusCode != http.StatusOK {
		evidence.FailedPhase = "capabilities"
		evidence.FailureClass = "capabilities_status"
		return fmt.Errorf("capabilities status %d", capResp.StatusCode)
	}
	var capBody struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.NewDecoder(capResp.Body).Decode(&capBody); err != nil {
		evidence.FailedPhase = "capabilities"
		evidence.FailureClass = "capabilities_decode"
		return fmt.Errorf("capabilities decode: %w", err)
	}
	evidence.ProtocolVersion = capBody.ProtocolVersion
	return nil
}

// probeTicketPhase mints a ticket. For host-classified protocol_mismatch the
// capabilities version is preserved in evidence while the operational header
// uses the live protocol-3 contract so real binaries mint without a proxy.
func probeTicketPhase(ctx context.Context, baseURL string, evidence *FleetHealthProbeEvidence) (ticket, ticketVersion string, err error) {
	ticketReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/ws-ticket", nil)
	if err != nil {
		evidence.FailedPhase = "ticket"
		evidence.FailureClass = "ticket_request"
		return "", "", fmt.Errorf("ticket request: %w", err)
	}
	ticketVersion = evidence.ProtocolVersion
	if ticketVersion != "" && ticketVersion != "protocol-3" {
		ticketVersion = "protocol-3"
	}
	ticketReq.Header.Set("X-Agent-Grid-Protocol-Version", ticketVersion)
	ticketResp, err := http.DefaultClient.Do(ticketReq)
	if err != nil {
		evidence.FailedPhase = "ticket"
		evidence.FailureClass = "ticket_dial"
		return "", "", fmt.Errorf("ticket dial: %w", err)
	}
	defer ticketResp.Body.Close()
	if ticketResp.StatusCode != http.StatusOK {
		evidence.FailedPhase = "ticket"
		evidence.FailureClass = "ticket_status"
		return "", "", fmt.Errorf("ticket status %d", ticketResp.StatusCode)
	}
	var ticketBody struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(ticketResp.Body).Decode(&ticketBody); err != nil {
		evidence.FailedPhase = "ticket"
		evidence.FailureClass = "ticket_decode"
		return "", "", fmt.Errorf("ticket decode: %w", err)
	}
	evidence.TicketNonEmpty = ticketBody.Ticket != ""
	if ticketBody.Ticket == "" {
		evidence.FailedPhase = "ticket"
		evidence.FailureClass = "empty_ticket"
		return "", "", fmt.Errorf("empty websocket ticket")
	}
	return ticketBody.Ticket, ticketVersion, nil
}

func probeHelloPhase(ctx context.Context, baseURL, ticket, ticketVersion string, evidence *FleetHealthProbeEvidence) error {
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws?ticket=" + ticket +
		"&protocolVersion=" + ticketVersion
	connection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		evidence.FailedPhase = "hello"
		evidence.FailureClass = "websocket_dial"
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer func() { _ = connection.CloseNow() }()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(ctx, time.Until(deadline))
		_, data, readErr := connection.Read(readCtx)
		cancel()
		if readErr != nil {
			evidence.FailedPhase = "hello"
			evidence.FailureClass = classifyHelloReadFailure(ctx, readErr)
			return fmt.Errorf("hello read: %w", readErr)
		}
		var frame struct {
			Kind     string          `json:"k"`
			Sessions json.RawMessage `json:"sessions"`
		}
		if json.Unmarshal(data, &frame) != nil {
			continue
		}
		if frame.Kind == "h" || frame.Kind == "v" {
			evidence.HelloKind = frame.Kind
			evidence.HelloHasSessions = frame.Sessions != nil
			return nil
		}
	}
	evidence.FailedPhase = "hello"
	evidence.FailureClass = "hello_timeout"
	return fmt.Errorf("hello timed out")
}

func classifyHelloReadFailure(ctx context.Context, readErr error) string {
	if ctx.Err() != nil {
		return "hello_timeout"
	}
	msg := readErr.Error()
	if strings.Contains(msg, "deadline") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "context deadline") {
		return "hello_timeout"
	}
	return "hello_read"
}

// ComparableProbeEvidence returns the phase-discriminating fields FakeVsReal
// must keep aligned (including failure phase/class). Scenario is excluded.
func ComparableProbeEvidence(ev FleetHealthProbeEvidence) FleetHealthProbeEvidence {
	return FleetHealthProbeEvidence{
		ProtocolVersion:  ev.ProtocolVersion,
		TicketNonEmpty:   ev.TicketNonEmpty,
		HelloKind:        ev.HelloKind,
		HelloHasSessions: ev.HelloHasSessions,
		FailedPhase:      ev.FailedPhase,
		FailureClass:     ev.FailureClass,
	}
}
