package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// pipeShim wires the shim server to a client Conn via two io.Pipe pairs.
// ids is a deterministic sequence for newID; returns the client conn, cancel, and a done chan.
func pipeShim(t *testing.T, launch claudeLauncher, ids []string) (*codexclient.Conn, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	shimTransport := codexclient.StdioTransport(pr1, pw2)
	clientTransport := codexclient.StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())

	idIdx := 0
	var idMu sync.Mutex
	newID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		if idIdx < len(ids) {
			v := ids[idIdx]
			idIdx++
			return v
		}
		return "id-extra"
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer pw2.Close()
		runWith(ctx, shimTransport, launch, newID)
	}()

	t.Cleanup(func() {
		cancel()
		pw1.Close()
		<-done
	})

	clientConn := codexclient.NewConn(clientTransport, 5*time.Second)
	return clientConn, cancel, done
}

// notificationCollector records inbound notifications on a client Conn.
type notificationCollector struct {
	mu      sync.Mutex
	methods []string
	params  map[string]json.RawMessage // last params per method key (method+index not tracked; last wins)
}

func (c *notificationCollector) OnNotification(method string, params json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods = append(c.methods, method)
	if c.params == nil {
		c.params = make(map[string]json.RawMessage)
	}
	c.params[method] = append([]byte{}, params...)
}

func (c *notificationCollector) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

func (c *notificationCollector) received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.methods))
	copy(out, c.methods)
	return out
}

func (c *notificationCollector) lastParams(method string) map[string]any {
	c.mu.Lock()
	raw := c.params[method]
	c.mu.Unlock()
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

// fakeLauncherSequence returns a launcher that, for each successive call,
// returns the corresponding line sequence from sequences. The last sequence
// is reused for any extra calls.
func fakeLauncherSequence(calls *[][]string, sequences ...[]string) claudeLauncher {
	i := 0
	var mu sync.Mutex
	return func(ctx context.Context, cwd, resumeID, prompt string) (io.ReadCloser, func() error, error) {
		mu.Lock()
		idx := i
		if i < len(sequences)-1 {
			i++
		} else {
			i = len(sequences) - 1
		}
		mu.Unlock()
		*calls = append(*calls, []string{cwd, resumeID, prompt})
		body := strings.Join(sequences[idx], "\n") + "\n"
		return io.NopCloser(strings.NewReader(body)), func() error { return nil }, nil
	}
}

// stream-json fixtures.
const (
	lineSystemInit = `{"type":"system","subtype":"init","session_id":"claude-sess-1"}`
	lineAssistant  = `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`
	lineResultOK   = `{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":10,"output_tokens":5}}`
	lineResultFail = `{"type":"result","subtype":"error","result":"oops","is_error":true,"usage":{"input_tokens":1,"output_tokens":0}}`
)

func waitForMethods(t *testing.T, nc *notificationCollector, want []string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got := nc.received()
		if len(got) >= len(want) {
			assert.Equal(t, want, got[:len(want)])
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %v, got %v", want, nc.received())
}

func TestShim_OneTurn(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineAssistant, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("hi")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodTurnCompleted,
	})
}

func TestShim_SessionID(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultOK})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("hi")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	})

	turnStartedParams := nc.lastParams(codexschema.MethodTurnStarted)
	assert.Equal(t, "thread-1-turn-1", turnStartedParams["sessionId"])

	turnCompletedParams := nc.lastParams(codexschema.MethodTurnCompleted)
	assert.Equal(t, "thread-1-turn-1", turnCompletedParams["sessionId"])
	assert.Equal(t, "done", turnCompletedParams["text"])
}

func TestShim_ContinuationResume(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineResultOK}, // first turn: new session
		[]string{lineSystemInit, lineResultOK}, // second turn: resume
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1", "turn-2"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))

	// First turn: no threadId → new thread/turn.
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("first")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	})

	// Second turn: pass threadId → shim should call launcher with --resume session id.
	require.NoError(t, codexclient.StartTurn(clientConn, "thread-1", "/ws", []byte("second")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	})

	require.Len(t, calls, 2)
	assert.Equal(t, "claude-sess-1", calls[1][1], "second turn should resume with claude session id")
}

func TestShim_TurnFailed(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultFail})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("fail me")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodError,
	})

	errParams := nc.lastParams(codexschema.MethodError)
	assert.Equal(t, "oops", errParams["message"])
}

func TestShim_KillPropagation(t *testing.T) {
	blocked := make(chan struct{})
	var launchCtx context.Context
	var launchMu sync.Mutex
	launch := func(ctx context.Context, cwd, resumeID, prompt string) (io.ReadCloser, func() error, error) {
		launchMu.Lock()
		launchCtx = ctx
		launchMu.Unlock()
		close(blocked)
		pr, pw := io.Pipe()
		go func() {
			<-ctx.Done()
			_ = pw.Close()
		}()
		return pr, func() error {
			<-ctx.Done()
			return ctx.Err()
		}, nil
	}

	clientConn, cancel, shimDone := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("block")))

	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("launcher was never called")
	}

	cancel()

	select {
	case <-shimDone:
	case <-time.After(3 * time.Second):
		t.Fatal("shim did not exit after cancel")
	}

	launchMu.Lock()
	lctx := launchCtx
	launchMu.Unlock()
	select {
	case <-lctx.Done():
	default:
		t.Error("launcher ctx was not cancelled when shim was cancelled")
	}
}

// TestProcessGroupKill verifies the process-group kill mechanism: spawning a
// process with Setpgid=true and a group-kill Cancel function terminates
// grandchildren when the context is cancelled.
func TestProcessGroupKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group kill not applicable on windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	scriptPath := dir + "/parent.sh"
	script := "#!/usr/bin/env bash\nsleep 9999 &\necho \"$!\"\nwait\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "bash", scriptPath) //nolint:gosec
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	// Read the grandchild PID.
	buf := make([]byte, 32)
	n, _ := stdout.Read(buf)
	grandchildPID := strings.TrimSpace(string(buf[:n]))
	require.NotEmpty(t, grandchildPID)

	// Kill via context cancellation.
	cancel()
	_ = cmd.Wait()

	time.Sleep(100 * time.Millisecond)

	out, _ := exec.Command("kill", "-0", grandchildPID).CombinedOutput()
	assert.Contains(t, string(out), "No such process",
		"grandchild PID %s should be dead after process group kill", grandchildPID)
}
