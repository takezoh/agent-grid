//go:build e2e

package fakecodex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-reactor/platform/agent/codexschema/v2"
	"github.com/takezoh/agent-reactor/platform/e2etest"
	codexcli "github.com/takezoh/agent-reactor/platform/lib/codex"
	"github.com/takezoh/agent-reactor/platform/procgroup"
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
	r.params[method] = append(r.params[method], append(json.RawMessage(nil), params...))
	r.mu.Unlock()
}

func (r *realEventRecorder) OnServerRequest(_ int64, method string, params json.RawMessage) {
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
