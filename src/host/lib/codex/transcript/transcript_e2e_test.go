//go:build e2e

package transcript

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

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-grid/platform/agent/codexschema/v2"
	"github.com/takezoh/agent-grid/platform/e2etest"
	codexcli "github.com/takezoh/agent-grid/platform/lib/codex"
	"github.com/takezoh/agent-grid/platform/procgroup"
)

func e2eCodexBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("AG_E2E_CODEX_BIN")
	if bin == "" {
		t.Skip("AG_E2E_CODEX_BIN is not set")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("AG_E2E_CODEX_BIN=%q is not executable: %v", bin, err)
	}
	return bin
}

func clonedHomeWithCodex(t *testing.T) string {
	t.Helper()
	return e2etest.PrepareCodexHome(t, ".codex-e2e-home-")
}

type e2eRecorder struct {
	mu      sync.Mutex
	methods []string
	params  map[string][]json.RawMessage
}

func (r *e2eRecorder) OnNotification(method string, params json.RawMessage) {
	r.mu.Lock()
	r.methods = append(r.methods, method)
	if r.params == nil {
		r.params = map[string][]json.RawMessage{}
	}
	r.params[method] = append(r.params[method], append(json.RawMessage(nil), params...))
	r.mu.Unlock()
}

func (r *e2eRecorder) OnServerRequest(_ codexclient.RequestID, _ string, _ json.RawMessage) {}

func startRealCodexAppServer(t *testing.T, bin, home, sock string, extra []string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	args := codexcli.AppServerListenArgs("codex", sock, extra, false)
	cmd := procgroup.Command(procgroup.Spec{Ctx: ctx, Bin: bin, Args: args[1:]})
	cmd.Env = append(os.Environ(), "HOME="+home)
	stderr := &syncBuffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start codex app-server: %v", err)
	}
	e2etest.WaitForUnixSocketReady(t, sock, 10*time.Second)
	return func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("codex stderr:\n%s", stderr.String())
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func waitForMethod(rec *e2eRecorder, method string, timeout time.Duration) json.RawMessage {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec.mu.Lock()
		items := append([]json.RawMessage(nil), rec.params[method]...)
		rec.mu.Unlock()
		if len(items) > 0 {
			return items[len(items)-1]
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (r *e2eRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.methods)
}

func waitForRecorderIdle(t *testing.T, rec *e2eRecorder, timeout, quiet time.Duration) {
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
	t.Fatalf("recorder never became idle within %s; methods=%v", timeout, rec.methods)
}

type ptyCLI struct {
	cmd   *exec.Cmd
	ptmx  *os.File
	lines []string
	mu    sync.Mutex
	wait  chan error
}

func spawnRemoteCLI(t *testing.T, bin, home, sock, cwd string, extra []string, prompt string) *ptyCLI {
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
	cli := &ptyCLI{cmd: cmd, ptmx: ptmx, wait: make(chan error, 1)}
	go func() {
		scanner := bufio.NewScanner(ptmx)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			cli.mu.Lock()
			cli.lines = append(cli.lines, scanner.Text())
			cli.mu.Unlock()
		}
		_ = scanner.Err()
	}()
	go func() { cli.wait <- cmd.Wait() }()
	t.Cleanup(cli.Close)
	return cli
}

func (c *ptyCLI) Close() {
	if c.ptmx == nil {
		return
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
		_ = c.cmd.Process.Kill()
	}
	select {
	case <-c.wait:
	case <-time.After(2 * time.Second):
	}
	_ = c.ptmx.Close()
	c.ptmx = nil
}

func (c *ptyCLI) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.lines))
	copy(out, c.lines)
	return out
}

func (c *ptyCLI) lineCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.lines)
}

func (c *ptyCLI) waitForIdle(t *testing.T, timeout, quiet time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	lastCount := -1
	lastChange := time.Now()
	for time.Now().Before(deadline) {
		count := c.lineCount()
		if count != lastCount {
			lastCount = count
			lastChange = time.Now()
		}
		if time.Since(lastChange) >= quiet {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("codex pty never became idle within %s; cli output=%s", timeout, formatCLIDebug(c.snapshot()))
}

func (c *ptyCLI) SendPrompt(t *testing.T, prompt string) {
	t.Helper()
	c.waitForIdle(t, 30*time.Second, 1500*time.Millisecond)
	if _, err := c.ptmx.Write([]byte(prompt)); err != nil {
		t.Fatalf("write prompt to codex pty: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := c.ptmx.Write([]byte{'\r'}); err != nil {
		t.Fatalf("submit prompt to codex pty: %v", err)
	}
}

func waitForTurnContext(t *testing.T, path string, timeout time.Duration) (json.RawMessage, string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil && len(raw) > 0 {
			payload, effort, ok := extractTurnContext(raw)
			if ok {
				return payload, effort
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("turn_context not found in rollout within %s: %s", timeout, path)
	return nil, ""
}

func extractTurnContext(raw []byte) (json.RawMessage, string, bool) {
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	scan.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		var item rolloutLine
		if json.Unmarshal([]byte(line), &item) != nil || item.Type != "turn_context" {
			continue
		}
		var payload struct {
			ReasoningEffort any `json:"reasoning_effort"`
			Effort          any `json:"effort"`
		}
		if json.Unmarshal(item.Payload, &payload) != nil {
			continue
		}
		effort := normalizeReasoningEffort(payload.ReasoningEffort)
		if effort == "" {
			effort = normalizeReasoningEffort(payload.Effort)
		}
		return item.Payload, effort, effort != ""
	}
	return nil, "", false
}

func TestE2E_RealCodexRolloutUsesReasoningEffortSchema(t *testing.T) {
	bin := e2eCodexBin(t)
	home := clonedHomeWithCodex(t)
	sockDir := e2etest.NewWorkspaceTempDir(t, ".codex-e2e-sock-")
	sock := filepath.Join(sockDir, "codex-appserver.sock")
	stopServer := startRealCodexAppServer(t, bin, home, sock, []string{
		"-c", `sandbox_mode="danger-full-access"`,
		"-c", `model_reasoning_effort="high"`,
	})
	defer stopServer()

	tr, err := codexclient.DialUDS(sock, 10*time.Second)
	if err != nil {
		t.Fatalf("dial observer uds: %v", err)
	}
	conn := codexclient.NewConn(tr, 30*time.Second)

	rec := &e2eRecorder{params: map[string][]json.RawMessage{}}
	go func() { _ = conn.Run(context.Background(), rec) }()

	if err := codexclient.Initialize(conn); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	initTr, err := codexclient.DialUDS(sock, 10*time.Second)
	if err != nil {
		t.Fatalf("dial initiator uds: %v", err)
	}
	initConn := codexclient.NewConn(initTr, 30*time.Second)
	initRec := &e2eRecorder{params: map[string][]json.RawMessage{}}
	go func() { _ = initConn.Run(context.Background(), initRec) }()
	if err := codexclient.Initialize(initConn); err != nil {
		t.Fatalf("initiator Initialize: %v", err)
	}
	session, err := codexclient.StartThread(initConn, cwd, nil, codexclient.ThreadOptions{})
	if err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadRaw := waitForMethod(rec, codexschema.MethodThreadStarted, 45*time.Second)
	if threadRaw == nil {
		t.Fatalf("real codex never emitted %s", codexschema.MethodThreadStarted)
	}
	thread, err := codexschemav2.UnmarshalThreadStartedNotification(threadRaw)
	if err != nil {
		t.Fatalf("decode thread/started: %v", err)
	}
	if thread.Thread.Path == nil || strings.TrimSpace(*thread.Thread.Path) == "" {
		t.Fatalf("thread/started missing rollout path: %s", string(threadRaw))
	}
	waitForRecorderIdle(t, initRec, 30*time.Second, 1500*time.Millisecond)
	if err := codexclient.StartTurn(initConn, session.ThreadID, cwd, []byte("Reply with hi."), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("turn/start: %v", err)
	}
	if waitForMethod(initRec, codexschema.MethodTurnStarted, 45*time.Second) == nil {
		t.Fatalf("real codex never emitted %s on initiator; initiator=%v observer=%v", codexschema.MethodTurnStarted, initRec.methods, rec.methods)
	}
	if waitForMethod(initRec, codexschema.MethodTurnCompleted, 3*time.Minute) == nil {
		t.Fatalf("real codex never emitted %s on initiator; initiator=%v observer=%v", codexschema.MethodTurnCompleted, initRec.methods, rec.methods)
	}

	rolloutPath := strings.TrimSpace(*thread.Thread.Path)
	turnContextPayload, rawEffort := waitForTurnContext(t, rolloutPath, 10*time.Second)
	if rawEffort == "" {
		t.Fatalf("turn_context missing reasoning_effort: %s", string(turnContextPayload))
	}

	rawRollout, readErr := os.ReadFile(rolloutPath)
	if readErr != nil {
		t.Fatalf("read rollout: %v", readErr)
	}
	parser := NewParser()
	parser.ParseLines(rawRollout)
	snap := parser.Snapshot()
	if snap.Effort != rawEffort {
		t.Fatalf("Snapshot.Effort = %q, want %q from turn_context payload %s", snap.Effort, rawEffort, string(turnContextPayload))
	}
	if !strings.Contains(string(turnContextPayload), `"reasoning_effort"`) &&
		!strings.Contains(string(turnContextPayload), `"effort"`) {
		t.Fatalf("turn_context payload did not include effort metadata: %s", string(turnContextPayload))
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
