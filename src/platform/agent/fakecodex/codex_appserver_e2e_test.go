//go:build e2e

// Opt-in fidelity backstop for the fakecodex Server. Runs the same
// codexclient handshake and turn lifecycle production uses against a REAL
// `codex app-server` binary (stdio JSON-RPC v2), then asserts every method
// the fake advertises also appears in the real binary's output — and vice
// versa.
//
// The stream-backend e2e (adr-20260624-0002) validates the WS transport; this
// file validates the stdio transport, which orchestrator/agent uses directly.
//
// Skipped in normal builds by the `e2e` tag. Skipped at runtime unless
// REACTOR_E2E_CODEX_BIN points at an executable.

package fakecodex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
	codexcli "github.com/takezoh/agent-reactor/platform/lib/codex"
)

// e2eCodexBin returns the codex binary path, or skips the test.
func e2eCodexBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("REACTOR_E2E_CODEX_BIN")
	if bin == "" {
		t.Skip("REACTOR_E2E_CODEX_BIN is not set — skipping real-codex e2e")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("REACTOR_E2E_CODEX_BIN=%q is not executable: %v", bin, err)
	}
	return bin
}

// e2eRecorder records notification method names emitted over the transport.
type e2eRecorder struct {
	mu      sync.Mutex
	methods []string
}

func (r *e2eRecorder) OnNotification(method string, _ json.RawMessage) {
	r.mu.Lock()
	r.methods = append(r.methods, method)
	r.mu.Unlock()
}
func (r *e2eRecorder) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

func (r *e2eRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.methods))
	copy(out, r.methods)
	return out
}

// startRealCodex spawns `codex app-server` on stdio and returns a client Conn
// wired to its stdin/stdout, plus a cleanup func. Cleanup kills the process
// and waits for it to exit.
func startRealCodex(t *testing.T, bin string) (*codexclient.Conn, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	// AppServerStdioArgs is what the orchestrator would use in prod.
	args := codexcli.AppServerStdioArgs(nil, false)
	cmd := exec.CommandContext(ctx, bin, args[1:]...) // skip DriverName in argv[0]
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("stdout pipe: %v", err)
	}
	// Do not attach stderr — codex writes a lot; keep it out of the test log
	// unless the test explicitly fails.
	stderrBuf := &syncBuf{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start codex: %v", err)
	}

	client := codexclient.NewConn(codexclient.StdioTransport(stdout, stdin), 10*time.Second)
	cleanup := func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("codex stderr:\n%s", stderrBuf.String())
		}
	}
	return client, cleanup
}

// syncBuf is a tiny thread-safe bytes.Buffer replacement.
type syncBuf struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}
func (b *syncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// waitForMethod polls until method appears in rec, or times out.
func waitForMethod(t *testing.T, rec *e2eRecorder, method string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, m := range rec.snapshot() {
			if m == method {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestE2E_AppServerInit verifies the initialize handshake works against a
// real codex app-server. If this fails, either codex changed its
// initialize contract, or the argv from AppServerStdioArgs is stale.
func TestE2E_AppServerInit(t *testing.T) {
	bin := e2eCodexBin(t)
	client, cleanup := startRealCodex(t, bin)
	defer cleanup()

	rec := &e2eRecorder{}
	go func() { _ = client.Run(context.Background(), rec) }()

	if err := codexclient.Initialize(client); err != nil {
		var netErr *exec.ExitError
		if errors.As(err, &netErr) {
			t.Fatalf("codex initialize returned an ExitError (process died): %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
}

// TestE2E_ThreadTurnLifecycle drives one full turn and asserts every event
// method the shim depends on appears in the notification stream.
func TestE2E_ThreadTurnLifecycle(t *testing.T) {
	bin := e2eCodexBin(t)
	client, cleanup := startRealCodex(t, bin)
	defer cleanup()

	rec := &e2eRecorder{}
	go func() { _ = client.Run(context.Background(), rec) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	ts, err := codexclient.StartThread(client, "", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if ts.ThreadID == "" {
		t.Fatalf("StartThread returned empty ThreadID")
	}

	if err := codexclient.StartTurn(client, ts.ThreadID, "", []byte("Say hi."), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// Every one of these methods must appear before we declare the shim
	// contract intact.
	required := []string{
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	}
	for _, m := range required {
		if !waitForMethod(t, rec, m, 90*time.Second) {
			t.Fatalf("real codex never emitted %s; methods so far: %v", m, rec.snapshot())
		}
	}
}

// TestE2E_FakeVsRealMethods runs the same scenario against both the fake and
// the real binary, and asserts the fake's method set is a subset of the real
// one. Extra methods only the real emits are logged, not failed — an
// intentional catchup task; extra methods only the fake emits fail immediately.
func TestE2E_FakeVsRealMethods(t *testing.T) {
	bin := e2eCodexBin(t)

	// Real side.
	realClient, cleanup := startRealCodex(t, bin)
	defer cleanup()
	realRec := &e2eRecorder{}
	go func() { _ = realClient.Run(context.Background(), realRec) }()

	if err := codexclient.Initialize(realClient); err != nil {
		t.Fatalf("real Initialize: %v", err)
	}
	rts, err := codexclient.StartThread(realClient, "", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("real StartThread: %v", err)
	}
	if err := codexclient.StartTurn(realClient, rts.ThreadID, "", []byte("Say hi."), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("real StartTurn: %v", err)
	}
	// Wait for turn/completed so we know we've seen the full event set.
	if !waitForMethod(t, realRec, codexschema.MethodTurnCompleted, 90*time.Second) {
		t.Fatalf("real codex never emitted turn/completed")
	}
	realSet := toSet(realRec.snapshot())

	// Fake side.
	fake := New(Config{})
	fakeSet := runFakeScenario(t, fake)

	// Every fake method must exist in the real set.
	for m := range fakeSet {
		if !realSet[m] {
			t.Errorf("fakecodex emits %q but real codex did not; real set = %s", m, formatSet(realSet))
		}
	}

	// Log (not fail) methods only the real emitted — those are catchup work.
	var missingInFake []string
	for m := range realSet {
		if !fakeSet[m] {
			missingInFake = append(missingInFake, m)
		}
	}
	if len(missingInFake) > 0 {
		t.Logf("real codex methods not modeled by fakecodex (catchup candidates): %v", missingInFake)
	}
}

// runFakeScenario drives the fake through the same lifecycle as the real
// scenario and returns the emitted method set.
func runFakeScenario(t *testing.T, fake *Server) map[string]bool {
	t.Helper()

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := fake.Attach(ctx, pr2, pw1)
	defer stop()

	client := codexclient.NewConn(codexclient.StdioTransport(pr1, pw2), 5*time.Second)
	rec := &e2eRecorder{}
	go func() { _ = client.Run(context.Background(), rec) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("fake Initialize: %v", err)
	}
	ts, err := codexclient.StartThread(client, "", nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("fake StartThread: %v", err)
	}
	if err := codexclient.StartTurn(client, ts.ThreadID, "", []byte("hi"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("fake StartTurn: %v", err)
	}
	if !waitForMethod(t, rec, codexschema.MethodTurnCompleted, 3*time.Second) {
		t.Fatalf("fake never emitted turn/completed; got %v", rec.snapshot())
	}
	return toSet(rec.snapshot())
}

func toSet(items []string) map[string]bool {
	s := map[string]bool{}
	for _, i := range items {
		s[i] = true
	}
	return s
}

func formatSet(s map[string]bool) string {
	out := "{"
	first := true
	for k := range s {
		if !first {
			out += ", "
		}
		out += fmt.Sprintf("%q", k)
		first = false
	}
	return out + "}"
}
