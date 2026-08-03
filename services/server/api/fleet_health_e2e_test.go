//go:build e2e

package api

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/internal/harnesspolicy/testsupport"
)

// TestFakeVsRealFleetHealthProbeEvidence compares scripted fake gateway probe
// evidence with a real server binary for the full shared script catalog:
// happy, protocol_mismatch, empty_ticket, daemon_disconnect, and recovery.
//
// Real-binary authority for divergence steps uses AG_E2E_FLEET_HEALTH_FAULT so
// production handlers/state transitions emit the scripted surface (not a
// reverse-proxy rewrite). Happy/recovery run without the fault env.
// shared_input: fleetHealthProbeScript
// real_marker: AG_E2E_SERVER_BIN
func TestFakeVsRealFleetHealthProbeEvidence(t *testing.T) {
	// fleetHealthProbeScript is the shared scripted input for fake and real.
	if testsupport.FleetHealthProbeScriptName != "fleetHealthProbeScript" {
		t.Fatalf("shared script name = %q", testsupport.FleetHealthProbeScriptName)
	}
	if len(testsupport.FleetHealthProbeScript) == 0 {
		t.Fatal("fleetHealthProbeScript catalog is empty")
	}

	bin := os.Getenv("AG_E2E_SERVER_BIN")
	if bin == "" {
		t.Log("AG_E2E_SERVER_BIN is not set; real fleet-health comparison unexecuted")
	} else if _, err := os.Stat(bin); err != nil {
		t.Fatalf("AG_E2E_SERVER_BIN=%q is not usable: %v", bin, err)
	}

	for _, step := range testsupport.FleetHealthProbeScript {
		step := step
		t.Run(step.Name, func(t *testing.T) {
			fake := testsupport.NewFleetHealthGatewayFakeForStep(step)
			t.Cleanup(fake.Close)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			fakeEvidence, fakeErr := testsupport.ProbeFleetHealthGateway(ctx, fake.URL)
			assertScriptedProbe(t, step, fakeEvidence, fakeErr, "fake")

			if bin == "" {
				return
			}

			// Each step starts a real process that enters production handlers.
			// Divergence steps set AG_E2E_FLEET_HEALTH_FAULT to the step name so
			// the real binary owns the failure surface (no proxy synthesis).
			realBase := startRealServerForFleetHealth(t, bin, realFaultForStep(step.Name))
			realCtx, realCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer realCancel()
			realEvidence, realErr := testsupport.ProbeFleetHealthGateway(realCtx, realBase)
			assertScriptedProbe(t, step, realEvidence, realErr, "real")

			// Error class and phase-specific structured evidence must match.
			// Swapping empty_ticket for a later hello disconnect must fail.
			if (fakeErr == nil) != (realErr == nil) {
				t.Fatalf("error class diverge: fake err=%v real err=%v; fix fake rather than weaken",
					fakeErr, realErr)
			}
			fakeCmp := testsupport.ComparableProbeEvidence(fakeEvidence)
			realCmp := testsupport.ComparableProbeEvidence(realEvidence)
			if fakeCmp != realCmp {
				t.Fatalf("fake evidence %+v != real evidence %+v; fix fake rather than weaken assertion",
					fakeCmp, realCmp)
			}
		})
	}
}

func realFaultForStep(name string) string {
	switch name {
	case "protocol_mismatch", "empty_ticket", "daemon_disconnect":
		return name
	default:
		return ""
	}
}

func assertScriptedProbe(
	t *testing.T,
	step testsupport.FleetHealthProbeStep,
	evidence testsupport.FleetHealthProbeEvidence,
	err error,
	label string,
) {
	t.Helper()
	expected := testsupport.ExpectedEvidenceForStep(step)
	switch step.Name {
	case "protocol_mismatch":
		// Probe still returns capabilities version; mismatch is host-classified.
		if evidence.ProtocolVersion != expected.ProtocolVersion {
			t.Fatalf("%s protocol_mismatch version = %q want %q", label, evidence.ProtocolVersion, expected.ProtocolVersion)
		}
		if err != nil {
			t.Fatalf("%s protocol_mismatch must complete ticket/hello for host classification: %v", label, err)
		}
		if !evidence.TicketNonEmpty || evidence.HelloKind != "h" {
			t.Fatalf("%s protocol_mismatch incomplete evidence %+v", label, evidence)
		}
		if evidence.FailedPhase != "" || evidence.FailureClass != "" {
			t.Fatalf("%s protocol_mismatch must not mark failure phase: %+v", label, evidence)
		}
	case "empty_ticket":
		if err == nil {
			t.Fatalf("%s empty_ticket must fail probe", label)
		}
		// Phase-specific structured evidence: ticket phase, not hello.
		if evidence.FailedPhase != "ticket" || evidence.FailureClass != "empty_ticket" {
			t.Fatalf("%s empty_ticket evidence phase/class = %q/%q want ticket/empty_ticket (got %+v)",
				label, evidence.FailedPhase, evidence.FailureClass, evidence)
		}
		if evidence.ProtocolVersion != expected.ProtocolVersion {
			t.Fatalf("%s empty_ticket protocol = %q want %q", label, evidence.ProtocolVersion, expected.ProtocolVersion)
		}
		if evidence.TicketNonEmpty {
			t.Fatalf("%s empty_ticket must leave TicketNonEmpty=false", label)
		}
		if evidence.HelloKind != "" {
			t.Fatalf("%s empty_ticket must not reach hello (kind=%q)", label, evidence.HelloKind)
		}
	case "daemon_disconnect":
		if err == nil {
			t.Fatalf("%s daemon_disconnect must fail hello", label)
		}
		// Phase-specific structured evidence: hello phase after a non-empty ticket.
		if evidence.FailedPhase != "hello" {
			t.Fatalf("%s daemon_disconnect FailedPhase = %q want hello (got %+v)",
				label, evidence.FailedPhase, evidence)
		}
		if evidence.FailureClass != "hello_timeout" && evidence.FailureClass != "hello_read" {
			t.Fatalf("%s daemon_disconnect FailureClass = %q want hello_timeout|hello_read",
				label, evidence.FailureClass)
		}
		if evidence.ProtocolVersion != expected.ProtocolVersion {
			t.Fatalf("%s daemon_disconnect protocol = %q want %q", label, evidence.ProtocolVersion, expected.ProtocolVersion)
		}
		if !evidence.TicketNonEmpty {
			t.Fatalf("%s daemon_disconnect must mint a non-empty ticket before hello fails", label)
		}
		if evidence.HelloKind != "" {
			t.Fatalf("%s daemon_disconnect must not accept hello (kind=%q)", label, evidence.HelloKind)
		}
	default:
		if err != nil {
			t.Fatalf("%s probe %s: %v", label, step.Name, err)
		}
		if evidence.ProtocolVersion != expected.ProtocolVersion ||
			evidence.TicketNonEmpty != expected.TicketNonEmpty ||
			evidence.HelloKind != expected.HelloKind ||
			evidence.HelloHasSessions != expected.HelloHasSessions {
			t.Fatalf("%s evidence %+v != scripted expected %+v", label, evidence, expected)
		}
		if evidence.FailedPhase != "" || evidence.FailureClass != "" {
			t.Fatalf("%s success path must not set failure evidence: %+v", label, evidence)
		}
	}
}

// startRealServerForFleetHealth launches the real server binary. fault is the
// optional AG_E2E_FLEET_HEALTH_FAULT value that enters production handlers.
func startRealServerForFleetHealth(t *testing.T, bin, fault string) string {
	t.Helper()
	dataDir := t.TempDir()
	homeDir := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	cmd := exec.Command(bin, //nolint:gosec // test-controlled binary path from AG_E2E_SERVER_BIN
		"-data-dir", dataDir,
		"-insecure",
		"-no-auth",
		"-addr", addr,
	)
	env := append(os.Environ(), "HOME="+homeDir)
	// Clear any inherited fault so happy/recovery are clean, then set only when needed.
	env = append(env, fleetHealthFaultEnv+"=")
	if fault != "" {
		env = append(env, fleetHealthFaultEnv+"="+fault)
	}
	cmd.Env = env
	logPath := filepath.Join(dataDir, "server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = logFile.Close()
	})

	deadline := time.Now().Add(8 * time.Second)
	url := "http://" + addr
	// Capabilities-only readiness: faulted processes must not block on full probe.
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/capabilities", nil)
		if reqErr == nil {
			resp, doErr := http.DefaultClient.Do(req)
			if doErr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					cancel()
					return url
				}
			}
		}
		cancel()
		time.Sleep(50 * time.Millisecond)
	}
	body, _ := os.ReadFile(logPath)
	t.Fatalf("real server never answered capabilities (fault=%q); log:\n%s", fault, body)
	return ""
}
