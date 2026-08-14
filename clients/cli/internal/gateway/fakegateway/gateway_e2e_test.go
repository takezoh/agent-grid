//go:build e2e

package fakegateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// gateway_e2e_test.go is the fidelity backstop for the gateway boundary.
//
// It runs the same probe sequence against the fake and against a real server
// binary and requires the two to agree on the shapes the CLI reads. If it
// fails, the fake is wrong — never the assertion.
//
// It skips when AG_E2E_SERVER_BIN is unset, which is the expected state on a
// pull-request runner: this is a T3 check that needs a real backend.

// probeEvidence is what one probe observed, reduced to the facts the CLI
// depends on. Comparing a reduction rather than raw bytes is deliberate: the
// session list differs between a fake and a fresh real server, and requiring
// them to match would be requiring the wrong thing.
type probeEvidence struct {
	Admitted        bool
	HasInstance     bool
	FirstFrameKind  string
	ProtocolVersion string
	FirstFrameKeys  []string
	AdmitErr        string
	DialErr         string
	ReadErr         string
}

// probeSequence is the shared input: the admission and hello sequence the CLI
// performs against every server, written once so the fake and the real binary
// are driven by identical code.
func probeSequence(t *testing.T, doer gateway.Doer, dialer gateway.Dialer, base string) probeEvidence {
	t.Helper()
	entry := clientconfig.Server{
		Index: 0, ID: "srv_e2e", DisplayName: "e2e", Enabled: true,
		URL: base, Auth: clientconfig.Auth{Mode: "none"},
	}
	client, err := gateway.New(entry, doer, dialer)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	var evidence probeEvidence
	if err := client.Admit(context.Background()); err != nil {
		evidence.AdmitErr = err.Error()
		return evidence
	}
	evidence.Admitted = true

	socket, instance, err := client.Dial(context.Background())
	if err != nil {
		evidence.DialErr = err.Error()
		return evidence
	}
	defer func() { _ = socket.Close() }()
	evidence.HasInstance = instance != ""

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := socket.Read(ctx)
	if err != nil {
		evidence.ReadErr = err.Error()
		return evidence
	}
	frame, err := gateway.DecodeFrame(raw)
	if err != nil {
		evidence.ReadErr = err.Error()
		return evidence
	}
	evidence.FirstFrameKind = fmt.Sprintf("%T", frame)
	if hello, ok := frame.(gateway.HelloFrame); ok {
		evidence.ProtocolVersion = hello.ProtocolVersion
	}

	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		for key := range object {
			evidence.FirstFrameKeys = append(evidence.FirstFrameKeys, key)
		}
		sort.Strings(evidence.FirstFrameKeys)
	}
	return evidence
}

// TestE2E_FakeVsRealGatewayAdmission requires the fake and a real server to
// agree on the admission and hello sequence the CLI depends on.
func TestE2E_FakeVsRealGatewayAdmission(t *testing.T) {
	binary := os.Getenv("AG_E2E_SERVER_BIN")
	if binary == "" {
		t.Skip("AG_E2E_SERVER_BIN is unset; the gateway fidelity backstop needs a real server")
	}

	fake := New(gateway.SessionInfo{ID: "s1"})
	fromFake := probeSequence(t, fake.Doer(), fake.Dialer(), "https://gateway.test")

	base, client := startRealServer(t, binary)
	fromReal := probeSequence(t, client, gateway.NewWebSocketDialer(client), base)

	if fromReal.AdmitErr != "" || fromReal.DialErr != "" || fromReal.ReadErr != "" {
		t.Fatalf("the real server could not be probed: %+v", fromReal)
	}
	if fromFake.Admitted != fromReal.Admitted {
		t.Errorf("admission differs: fake %v, real %v", fromFake.Admitted, fromReal.Admitted)
	}
	if fromFake.HasInstance != fromReal.HasInstance {
		t.Errorf("client instance identity differs: fake %v, real %v",
			fromFake.HasInstance, fromReal.HasInstance)
	}
	if fromFake.FirstFrameKind != fromReal.FirstFrameKind {
		t.Errorf("first frame differs: fake %s, real %s",
			fromFake.FirstFrameKind, fromReal.FirstFrameKind)
	}
	if fromFake.ProtocolVersion != fromReal.ProtocolVersion {
		t.Errorf("protocol version differs: fake %q, real %q",
			fromFake.ProtocolVersion, fromReal.ProtocolVersion)
	}
	for _, key := range fromReal.FirstFrameKeys {
		if !slices.Contains(fromFake.FirstFrameKeys, key) {
			t.Errorf("the real hello frame carries %q and the fake does not; fix the fake", key)
		}
	}
}

// startRealServer launches the real backend on a loopback port with
// authentication disabled, and returns its base URL together with an HTTP
// client that trusts the certificate the server minted for itself.
//
// TLS is left on, which is the server's default. The `-insecure` flag would
// make this simpler and would test less: the CLI derives the socket scheme from
// the configured URL, so a plaintext run never exercises the https -> wss
// mapping every real deployment takes. An earlier version of this helper built
// an `http://` URL against that TLS listener and got a 400 back — which the CLI
// classified, correctly, as a bad request. The fake was never wrong; the
// harness was.
func startRealServer(t *testing.T, binary string) (string, *http.Client) {
	t.Helper()
	const address = "127.0.0.1:18873"
	command := exec.Command(binary, "-no-auth", "-addr", address)
	command.Env = append(os.Environ(), "AG_DATA_DIR="+t.TempDir())
	if err := command.Start(); err != nil {
		t.Fatalf("start the real server: %v", err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})

	// The server generates a self-signed certificate at startup and this test
	// is the process that started it, so there is no authority to check it
	// against. Confined to this loopback child process; the module proper never
	// relaxes verification.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert minted by the child process this test starts
		},
	}

	base := "https://" + address
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(base + "/api/capabilities")
		if err == nil {
			_ = response.Body.Close()
			return base, client
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the real server never became reachable")
	return "", nil
}
