package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/host/runtime/subsystem/stream/fake"
	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-grid/platform/lib/codex"
	"github.com/takezoh/agent-grid/platform/pathmap"
)

// Stream subsystem launch-flow tests: the codex app-server is faked so no real
// codex binary runs. Two altitudes:
//
//   - BindFrame command/socket rewrite — white-box: b.conn is paired with an
//     in-process fake server over a pipe, BindFrame's thread/resume + turn/start
//     RPCs are answered, and the resolved launch command is asserted.
//   - Start ordering — a helper sub-process (this test binary re-invoked with a
//     leading "app-server" arg) binds a real WebSocket-over-UDS server, so the
//     full spawn → dial → initialize sequence runs. The Initialize-failure path
//     pins the e41ab1c regression where a failed handshake must reap the process.

// TestMain doubles as (1) the fake app-server entry point used by Backend
// spawn tests — invoked as `<bin> app-server --listen unix://<sock> --mode
// <mode>` — and (2) the fake CLI entry point used by interactive_flow_test's
// pty-attached FakeCLI (`<bin> fake-cli …`). Both dispatch tokens are
// consumed before the test runner sees the argv.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "app-server" {
		runFakeAppServer(os.Args[2:])
		os.Exit(0)
	}
	fake.MaybeRunCLIFromArgs(os.Args)
	os.Exit(m.Run())
}

// runFakeAppServer binds a WebSocket-over-UDS server at the --listen socket and
// answers `initialize` per --mode ("ok" → success, "initfail" → JSON-RPC error).
// It blocks serving until the parent SIGKILLs it (subsystem ctx cancel).
func runFakeAppServer(args []string) {
	var sock, mode string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 < len(args) {
				sock = strings.TrimPrefix(args[i+1], "unix://")
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				mode = args[i+1]
				i++
			}
		}
	}
	if sock == "" {
		os.Exit(2)
	}
	l, err := net.Listen("unix", sock)
	if err != nil {
		os.Exit(3)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg struct {
				ID     *int64 `json:"id"`
				Method string `json:"method"`
			}
			if json.Unmarshal(data, &msg) != nil || msg.ID == nil || msg.Method != codexschema.MethodInitialize {
				continue
			}
			reply := map[string]any{"id": *msg.ID, "result": map[string]any{}}
			if mode == "initfail" {
				reply = map[string]any{"id": *msg.ID, "error": map[string]any{"message": "boom"}}
			}
			b, _ := json.Marshal(reply)
			_ = c.Write(ctx, websocket.MessageText, b)
		}
	})}
	_ = srv.Serve(l)
}

// === BindFrame command / socket rewrite (white-box, in-process server) ===

// streamPipe wires two StdioTransports back-to-back (mirrors codexclient's own
// test helper, which is package-private).
func streamPipe() (codexclient.Transport, codexclient.Transport) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return codexclient.StdioTransport(pr1, pw2), codexclient.StdioTransport(pr2, pw1)
}

// bindServer is the in-process fake app-server for BindFrame tests. It replies
// to thread/start with a fresh unique thread id (cold start creates the thread
// synchronously) and to thread/resume with an empty result (backend keeps the
// requested id).
type bindServer struct {
	conn       *codexclient.Conn
	mu         sync.Mutex
	threadSeq  int
	lastResume map[string]any
	omitPath   bool
	customPath string
}

func (s *bindServer) OnNotification(string, json.RawMessage) {}

func (s *bindServer) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	if method == codexschema.MethodThreadStart {
		s.mu.Lock()
		s.threadSeq++
		tid := fmt.Sprintf("thread-%d", s.threadSeq)
		path := s.customPath
		omitPath := s.omitPath
		s.mu.Unlock()
		thread := map[string]any{"id": tid, "sessionId": "sess-" + tid}
		if !omitPath {
			if path == "" {
				path = "/repo/.codex/rollout.jsonl"
			}
			thread["path"] = path
		}
		_ = s.conn.Reply(id, map[string]any{"thread": thread})
		return
	}
	if method == codexschema.MethodThreadResume {
		var decoded map[string]any
		_ = json.Unmarshal(params, &decoded)
		s.mu.Lock()
		s.lastResume = decoded
		path := s.customPath
		omitPath := s.omitPath
		s.mu.Unlock()
		threadID, _ := decoded["threadId"].(string)
		if threadID == "" {
			threadID = "thread-resumed"
		}
		thread := map[string]any{"id": threadID, "sessionId": "sess-" + threadID}
		if !omitPath {
			if path == "" {
				if p, ok := decoded["path"].(string); ok {
					path = p
				}
			}
			if path != "" {
				thread["path"] = path
			}
		}
		_ = s.conn.Reply(id, map[string]any{"thread": thread})
		return
	}
	_ = s.conn.Reply(id, map[string]any{})
}

type boundHarness struct {
	backend *Backend
	server  *bindServer
}

func newBoundBackend(t *testing.T, listenSock string) *boundHarness {
	t.Helper()
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", "", false, false,
		listenSock, time.Second)
	ta, tb := streamPipe()
	b.conn = codexclient.NewConn(ta, time.Second)
	srv := &bindServer{conn: codexclient.NewConn(tb, time.Second)}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go b.conn.Run(ctx, b)     //nolint:errcheck
	go srv.conn.Run(ctx, srv) //nolint:errcheck
	return &boundHarness{backend: b, server: srv}
}

func writeRollout(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The launch-flow tests below pin BindFrame's contract under ADR-0081 and the
// observer-subscription ADR. The CLI owns thread creation and interactive
// attachment; Backend independently resumes the identity to subscribe:
//   - Fresh cold-start: pending binding (empty threadID), initState occupied,
//     Command shape has no `resume` argv.
//   - Recovery (ThreadID present): binding bound directly (backend knows the
//     id up front), initState untouched, Command shape has `resume <id>`.

func TestBackendBindFrame_FreshColdStart_LeavesPendingBinding(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-sess1.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan:    state.LaunchPlan{StartDir: "/repo"},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	// Fresh path: binding exists but threadID is still empty — the CLI's
	// thread/started (async) will fill it in via handleThreadStarted.
	h.backend.mu.Lock()
	binding := h.backend.frames["f1"]
	h.backend.mu.Unlock()
	if binding == nil {
		t.Fatal("fresh BindFrame did not register a frameBinding")
	}
	if binding.threadID != "" {
		t.Errorf("fresh binding.threadID = %q, want empty (pending)", binding.threadID)
	}
	// initState is occupied by this frame (adopt reservation).
	if pendingCount(h.backend) != 1 {
		t.Errorf("initState occupancy = %d, want 1", pendingCount(h.backend))
	}
	// Argv must be `codex --remote unix://<sock>` with NO `resume`.
	wantArgv := libcodex.RemoteAttachArgs(listen, "", "/repo", "", "")
	if !reflect.DeepEqual(res.Plan.Argv, wantArgv) {
		t.Fatalf("Argv = %#v, want %#v", res.Plan.Argv, wantArgv)
	}
	if res.Plan.Command != "" {
		t.Errorf("legacy Command must be empty in argv path: %q", res.Plan.Command)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if strings.Contains(joined, " resume ") {
		t.Errorf("fresh cold-start argv must not contain `resume`: %q", joined)
	}
}

func TestBackendBindFrame_ContainerTrustsResolvedWorkingDirectoryBeforeAttach(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-trust.sock"
	h := newBoundBackend(t, listen)
	h.backend.sandboxed = true
	worktree := "/repo/.agent-grid/worktrees/feature"

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f-trust",
		Plan:    state.LaunchPlan{StartDir: worktree},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	wantArgv := libcodex.RemoteAttachArgs(listen, "", worktree, "", "")
	if !reflect.DeepEqual(res.Plan.Argv, wantArgv) {
		t.Fatalf("Argv = %#v, want %#v", res.Plan.Argv, wantArgv)
	}
	wantPre := [][]string{{agentlaunch.ContainerBinaryPath, "codex-trust-project"}}
	if !reflect.DeepEqual(res.Plan.PreCommands, wantPre) {
		t.Fatalf("PreCommands = %#v, want %#v", res.Plan.PreCommands, wantPre)
	}
	if res.Plan.Command != "" {
		t.Errorf("legacy Command must be empty in argv path: %q", res.Plan.Command)
	}
}

func TestBackendBindFrame_HostDoesNotWriteContainerCodexTrust(t *testing.T) {
	const listen = "/tmp/codex-host.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f-host",
		Plan:    state.LaunchPlan{StartDir: "/repo"},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	if len(res.Plan.PreCommands) != 0 {
		t.Fatalf("host PreCommands must not include codex-trust-project: %#v", res.Plan.PreCommands)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if strings.Contains(joined, "codex-trust-project") {
		t.Fatalf("host argv must not modify container Codex config: %q", joined)
	}
}

func TestBackendBindFrame_Recovery_BindsThreadDirectly(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-sess2.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget:       state.ResumeTarget{ThreadID: "thread-abc"},
			ColdStartSessionID: "019e727e-fde4-7432-9036-ae6604ce1b27",
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	// Recovery path: threadID is already known → registered synchronously,
	// initState is NOT consumed (adopt not needed for known thread).
	h.backend.mu.Lock()
	binding := h.backend.frames["f1"]
	reverse := h.backend.threads["thread-abc"]
	h.backend.mu.Unlock()
	if binding == nil || binding.threadID != "thread-abc" {
		t.Fatalf("recovery binding = %+v, want threadID=thread-abc", binding)
	}
	if reverse != "f1" {
		t.Errorf("b.threads[thread-abc] = %q, want f1", reverse)
	}
	if pendingCount(h.backend) != 0 {
		t.Errorf("initState occupancy = %d, want 0 (recovery bypasses semaphore)", pendingCount(h.backend))
	}
	h.server.mu.Lock()
	lastResume := h.server.lastResume
	h.server.mu.Unlock()
	if got, _ := lastResume["threadId"].(string); got != "thread-abc" {
		t.Fatalf("backend observer thread/resume id = %q, want thread-abc (params=%v)", got, lastResume)
	}
	// Argv must contain `resume thread-abc`.
	wantArgv := libcodex.RemoteAttachArgs(listen, "thread-abc", "/repo", "", "")
	if !reflect.DeepEqual(res.Plan.Argv, wantArgv) {
		t.Fatalf("Argv = %#v, want %#v", res.Plan.Argv, wantArgv)
	}
	if res.Plan.Stream.ColdStartSessionID != "019e727e-fde4-7432-9036-ae6604ce1b27" {
		t.Errorf("ColdStartSessionID = %q, want passthrough of caller value", res.Plan.Stream.ColdStartSessionID)
	}
}

func TestBackendBindFrame_RecoveryRestoresModelEffortIntoAttachCommand(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-sess-model.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{
			Command:  "codex --model gpt-5-codex --effort high",
			StartDir: "/repo",
			Stream: state.StreamLaunchOptions{
				ResumeTarget: state.ResumeTarget{ThreadID: "thread-abc"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if !strings.Contains(joined, "--model gpt-5-codex") {
		t.Fatalf("Argv = %q, want restored --model", joined)
	}
	if !strings.Contains(joined, "--effort high") {
		t.Fatalf("Argv = %q, want restored --effort", joined)
	}
}

func TestBackendBindFrame_RecoveryUsesPerFrameSettingsNotAnotherThreadsSettings(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-sess-isolated.sock"
	h := newBoundBackend(t, listen)
	h.backend.mu.Lock()
	h.backend.frames["other"] = &frameBinding{frameID: "other", threadID: "thread-other", model: "gpt-4.1", effort: "low"}
	h.backend.threads["thread-other"] = "other"
	h.backend.mu.Unlock()

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{
			Command:  "codex --model gpt-5-codex --effort high",
			StartDir: "/repo",
			Stream: state.StreamLaunchOptions{
				ResumeTarget: state.ResumeTarget{ThreadID: "thread-abc"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if !strings.Contains(joined, "--model gpt-5-codex") || strings.Contains(joined, "--model gpt-4.1") {
		t.Fatalf("Argv = %q, want recovered thread settings only", joined)
	}
	if !strings.Contains(joined, "--effort high") || strings.Contains(joined, "--effort low") {
		t.Fatalf("Argv = %q, want recovered thread effort only", joined)
	}
}

func TestBackendBindFrame_RecoveryTranslatesHostRolloutPathForContainer(t *testing.T) {
	const listen = "/opt/agent-grid/run/codex-sess3.sock"
	h := newBoundBackend(t, listen)
	h.backend.mounts = pathmap.Mounts{{Host: "/host/work", Container: "/work"}}
	h.backend.sandboxed = true

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget: state.ResumeTarget{
				ThreadID:    "thread-xyz",
				RolloutPath: "/host/work/.codex/rollout.jsonl",
			},
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	// The container-relative rollout path must appear in the returned
	// ResumeTarget so the CLI (which runs in-container) can find the file.
	if res.Plan.Stream.ResumeTarget.RolloutPath != "/work/.codex/rollout.jsonl" {
		t.Errorf("translated RolloutPath = %q, want /work/.codex/rollout.jsonl",
			res.Plan.Stream.ResumeTarget.RolloutPath)
	}
	if res.Plan.Stream.ResumeTarget.ThreadID != "thread-xyz" {
		t.Errorf("ResumeTarget.ThreadID = %q, want thread-xyz", res.Plan.Stream.ResumeTarget.ThreadID)
	}
}

func TestBackendBindFrame_RolloutOnly_TreatedAsFresh(t *testing.T) {
	// A ResumeTarget with only RolloutPath (no ThreadID) cannot drive
	// `codex resume <id> --remote` — the CLI requires an id. The backend
	// therefore treats such inputs as a fresh cold-start (adopt path). This
	// pins that behaviour so callers that rely on ThreadID-derived rollout
	// resolution (see codex_resume.go's resolveCodexRolloutPath) know they
	// must populate ThreadID upstream.
	const listen = "/opt/agent-grid/run/codex-sess4.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget: state.ResumeTarget{RolloutPath: writeRollout(t)},
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if strings.Contains(joined, " resume ") {
		t.Errorf("rollout-only fell back to fresh, but Argv still has `resume`: %q", joined)
	}
	if pendingCount(h.backend) != 1 {
		t.Errorf("rollout-only should acquire initState (fresh fallback); count=%d", pendingCount(h.backend))
	}
}

// === Start: spawn → dial → initialize (helper sub-process) ===

func newHelperBackend(t *testing.T, mode string) *Backend {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "codex-x.sock")
	return New(&fakeRuntime{}, nil, "sid", "sess1", "/p",
		os.Args[0], []string{"--mode", mode}, "", "", false, false,
		sock, 3*time.Second)
}

func TestBackendStartDialsAndInitializes(t *testing.T) {
	b := newHelperBackend(t, "ok")
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown) })
	if b.conn == nil {
		t.Error("conn not established after successful Start")
	}
}

// TestBackendStartReapsOnInitializeFailure pins the e41ab1c robustness fix: when
// the app-server dials successfully but rejects `initialize`, Start must surface
// the error (after reaping the process) rather than leaving it orphaned.
func TestBackendStartReapsOnInitializeFailure(t *testing.T) {
	b := newHelperBackend(t, "initfail")
	err := b.Start(context.Background())
	if err == nil {
		t.Fatal("Start must fail when the app-server rejects initialize")
	}
	if !strings.Contains(err.Error(), codexschema.MethodInitialize) {
		t.Errorf("error should come from the initialize handshake (dial succeeded), got: %v", err)
	}
}
