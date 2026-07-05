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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

func realAppServerExtra(extra ...string) []string {
	args := []string{"-c", `sandbox_mode="danger-full-access"`}
	args = append(args, extra...)
	return args
}

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

// TestE2E_AppServerInit verifies the initialize handshake works against a
// real codex app-server. If this fails, either codex changed its
// initialize contract, or the argv from AppServerStdioArgs is stale.
func TestE2E_AppServerInit(t *testing.T) {
	bin := e2eCodexBin(t)
	home := clonedHomeWithCodex(t)
	sock := filepath.Join(t.TempDir(), "codex-init.sock")
	stopServer := startRealCodexListener(t, bin, home, sock, realAppServerExtra())
	defer stopServer()

	client, _, stopObserver := startObserverConn(t, sock)
	defer stopObserver()
	if client == nil {
		t.Fatal("observer conn is nil")
	}
}

// TestE2E_ThreadTurnLifecycle drives one full turn and asserts every event
// method the shim depends on appears in the notification stream.
func TestE2E_ThreadTurnLifecycle(t *testing.T) {
	bin := e2eCodexBin(t)
	scenario := runRealAppServerScenario(t, bin, realAppServerExtra(), "Say hi.")

	// Every one of these methods must appear before we declare the shim
	// contract intact.
	required := []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	}
	for _, m := range required {
		waitForRecordedMethod(t, scenario.rec, m, 5*time.Second)
	}
}

// TestE2E_FakeVsRealMethods runs the same scenario against both the fake and
// the real binary, and asserts the fake's method set is a subset of the real
// one. Extra methods only the real emits are logged, not failed — an
// intentional catchup task; extra methods only the fake emits fail immediately.
func TestE2E_FakeVsRealMethods(t *testing.T) {
	bin := e2eCodexBin(t)

	// Real side.
	realScenario := runRealAppServerScenario(t, bin, realAppServerExtra(), "Say hi.")
	realSet := toSet(realScenario.rec.snapshot())

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
	rec := newRealEventRecorder()
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
	if waitForRecordedMethodNoFail(rec, codexschema.MethodTurnCompleted, 3*time.Second) == nil {
		t.Fatalf("fake never emitted turn/completed; got %v", rec.snapshot())
	}
	return toSet(rec.snapshot())
}

func waitForRecordedMethodNoFail(rec *realEventRecorder, method string, timeout time.Duration) json.RawMessage {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if raw := rec.last(method); len(raw) > 0 {
			return raw
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
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
