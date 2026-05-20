package agent

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const (
	testThreadID = "thread-abc"
	testTurnID   = "turn-xyz"
)

// fakeServer simulates a codex app-server over an in-memory pipe.
// It handles initialize, then responds to turn/start by emitting the standard sequence.
type fakeServer struct {
	srv      *codexclient.Server
	failTurn bool // if true, emits error instead of turn/completed
	hangTurn bool // if true, starts the session but never completes the turn
	mu       sync.Mutex
}

func (f *fakeServer) OnNotification(method string, _ json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	f.mu.Lock()
	fail := f.failTurn
	hang := f.hangTurn
	f.mu.Unlock()

	_ = f.srv.EmitThreadStarted(testThreadID, "/ws")
	_ = f.srv.EmitTurnStarted(testThreadID, testTurnID)
	switch {
	case hang:
		// session is live but the turn never resolves — exercises turn_timeout_ms.
	case fail:
		_ = f.srv.EmitTurnFailed(testThreadID, "simulated failure")
	default:
		_ = f.srv.EmitTurnCompleted(testThreadID, testTurnID, "done")
	}
}

func (f *fakeServer) OnServerRequest(id int64, method string, _ json.RawMessage) {
	if method == codexschema.MethodInitialize {
		_ = f.srv.Conn().Reply(id, map[string]any{})
	}
}

// makeFakeProc returns a procFunc that wires runner ↔ fakeServer via io.Pipe.
func makeFakeProc(fs *fakeServer) procFunc {
	return func(ctx context.Context, cwd, cmdLine string) (io.ReadCloser, io.WriteCloser, func(), error) {
		// runner reads pr1; server reads pr2
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()

		serverConn := codexclient.NewConn(
			codexclient.StdioTransport(pr2, pw1),
			2*time.Second,
		)
		fs.srv = codexclient.NewServer(serverConn)

		go func() {
			defer pw2.Close()
			_ = serverConn.Run(ctx, fs)
		}()

		// The stdio transport is not context-aware; emulate process death on
		// cancellation by closing the runner's read end so its loop sees EOF
		// (a real bash subprocess dies and EOFs its stdout the same way).
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()

		return pr1, pw2, func() {}, nil
	}
}

func makeRunner(t *testing.T, tmpl string, proc procFunc) *Runner {
	t.Helper()
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused-in-test"},
	}
	ws := workspace.New(cfg)
	return &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: tmpl,
		proc:           proc,
	}
}

func collectEvents(t *testing.T, r *Runner, issue tracker.Issue, attempt int) []Event {
	t.Helper()
	var mu sync.Mutex
	var events []Event
	emit := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := r.spawnWith(ctx, issue, attempt, emit)
	require.NoError(t, err)
	assert.Equal(t, testThreadID+"-"+testTurnID, sess.SessionID)
	assert.NotNil(t, sess.Worker)

	// wait for monitor to deliver turn_completed
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 2 { //nolint:mnd
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	out := make([]Event, len(events))
	copy(out, events)
	mu.Unlock()
	return out
}

func TestSpawn_sessionStartedAndTurnCompleted(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "Work on {{ issue.identifier }}", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-1", Title: "Test issue"}

	events := collectEvents(t, r, iss, 1)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, testThreadID+"-"+testTurnID, events[0].SessionID)
	assert.Equal(t, EventTurnCompleted, events[1].Kind)
	assert.Nil(t, events[1].Err)
}

func TestSpawn_turnFailedEmitsEvent(t *testing.T) {
	fs := &fakeServer{failTurn: true}
	r := makeRunner(t, "", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-2"}

	events := collectEvents(t, r, iss, 1)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, EventTurnFailed, events[1].Kind)
	assert.NotNil(t, events[1].Err)
}

func TestSpawn_turnTimeoutKillsAndFails(t *testing.T) {
	fs := &fakeServer{hangTurn: true}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused", TurnTimeoutMS: 50},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-T"}

	events := collectEvents(t, r, iss, 1)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, EventTurnFailed, events[1].Kind)
	assert.ErrorContains(t, events[1].Err, "turn timeout")
}

func TestSpawn_workspaceEnsureCreatesDir(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-3"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.NoError(t, err)

	wsPath, _ := r.Workspace.Path(iss.Identifier)
	info, statErr := os.Stat(wsPath)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestSpawn_beforeRunFailureAborts(t *testing.T) {
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Hooks: wfconfig.HooksConfig{
			BeforeRun: "exit 1",
			TimeoutMS: 2000,
		},
		Codex: wfconfig.CodexConfig{Command: "unused"},
	}
	ws := workspace.New(cfg)
	// pre-create the workspace dir so Ensure succeeds before hook runs
	iss := tracker.Issue{Identifier: "PROJ-4"}
	wsPath := filepath.Join(wsRoot, iss.Identifier)
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	r := &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: "",
		proc:           makeFakeProc(&fakeServer{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	assert.Error(t, err, "before_run failure should abort spawn")
}
