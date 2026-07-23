package runtime_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	clientruntime "github.com/takezoh/agent-grid/host/runtime"
	"github.com/takezoh/agent-grid/host/runtime/runtimetest"
	"github.com/takezoh/agent-grid/host/state"
)

type blockingBackend struct {
	mu             sync.Mutex
	spawnCalls     int
	spawnCmds      []string
	captureStarted chan struct{}
	captureRelease chan struct{}
	captureOnce    sync.Once
	releaseOnce    sync.Once
}

func newBlockingBackend() *blockingBackend {
	return &blockingBackend{
		captureStarted: make(chan struct{}),
		captureRelease: make(chan struct{}),
	}
}

func (b *blockingBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string, _, _ uint16) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spawnCalls++
	b.spawnCmds = append(b.spawnCmds, command)
	return nil
}

func (b *blockingBackend) KillFrame(string) error                    { return nil }
func (b *blockingBackend) ResolveID(string) (string, error)          { return "", nil }
func (b *blockingBackend) SetEnv(string, string) error               { return nil }
func (b *blockingBackend) UnsetEnv(string) error                     { return nil }
func (b *blockingBackend) FrameExitStatus(string) (bool, int, error) { return false, -1, nil }
func (b *blockingBackend) RespawnFrame(string, string) error         { return nil }
func (b *blockingBackend) ShowEnvironment() (string, error)          { return "", nil }
func (b *blockingBackend) SendKeys(string, string) error             { return nil }
func (b *blockingBackend) SendKey(string, string) error              { return nil }
func (b *blockingBackend) LoadBuffer(string, string) error           { return nil }
func (b *blockingBackend) PasteBuffer(string, string) error          { return nil }
func (b *blockingBackend) SendEnter(string) error                    { return nil }

func (b *blockingBackend) CaptureFrame(string, int) (string, error) {
	b.captureOnce.Do(func() { close(b.captureStarted) })
	<-b.captureRelease
	return "", nil
}

func (b *blockingBackend) WaitForCapture(t *testing.T) {
	t.Helper()
	select {
	case <-b.captureStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("blockingBackend: CaptureFrame was not called within 2s")
	}
}

func (b *blockingBackend) ReleaseCapture() {
	b.releaseOnce.Do(func() { close(b.captureRelease) })
}

func (b *blockingBackend) SpawnCalls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spawnCalls
}

func (b *blockingBackend) LastSpawnCommand() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.spawnCmds) == 0 {
		return ""
	}
	return b.spawnCmds[len(b.spawnCmds)-1]
}

func createSessionPayload(project, command string) json.RawMessage {
	body, _ := json.Marshal(map[string]string{
		"project": project,
		"command": command,
	})
	return body
}

func stopSessionPayload(sessionID state.SessionID) json.RawMessage {
	body, _ := json.Marshal(map[string]string{
		"session_id": string(sessionID),
	})
	return body
}

func waitForSingleSession(t *testing.T, h *runtimetest.Harness) (state.State, state.SessionID) {
	t.Helper()
	snapshot := h.WaitFor(t, func(s state.State) bool { return len(s.Sessions) == 1 })
	for id := range snapshot.Sessions {
		return snapshot, id
	}
	t.Fatal("expected one session")
	return state.State{}, ""
}

func TestRuntimeHarness_CreateSessionFlow(t *testing.T) {
	backend := newBlockingBackend()
	h := runtimetest.New(t, runtimetest.WithBackend(backend))

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "create", Event: state.EventCreateSession,
		Payload: createSessionPayload("/tmp/harness-create", "stub-fallback"),
	})

	snapshot, sessionID := waitForSingleSession(t, h)
	h.Quiesce(t)

	if backend.SpawnCalls() != 1 {
		t.Fatalf("SpawnFrame calls = %d, want 1", backend.SpawnCalls())
	}
	session := snapshot.Sessions[sessionID]
	if session.Project != "/tmp/harness-create" {
		t.Fatalf("session project = %q, want /tmp/harness-create", session.Project)
	}
	if len(session.Frames) != 1 {
		t.Fatalf("session frames = %d, want 1", len(session.Frames))
	}
}

func TestRuntimeHarness_RequestShutdownTimeoutWhenLoopIsBlocked(t *testing.T) {
	backend := newBlockingBackend()
	h := runtimetest.New(t, runtimetest.WithBackend(backend))

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "create", Event: state.EventCreateSession,
		Payload: createSessionPayload("/tmp/harness-shutdown", "stub-fallback"),
	})
	_, sessionID := waitForSingleSession(t, h)

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "stop", Event: state.EventStopSession,
		Payload: stopSessionPayload(sessionID),
	})
	backend.WaitForCapture(t)

	start := time.Now()
	result := h.Runtime().RequestShutdown(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("RequestShutdown returned after %v, want <=500ms", elapsed)
	}
	if result != clientruntime.ShutdownResultDeadlineExceeded {
		t.Fatalf("RequestShutdown result = %q, want deadline_exceeded", result)
	}

	backend.ReleaseCapture()
	select {
	case <-h.Runtime().Done():
	case <-time.After(time.Second):
		t.Fatal("runtime did not terminate after processing expired shutdown transaction")
	}
}

func TestRuntimeHarness_EnqueueDropsWhenEventQueueIsFull(t *testing.T) {
	backend := newBlockingBackend()
	h := runtimetest.New(t, runtimetest.WithBackend(backend))

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "create", Event: state.EventCreateSession,
		Payload: createSessionPayload("/tmp/harness-fill", "stub-fallback"),
	})
	_, sessionID := waitForSingleSession(t, h)

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "stop", Event: state.EventStopSession,
		Payload: stopSessionPayload(sessionID),
	})
	backend.WaitForCapture(t)

	for i := 0; i < h.Runtime().TestEventQueueCapacity(); i++ {
		h.Runtime().Enqueue(state.EvTick{Now: time.Unix(int64(i), 0), N: uint64(i + 1)})
	}
	h.Runtime().Enqueue(state.EvEvent{
		ConnID: 1, ReqID: "drop", Event: state.EventCreateSession,
		Payload: createSessionPayload("/tmp/harness-dropped", "stub-fallback"),
	})

	backend.ReleaseCapture()
	h.Quiesce(t)

	snapshot := h.Runtime().TestPublishedState()
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after stop-session and dropped enqueue", len(snapshot.Sessions))
	}
	if backend.SpawnCalls() != 1 {
		t.Fatalf("SpawnFrame calls = %d, want 1 after dropped enqueue", backend.SpawnCalls())
	}
	for _, session := range snapshot.Sessions {
		if strings.Contains(session.Project, "harness-dropped") {
			t.Fatal("dropped create-session unexpectedly reached reducer state")
		}
	}
}

func TestRuntimeHarness_ShellCreateSessionSpawnsCommand(t *testing.T) {
	backend := newBlockingBackend()
	h := runtimetest.New(t, runtimetest.WithBackend(backend))

	h.Enqueue(t, state.EvEvent{
		ConnID: 1, ReqID: "create", Event: state.EventCreateSession,
		Payload: createSessionPayload("/tmp/harness-shell", "shell"),
	})
	waitForSingleSession(t, h)
	h.Quiesce(t)

	if backend.SpawnCalls() != 1 {
		t.Fatalf("SpawnFrame calls = %d, want 1", backend.SpawnCalls())
	}
	// Shell sessions also go through frame-exec; login-shell expansion is the
	// FrameSpec MainCommand, not the host spawn string.
	command := backend.LastSpawnCommand()
	if !strings.Contains(command, "frame-exec") {
		t.Fatalf("shell create-session command = %q, want frame-exec", command)
	}
}
