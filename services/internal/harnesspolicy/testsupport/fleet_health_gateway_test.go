package testsupport

import (
	"context"
	"testing"
	"time"
)

// TestFleetHealthGatewayContract_ProbeEvidenceShape is the named invariant
// contract for the gateway fleet-health probe triple.
// invariant: capabilities ticket and hello preserve fleet health probe evidence
// assertion: assert fleet-health-probe-evidence
func TestFleetHealthGatewayContract_ProbeEvidenceShape(t *testing.T) {
	// fleetHealthProbeScript is the shared scripted input for fake and fidelity.
	if FleetHealthProbeScriptName != "fleetHealthProbeScript" || len(FleetHealthProbeScript) == 0 {
		t.Fatalf("shared script unavailable: name=%q steps=%d", FleetHealthProbeScriptName, len(FleetHealthProbeScript))
	}
	happy, ok := FleetHealthScriptByName("happy")
	if !ok {
		t.Fatal("script missing happy step")
	}
	fake := NewFleetHealthGatewayFakeForStep(happy)
	t.Cleanup(fake.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	evidence, err := ProbeFleetHealthGateway(ctx, fake.URL)
	if err != nil {
		t.Fatalf("probe fake gateway: %v", err)
	}
	// assert fleet-health-probe-evidence
	expected := ExpectedEvidenceForStep(happy)
	if evidence.ProtocolVersion != expected.ProtocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", evidence.ProtocolVersion, expected.ProtocolVersion)
	}
	if evidence.TicketNonEmpty != expected.TicketNonEmpty {
		t.Fatalf("ticketNonEmpty = %v, want %v", evidence.TicketNonEmpty, expected.TicketNonEmpty)
	}
	if evidence.HelloKind != expected.HelloKind {
		t.Fatalf("hello kind = %q, want %q", evidence.HelloKind, expected.HelloKind)
	}
	if evidence.HelloHasSessions != expected.HelloHasSessions {
		t.Fatalf("hello sessions = %v, want %v", evidence.HelloHasSessions, expected.HelloHasSessions)
	}
}

func TestFleetHealthGatewayContract_ScriptedDivergenceSurfaces(t *testing.T) {
	// Shared script must drive executable mismatch / disconnect / recovery
	// surfaces, not merely declare a marker string.
	if len(FleetHealthProbeScript) < 4 {
		t.Fatalf("script steps = %d, want full catalog", len(FleetHealthProbeScript))
	}

	// protocol_mismatch: capabilities report a different protocol.
	mismatch, ok := FleetHealthScriptByName("protocol_mismatch")
	if !ok {
		t.Fatal("missing protocol_mismatch")
	}
	fakeMismatch := NewFleetHealthGatewayFakeForStep(mismatch)
	t.Cleanup(fakeMismatch.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	mismatchEvidence, err := ProbeFleetHealthGateway(ctx, fakeMismatch.URL)
	// Probe still collects ticket/hello unless host stops early; protocol differs.
	if err == nil && mismatchEvidence.ProtocolVersion == "protocol-3" {
		t.Fatal("protocol_mismatch step must not report protocol-3")
	}
	if mismatchEvidence.ProtocolVersion != "protocol-1" {
		t.Fatalf("protocol_mismatch version = %q", mismatchEvidence.ProtocolVersion)
	}

	// empty_ticket: capabilities ok, ticket empty → probe error at ticket phase.
	emptyTicket, ok := FleetHealthScriptByName("empty_ticket")
	if !ok {
		t.Fatal("missing empty_ticket")
	}
	fakeEmpty := NewFleetHealthGatewayFakeForStep(emptyTicket)
	t.Cleanup(fakeEmpty.Close)
	emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer emptyCancel()
	emptyEvidence, err := ProbeFleetHealthGateway(emptyCtx, fakeEmpty.URL)
	if err == nil {
		t.Fatal("empty_ticket must fail probe")
	}
	if emptyEvidence.FailedPhase != "ticket" || emptyEvidence.FailureClass != "empty_ticket" {
		t.Fatalf("empty_ticket phase/class = %q/%q want ticket/empty_ticket",
			emptyEvidence.FailedPhase, emptyEvidence.FailureClass)
	}
	if emptyEvidence.TicketNonEmpty || emptyEvidence.HelloKind != "" {
		t.Fatalf("empty_ticket must stop before hello: %+v", emptyEvidence)
	}

	// daemon_disconnect: open WS without hello → hello timeout after non-empty ticket.
	disconnect, ok := FleetHealthScriptByName("daemon_disconnect")
	if !ok {
		t.Fatal("missing daemon_disconnect")
	}
	fakeDisconnect := NewFleetHealthGatewayFakeForStep(disconnect)
	t.Cleanup(fakeDisconnect.Close)
	discCtx, discCancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer discCancel()
	discEvidence, err := ProbeFleetHealthGateway(discCtx, fakeDisconnect.URL)
	if err == nil {
		t.Fatal("daemon_disconnect must fail hello")
	}
	if discEvidence.FailedPhase != "hello" {
		t.Fatalf("daemon_disconnect FailedPhase = %q want hello", discEvidence.FailedPhase)
	}
	if discEvidence.FailureClass != "hello_timeout" && discEvidence.FailureClass != "hello_read" {
		t.Fatalf("daemon_disconnect FailureClass = %q", discEvidence.FailureClass)
	}
	if !discEvidence.TicketNonEmpty || discEvidence.HelloKind != "" {
		t.Fatalf("daemon_disconnect must fail after ticket, before hello: %+v", discEvidence)
	}
	// Phase discrimination: empty_ticket and daemon_disconnect must not compare equal.
	if ComparableProbeEvidence(emptyEvidence) == ComparableProbeEvidence(discEvidence) {
		t.Fatalf("empty_ticket and daemon_disconnect evidence must differ: %+v", emptyEvidence)
	}

	// recovery: after disconnect, switch script to recovery and succeed.
	recovery, ok := FleetHealthScriptByName("recovery")
	if !ok {
		t.Fatal("missing recovery")
	}
	fakeDisconnect.SetStep(recovery)
	recCtx, recCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer recCancel()
	recovered, err := ProbeFleetHealthGateway(recCtx, fakeDisconnect.URL)
	if err != nil {
		t.Fatalf("recovery probe: %v", err)
	}
	expected := ExpectedEvidenceForStep(recovery)
	if recovered.ProtocolVersion != expected.ProtocolVersion ||
		!recovered.TicketNonEmpty ||
		recovered.HelloKind != expected.HelloKind ||
		!recovered.HelloHasSessions {
		t.Fatalf("recovery evidence incomplete: got %+v want %+v", recovered, expected)
	}
}
