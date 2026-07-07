//go:build e2e

package fakecodex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-grid/platform/agent/codexschema/v2"
	"github.com/takezoh/agent-grid/platform/e2etest"
	codexcli "github.com/takezoh/agent-grid/platform/lib/codex"
	"github.com/takezoh/agent-grid/platform/procgroup"
)

func startRealCodex(t *testing.T, bin string) (*codexclient.Conn, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	args := codexcli.AppServerStdioArgs(nil, false)
	cmd := procgroup.Command(procgroup.Spec{Ctx: ctx, Bin: bin, Args: args[1:]})
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

type realEventRecorder struct {
	mu      sync.Mutex
	methods []string
	params  map[string][]json.RawMessage
	events  []recordedNotification
}

type recordedNotification struct {
	Method string
	Params json.RawMessage
}

func clonedHomeWithCodex(t *testing.T) string {
	t.Helper()
	return e2etest.PrepareCodexHome(t, ".codex-e2e-home-")
}

func newRealEventRecorder() *realEventRecorder {
	return &realEventRecorder{params: map[string][]json.RawMessage{}}
}

func (r *realEventRecorder) OnNotification(method string, params json.RawMessage) {
	r.mu.Lock()
	r.methods = append(r.methods, method)
	raw := append(json.RawMessage(nil), params...)
	r.params[method] = append(r.params[method], raw)
	r.events = append(r.events, recordedNotification{Method: method, Params: raw})
	r.mu.Unlock()
}

func (r *realEventRecorder) OnServerRequest(_ codexclient.RequestID, method string, params json.RawMessage) {
	r.OnNotification("request:"+method, params)
}

func (r *realEventRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.methods))
	copy(out, r.methods)
	return out
}

func (r *realEventRecorder) last(method string) json.RawMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.params[method]
	if len(items) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), items[len(items)-1]...)
}

func (r *realEventRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.methods)
}

func (r *realEventRecorder) snapshotEvents() []recordedNotification {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedNotification, len(r.events))
	copy(out, r.events)
	return out
}

func waitForRecordedMethod(t *testing.T, rec *realEventRecorder, method string, timeout time.Duration) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if raw := rec.last(method); len(raw) > 0 {
			return raw
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("real codex never emitted %s; methods so far: %v", method, rec.snapshot())
	return nil
}

func waitForRecorderIdle(t *testing.T, rec *realEventRecorder, timeout, quiet time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	lastCount := -1
	lastChange := time.Now()
	for time.Now().Before(deadline) {
		count := rec.count()
		if count != lastCount {
			lastCount = count
			lastChange = time.Now()
		}
		if time.Since(lastChange) >= quiet {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("recorder never became idle within %s; methods=%v", timeout, rec.snapshot())
}

func startRealCodexListener(t *testing.T, bin, home, sock string, extra []string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	args := codexcli.AppServerListenArgs("codex", sock, extra, false)
	cmd := procgroup.Command(procgroup.Spec{Ctx: ctx, Bin: bin, Args: args[1:]})
	cmd.Env = append(os.Environ(), "HOME="+home)
	stderr := &syncBuf{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start codex app-server listen: %v", err)
	}
	e2etest.WaitForUnixSocketReady(t, sock, 10*time.Second)
	return func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("codex listen stderr:\n%s", stderr.String())
		}
	}
}

func startObserverConn(t *testing.T, sock string) (*codexclient.Conn, *realEventRecorder, func()) {
	t.Helper()
	tr, err := codexclient.DialUDS(sock, 10*time.Second)
	if err != nil {
		t.Fatalf("dial observer uds: %v", err)
	}
	conn := codexclient.NewConn(tr, 30*time.Second)
	rec := newRealEventRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = conn.Run(ctx, rec) }()
	if err := codexclient.Initialize(conn); err != nil {
		cancel()
		t.Fatalf("initialize observer conn: %v", err)
	}
	return conn, rec, cancel
}

type realCLIHandle struct {
	cmd   *exec.Cmd
	ptmx  *os.File
	lines []string
	mu    sync.Mutex
	wait  chan error
}

func spawnRealRemoteCLI(t *testing.T, bin, home, sock, cwd string, extra []string, prompt string) *realCLIHandle {
	t.Helper()
	args := []string{"--remote", "unix://" + sock, "--dangerously-bypass-approvals-and-sandbox", "--no-alt-screen"}
	if cwd != "" {
		args = append(args, "-C", cwd)
	}
	args = append(args, extra...)
	if strings.TrimSpace(prompt) != "" {
		args = append(args, prompt)
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start(codex remote): %v", err)
	}
	h := &realCLIHandle{cmd: cmd, ptmx: ptmx}
	h.wait = make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(ptmx)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			h.mu.Lock()
			h.lines = append(h.lines, scanner.Text())
			h.mu.Unlock()
		}
		_ = scanner.Err()
	}()
	go func() { h.wait <- cmd.Wait() }()
	t.Cleanup(h.Close)
	return h
}

func (h *realCLIHandle) Close() {
	if h.ptmx == nil {
		return
	}
	if h.cmd != nil && h.cmd.Process != nil {
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGKILL)
		_ = h.cmd.Process.Kill()
	}
	select {
	case <-h.wait:
	case <-time.After(2 * time.Second):
	}
	_ = h.ptmx.Close()
	h.ptmx = nil
}

func (h *realCLIHandle) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.lines))
	copy(out, h.lines)
	return out
}

func (h *realCLIHandle) lineCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.lines)
}

func (h *realCLIHandle) waitForIdle(t *testing.T, timeout, quiet time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	lastCount := -1
	lastChange := time.Now()
	for time.Now().Before(deadline) {
		count := h.lineCount()
		if count != lastCount {
			lastCount = count
			lastChange = time.Now()
		}
		if time.Since(lastChange) >= quiet {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("codex pty never became idle within %s; cli output=%s", timeout, formatCLIDebug(h.snapshot()))
}

func (h *realCLIHandle) SendPrompt(t *testing.T, prompt string) {
	t.Helper()
	h.waitForIdle(t, 30*time.Second, 1500*time.Millisecond)
	if _, err := h.ptmx.Write([]byte(prompt)); err != nil {
		t.Fatalf("write prompt to codex pty: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := h.ptmx.Write([]byte{'\r'}); err != nil {
		t.Fatalf("submit prompt to codex pty: %v", err)
	}
}

type realScenario struct {
	rec         *realEventRecorder
	threadID    string
	rolloutPath string
}

func runRealAppServerScenario(t *testing.T, bin string, appServerExtra []string, prompt string) realScenario {
	t.Helper()
	home := clonedHomeWithCodex(t)
	sock := filepath.Join(t.TempDir(), "codex-appserver.sock")
	stopServer := startRealCodexListener(t, bin, home, sock, appServerExtra)
	t.Cleanup(stopServer)
	initiator, initRec, stopInitiator := startObserverConn(t, sock)
	t.Cleanup(stopInitiator)
	_, observerRec, stopObserver := startObserverConn(t, sock)
	t.Cleanup(stopObserver)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	session, err := codexclient.StartThread(initiator, cwd, nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadRaw := waitForRecordedMethod(t, observerRec, codexschema.MethodThreadStarted, 45*time.Second)
	if threadRaw == nil {
		t.Fatal("thread/started missing")
	}
	waitForRecorderIdle(t, initRec, 30*time.Second, 1500*time.Millisecond)
	if strings.TrimSpace(prompt) != "" {
		if err := codexclient.StartTurn(initiator, session.ThreadID, cwd, []byte(prompt), codexclient.TurnOptions{}); err != nil {
			t.Fatalf("turn/start: %v", err)
		}
	}
	if turnStarted := waitForRecordedMethodNoFail(initRec, codexschema.MethodTurnStarted, 45*time.Second); turnStarted == nil {
		t.Fatalf("turn/started missing on initiator; initiator=%v observer=%v", initRec.snapshot(), observerRec.snapshot())
	}
	if turnCompleted := waitForRecordedMethodNoFail(initRec, codexschema.MethodTurnCompleted, 3*time.Minute); turnCompleted == nil {
		t.Fatalf("turn/completed missing on initiator; initiator=%v observer=%v", initRec.snapshot(), observerRec.snapshot())
	}

	thread, err := codexschemav2.UnmarshalThreadStartedNotification(threadRaw)
	if err != nil {
		t.Fatalf("decode thread/started payload: %v", err)
	}
	if thread.Thread.ID == "" {
		t.Fatalf("thread/started missing thread id: %s", string(threadRaw))
	}
	if thread.Thread.Path == nil || strings.TrimSpace(*thread.Thread.Path) == "" {
		t.Fatalf("thread/started missing rollout path: %s", string(threadRaw))
	}
	return realScenario{
		rec:         initRec,
		threadID:    thread.Thread.ID,
		rolloutPath: strings.TrimSpace(*thread.Thread.Path),
	}
}

func formatCLIDebug(lines []string) string {
	if len(lines) == 0 {
		return "<empty>"
	}
	if len(lines) > 80 {
		lines = lines[len(lines)-80:]
	}
	return fmt.Sprintf("%q", strings.Join(lines, "\n"))
}

// ---- shim-inverted driving (adr-20260707-fakevsreal-shim-inversion) ----
//
// Every harness above drives a real codex app-server from agent-grid's
// client role. This section inverts that direction: real codex-cli 0.142.5
// drives agent-grid's own shim surface (cmd/bridge/codex_app_server_shim.go)
// as a `--remote` client, sending its "initialize" request under a JSON
// string id — the direction in which that string id was observed silently
// dropped (spec-20260707-codexclient-jsonrpc-id-opaque AC-005).
//
// codexShimSession/shimDownstreamHandler/shimUpstreamHandler live in package
// main (cmd/bridge) and cannot be imported from this package, so this
// rebuilds the same forward-and-preserve-the-id contract from codexclient's
// exported Conn/RequestID primitives — the same primitives the real shim is
// itself built on (see conn.go's RequestID doc comment). The upstream side
// is an in-process fakecodex.Server rather than a second real binary, which
// keeps this subtest hermetic and fast while still exercising the real
// codex-cli 0.142.5 binary as the driving client (NFR-003: on failure,
// suspect the shim/codexclient wiring below, not fakecodex).

// shimInvertedListenerTransport adapts a coder/websocket connection accepted
// on the shim-inverted listen socket to codexclient.Transport.
type shimInvertedListenerTransport struct {
	c  *websocket.Conn
	mu sync.Mutex
}

func (t *shimInvertedListenerTransport) ReadMessage(ctx context.Context) ([]byte, error) {
	_, data, err := t.c.Read(ctx)
	return data, err
}

func (t *shimInvertedListenerTransport) WriteMessage(ctx context.Context, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.c.Write(ctx, websocket.MessageText, data)
}

func (t *shimInvertedListenerTransport) Close() error { return t.c.CloseNow() }

// wireIDEnvelope decodes just enough of a JSON-RPC 2.0 frame to observe its
// "id" and "method" members without losing the id's raw wire bytes.
type wireIDEnvelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

// shimInvertedIDRecorder observes the raw wire bytes of real codex-cli's
// "initialize" request and of the reply forwarded back to it, so the test
// can assert the id round-trips byte-for-byte (AC-005) directly off the
// wire, independent of codexclient.RequestID's own preserve-the-bytes
// contract.
type shimInvertedIDRecorder struct {
	mu      sync.Mutex
	reqID   json.RawMessage
	replyID json.RawMessage
	done    chan struct{}
}

func newShimInvertedIDRecorder() *shimInvertedIDRecorder {
	return &shimInvertedIDRecorder{done: make(chan struct{})}
}

func (r *shimInvertedIDRecorder) observeInbound(raw []byte) {
	var env wireIDEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Method != codexschema.MethodInitialize || len(env.ID) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reqID == nil {
		r.reqID = append(json.RawMessage(nil), env.ID...)
	}
}

func (r *shimInvertedIDRecorder) observeOutbound(raw []byte) {
	var env wireIDEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Method != "" || len(env.ID) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reqID == nil || r.replyID != nil || !bytes.Equal(r.reqID, env.ID) {
		return
	}
	r.replyID = append(json.RawMessage(nil), env.ID...)
	close(r.done)
}

func (r *shimInvertedIDRecorder) snapshot() (reqID, replyID json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reqID, r.replyID
}

// shimInvertedErrSink collects errors surfaced by the shim-inverted harness's
// background goroutines (the websocket accept loop, the downstream/upstream
// Conn.Run loops, http.Server.Serve, and the downstream Reply/ReplyError/
// Notify writes). Per NFR-003 a failure anywhere in this wiring must point
// the reader at the shim/codexclient layer that broke rather than manifest
// only as the top-level 30s rec.done timeout, so every call site that used to
// discard its error with `_ =` now routes it here instead. Background
// goroutines only ever call add (no *testing.T access), so it is safe for
// them to keep running past the point the subtest body returns; reportTo is
// called once, synchronously, from the subtest body itself.
type shimInvertedErrSink struct {
	mu   sync.Mutex
	errs []string
}

func (s *shimInvertedErrSink) add(label string, err error) {
	if err == nil || isBenignShimShutdown(err) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, fmt.Sprintf("%s: %v", label, err))
}

func (s *shimInvertedErrSink) reportTo(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.errs {
		t.Errorf("shim-inverted harness error: %s", e)
	}
}

// isBenignShimShutdown reports whether err is an expected consequence of
// tearing the harness down (context cancellation, a closed pipe/socket,
// http.ErrServerClosed) rather than a genuine failure in the wiring under
// test.
func isBenignShimShutdown(err error) bool {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, io.EOF),
		errors.Is(err, io.ErrClosedPipe),
		errors.Is(err, net.ErrClosed),
		errors.Is(err, http.ErrServerClosed):
		return true
	default:
		s := err.Error()
		// SIGKILL of the codex-cli subprocess at t.Cleanup drops the
		// websocket abruptly and the accepting side's Read returns
		// "connection reset by peer" as a syscall.ECONNRESET wrapping —
		// not one of the sentinel errors above. This is race-dependent
		// (whether the kernel delivers the RST before the CLI process
		// finishes its own graceful close) and unrelated to what's under
		// test, so it must count as a benign teardown artifact.
		return strings.Contains(s, "use of closed network connection") ||
			strings.Contains(s, "connection reset by peer")
	}
}

// recordingTransport wraps a codexclient.Transport, feeding every inbound
// read and outbound write to rec before passing the bytes through unchanged.
type recordingTransport struct {
	codexclient.Transport
	rec *shimInvertedIDRecorder
}

func (t *recordingTransport) ReadMessage(ctx context.Context) ([]byte, error) {
	data, err := t.Transport.ReadMessage(ctx)
	if err == nil {
		t.rec.observeInbound(data)
	}
	return data, err
}

func (t *recordingTransport) WriteMessage(ctx context.Context, data []byte) error {
	t.rec.observeOutbound(data)
	return t.Transport.WriteMessage(ctx, data)
}

// shimInvertedDownstreamHandler forwards a downstream (real codex-cli)
// server-initiated request to the upstream (fakecodex) Conn and relays the
// reply back under the ORIGINAL downstream request id — mirroring
// cmd/bridge/codex_app_server_shim.go's shimDownstreamHandler, rebuilt here
// from exported codexclient primitives only (package main is not
// importable).
type shimInvertedDownstreamHandler struct {
	downstream *codexclient.Conn
	upstream   *codexclient.Conn
	errs       *shimInvertedErrSink
}

func (h shimInvertedDownstreamHandler) OnNotification(method string, params json.RawMessage) {
	h.errs.add("upstream.Notify("+method+")", h.upstream.Notify(method, params))
}

func (h shimInvertedDownstreamHandler) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	result, err := h.upstream.Request(method, params)
	if err != nil {
		// Mirror the real shim (cmd/bridge/codex_app_server_shim.go): use
		// ReplyRPCError so upstream JSON-RPC error object bytes forward
		// verbatim (code / message / data) and local synthesized errors
		// still get a spec-compliant -32603 fill. Without this, the
		// downstream (real codex-cli) rejects the reply as
		// "invalid JSON-RPC: data did not match any variant of untagged
		// enum JSONRPCMessage" because the error object lacks the
		// required "code" field.
		h.errs.add("downstream.ReplyRPCError("+method+")", h.downstream.ReplyRPCError(id, err))
		return
	}
	h.errs.add("downstream.Reply("+method+")", h.downstream.Reply(id, result))
}

// shimInvertedUpstreamHandler is the fakecodex-facing side of the harness.
// The initialize-only subtest here never drives fakecodex into emitting a
// notification or a server-initiated request of its own — fakecodex only
// ever replies to initialize/thread/start (see fakecodex.go's
// OnServerRequest) before this subtest tears down — so this handler
// intentionally has nothing to forward. It exists purely to keep the
// upstream Conn's read loop (Conn.Run) running so replies to OUR requests
// get dispatched out of the pending map.
type shimInvertedUpstreamHandler struct{}

func (shimInvertedUpstreamHandler) OnNotification(_ string, _ json.RawMessage) {}
func (shimInvertedUpstreamHandler) OnServerRequest(_ codexclient.RequestID, _ string, _ json.RawMessage) {
}

// attachFakeUpstream wires an in-process fakecodex.Server as the shim's
// upstream. Per adr-20260707-fakevsreal-shim-inversion, the fake stands in
// for the real app-server on this side so the subtest stays hermetic while
// real codex-cli 0.142.5 remains the actual driving client under test.
func attachFakeUpstream(ctx context.Context, fake *Server, errs *shimInvertedErrSink) (upstream *codexclient.Conn, stop func()) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	stopFake := fake.Attach(ctx, pr2, pw1)
	upstream = codexclient.NewConn(codexclient.StdioTransport(pr1, pw2), 30*time.Second)
	go func() { errs.add("upstream.Run", upstream.Run(ctx, shimInvertedUpstreamHandler{})) }()
	stop = func() {
		stopFake()
		_ = upstream.Close()
	}
	return upstream, stop
}

// startShimInvertedListener binds sock and, for every accepted downstream
// (real codex-cli) connection, forwards requests to upstream so replies
// preserve the caller's original id bytes verbatim.
func startShimInvertedListener(t *testing.T, sock string, upstream *codexclient.Conn, rec *shimInvertedIDRecorder, errs *shimInvertedErrSink) func() {
	t.Helper()
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen shim-inverted socket: %v", err)
	}
	httpSrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			errs.add("websocket.Accept", err)
			return
		}
		defer func() { _ = ws.CloseNow() }()
		ws.SetReadLimit(-1)
		downstream := codexclient.NewConn(&recordingTransport{
			Transport: &shimInvertedListenerTransport{c: ws},
			rec:       rec,
		}, 30*time.Second)
		errs.add("downstream.Run", downstream.Run(r.Context(), shimInvertedDownstreamHandler{downstream: downstream, upstream: upstream, errs: errs}))
	})}
	go func() { errs.add("httpSrv.Serve", httpSrv.Serve(ln)) }()
	return func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		_ = ln.Close()
		_ = os.Remove(sock)
	}
}

// TestE2E_ShimInvertedDriving pins adr-20260707-fakevsreal-shim-inversion.
// Like every other test in this file it rides the existing AG_E2E_CODEX_BIN
// gate (see e2eCodexBin in codex_appserver_e2e_test.go): unset, this test
// t.Skip()s so normal `go test ./...` (no -tags e2e) never even builds this
// file (AC-008); set to a real codex-cli 0.142.5 binary, it drives that
// binary for real (AC-005). No new env var or build tag is introduced.
func TestE2E_ShimInvertedDriving(t *testing.T) {
	bin := e2eCodexBin(t) // gated on AG_E2E_CODEX_BIN

	t.Run("shim_inverted_string_id_initialize", func(t *testing.T) {
		home := clonedHomeWithCodex(t)
		sock := filepath.Join(t.TempDir(), "codex-shim-inverted.sock")
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}

		// errs is reported first among these defers so it runs LAST (defers
		// are LIFO): it is checked only after ctx/upstream/listener teardown
		// has been kicked off, but the genuine round-trip already completed
		// (or failed) earlier in the function body, so nothing substantive
		// is missed.
		errs := &shimInvertedErrSink{}
		defer errs.reportTo(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		rec := newShimInvertedIDRecorder()
		fake := New(Config{})
		upstream, stopFake := attachFakeUpstream(ctx, fake, errs)
		defer stopFake()

		stopListener := startShimInvertedListener(t, sock, upstream, rec, errs)
		defer stopListener()
		e2etest.WaitForUnixSocketReady(t, sock, 10*time.Second)

		cli := spawnRealRemoteCLI(t, bin, home, sock, cwd, nil, "")
		defer cli.Close()

		select {
		case <-rec.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("shim never observed a completed initialize round trip driven by real codex-cli; cli output=%s", formatCLIDebug(cli.snapshot()))
		}

		reqID, replyID := rec.snapshot()
		t.Logf("real codex-cli initialize id=%s shim-forwarded reply id=%s", reqID, replyID)
		if len(reqID) == 0 {
			t.Fatal("shim never observed real codex-cli's initialize request id")
		}
		if reqID[0] != '"' {
			t.Fatalf("real codex-cli's initialize id = %s, want a JSON string id (the historically dropped shape)", reqID)
		}
		if !bytes.Equal(reqID, replyID) {
			t.Fatalf("shim-forwarded initialize reply id = %s, want bytes-preserving match with request id %s", replyID, reqID)
		}
	})

	// shim_inverted_forwards_upstream_error pins the error-object
	// bytes-preserving invariant that is the twin of the id-opacity SSOT
	// (see docs/note/note-20260707-technical-jsonrpc-id-opacity.md). Before
	// the fix, cmd/bridge/codex_app_server_shim.go's shimDownstreamHandler
	// called ReplyError(id, err.Error()) on any upstream failure; the
	// codexclient.ReplyError implementation in turn produced
	// {"error":{"message":"..."}} without the "code" field required by
	// codex-cli 0.142.5's JSONRPCErrorError schema, so any upstream failure
	// during thread/start / initialize surfaced to real codex-cli as
	// "sent invalid JSON-RPC: data did not match any variant of untagged
	// enum JSONRPCMessage" and killed the TUI bootstrap. This subtest
	// drives the fixed code path with fakecodex.FailInit=true: shim now
	// uses ReplyRPCError (which bytes-preserves the peer's error object,
	// or fills code=-32603 for local errors), and real codex-cli must
	// receive a parseable JSON-RPC error rather than an envelope-shape
	// rejection.
	t.Run("shim_inverted_upstream_err", func(t *testing.T) {
		home := clonedHomeWithCodex(t)
		// t.TempDir() paths in this deeply-nested subtest exceed the
		// 108-byte AF_UNIX sun_path limit; drop the socket in a short
		// /tmp/ dir the test owns and cleans up.
		sockDir, err := os.MkdirTemp("", "cxerr-")
		if err != nil {
			t.Fatalf("MkdirTemp: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
		sock := filepath.Join(sockDir, "s.sock")
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}

		errs := &shimInvertedErrSink{}
		defer errs.reportTo(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		rec := newShimInvertedIDRecorder()
		fake := New(Config{FailInit: true})
		upstream, stopFake := attachFakeUpstream(ctx, fake, errs)
		defer stopFake()

		stopListener := startShimInvertedListener(t, sock, upstream, rec, errs)
		defer stopListener()
		e2etest.WaitForUnixSocketReady(t, sock, 10*time.Second)

		cli := spawnRealRemoteCLI(t, bin, home, sock, cwd, nil, "")
		defer cli.Close()

		// codex-cli terminates quickly once it receives the initialize
		// error reply, since --remote treats a rejected initialize as
		// fatal. Wait for either the process to exit or a hard cap.
		select {
		case <-cli.wait:
		case <-time.After(15 * time.Second):
			t.Fatalf("codex-cli did not exit within 15s after upstream FailInit; output=%s", formatCLIDebug(cli.snapshot()))
		}

		out := strings.Join(cli.snapshot(), "\n")
		// Regression signals — these substrings only appear in codex-cli
		// output when it failed to deserialize a reply as JSONRPCMessage
		// (the exact pre-fix failure mode: {"error":{"message":"..."}}
		// without the required "code" field). A spec-compliant reply is
		// parsed cleanly and codex-cli surfaces the app-level failure
		// instead (its own "failed to connect to remote app server" line
		// on init rejection). Their absence pins the fix.
		regressionSignals := []string{
			"invalid JSON-RPC",
			"did not match any variant",
			"JSONRPCMessage",
		}
		for _, s := range regressionSignals {
			if strings.Contains(out, s) {
				t.Fatalf("codex-cli reported %q — shim reply failed JSON-RPC envelope parsing; output=%s", s, formatCLIDebug(cli.snapshot()))
			}
		}
		// Positive-liveness signal: codex-cli must have reached its own
		// initialize-failure branch (not hung at the transport layer),
		// meaning our reply arrived and parsed. codex-cli 0.142.5's TUI
		// surfaces this as "failed to connect to remote app server" or
		// similar; matching just "remote app server" keeps the assertion
		// robust to phrasing tweaks across patch versions while still
		// distinguishing "codex-cli got a reply and hard-failed init"
		// from "codex-cli never received a parseable reply".
		if !strings.Contains(out, "remote app server") {
			t.Fatalf("codex-cli output has no app-server reaction line; either the shim never delivered a reply or the failure mode changed; output=%s", formatCLIDebug(cli.snapshot()))
		}
	})
}
