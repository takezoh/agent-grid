// Package stream implements the stream subsystem backend that fronts
// structured app-servers (currently codex app-server) via WebSocket-over-UDS.
// This is the only location in runtime/ permitted to import driver/<tool>.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-grid/platform/lib/codex"
	"github.com/takezoh/agent-grid/platform/pathmap"
	"github.com/takezoh/agent-grid/platform/procgroup"
)

const (
	serverDialTimeout   = 15 * time.Second
	resumePhasePending  = "resume_pending"
	resumePhaseAttached = "attached"

	// stopGrace bounds how long Stop waits for the read loop + process Wait to
	// finish after cancelling. A little above procgroup's WaitDelay so the
	// SIGKILL'd group has time to be reaped before Stop returns.
	stopGrace = procgroup.DefaultWaitDelay + time.Second

	// initAcquireTimeout bounds how long a fresh (non-resume) BindFrame will
	// wait for the previous pending frame's adopt to complete before failing.
	// Interactive CLI startup is typically <1s; 60s comfortably accommodates
	// slow container starts without leaving the caller stuck forever.
	initAcquireTimeout = 60 * time.Second

	// initAdoptDeadline is written into a pendingSlot when it is acquired.
	// If handleThreadStarted does not consume the slot within this window,
	// reapExpiredSlots discards it and removes the orphan frameBinding —
	// covering the case where the spawned CLI crashes before issuing
	// `thread/start` and therefore no adopt or ReleaseFrame ever fires.
	initAdoptDeadline = 60 * time.Second

	// reapInterval is how often reapExpiredSlots wakes to sweep the initState.
	// Set to (initAdoptDeadline / 6) so an expired slot is reclaimed within a
	// small multiple of its deadline without polling excessively.
	reapInterval = 10 * time.Second
)

// RuntimeHook is implemented by *runtime.Runtime and lets the stream backend
// enqueue events without a circular import.
type RuntimeHook interface {
	Enqueue(event state.Event)
}

// Backend is the codex app-server stream subsystem. One instance exists per
// client Session. It manages the per-session app-server process, the
// WebSocket-over-UDS connection, and per-frame thread bindings.
//
// Thread ownership: the codex CLI (`codex --remote` or `codex resume <id>
// --remote`) always creates or resumes threads on its own connection. The
// Backend is a passive router — it never calls `thread/start` itself. Fresh
// interactive frames are adopted when `thread/started` arrives on the
// broadcast channel; the initState reservation keeps at most one adopt candidate
// pending at a time so the incoming thread has an unambiguous owner. See
// docs/adr/0081-codex-frame-init-serialize.md.
type Backend struct {
	runtime      RuntimeHook
	dispatcher   agentlaunch.Dispatcher
	subsystemID  state.SubsystemID
	sessionID    state.SessionID
	project      string
	serverBin    string
	serverArgs   []string
	helperBin    string
	isContainer  bool
	sandboxed    bool
	autoApprove  bool
	readTimeout  time.Duration
	ctx          context.Context    // subsystem-scoped; child of the daemon ctx
	cancel       context.CancelFunc // cancels ctx → reaps read loop + process group
	done         chan struct{}      // closed when waitProcess returns (process reaped)
	tracker      *procgroup.Tracker // records pgids for crash-path reaping; may be nil
	spawnRes     agentlaunch.SpawnResult
	spawnCleanup func(context.Context) error
	spawnCleaned sync.Once
	conn         *codexclient.Conn
	listenSock   string // UDS path the app-server binds (container-absolute under a devcontainer)
	dialSock     string // host-side UDS path the daemon dials; resolved from listenSock + bind mounts in spawnServer
	mounts       pathmap.Mounts
	mu           sync.Mutex
	frames       map[state.FrameID]*frameBinding
	threads      map[string]state.FrameID
	// initState serializes fresh (non-resume) frame init. Holds a single
	// pending slot at a time (see initsem.go). Replaces the earlier
	// chan pendingSlot design whose drain-check-put-back non-atomicity was
	// the root cause of race chains around reaper / adopt / release.
	initState *initState
}

type frameBinding struct {
	frameID         state.FrameID
	startDir        string
	worktreePath    string // non-empty when a managed worktree was adopted or created
	threadID        string
	sessionID       string
	rolloutPath     string
	requestedID     string
	observedID      string
	resumePhase     string
	threadStatus    string
	waitApproval    bool
	activeTurnID    string
	turnAssistant   string
	lastAssistant   string
	model           string
	modelSet        bool
	effort          string
	effortSet       bool
	history         []state.SubsystemTurn
	failureReported bool
}

// New constructs a Backend. Call Start before calling BindFrame.
func New(
	rt RuntimeHook,
	dispatcher agentlaunch.Dispatcher,
	subsystemID state.SubsystemID,
	sessionID state.SessionID,
	project, serverBin string,
	serverArgs []string,
	_, _ string,
	sandboxed, autoApprove bool,
	listenSock string,
	readTimeout time.Duration,
) *Backend {
	return &Backend{
		runtime:     rt,
		dispatcher:  dispatcher,
		subsystemID: subsystemID,
		sessionID:   sessionID,
		project:     project,
		serverBin:   serverBin,
		serverArgs:  serverArgs,
		sandboxed:   sandboxed,
		autoApprove: autoApprove,
		readTimeout: readTimeout,
		listenSock:  listenSock,
		frames:      map[state.FrameID]*frameBinding{},
		threads:     map[string]state.FrameID{},
		initState:   newInitState(),
	}
}

func (b *Backend) FrameForThread(threadID string) state.FrameID {
	return b.frameForThread(threadID)
}

// Kind implements subsystem.Subsystem.
func (b *Backend) Kind() state.LaunchSubsystem { return state.LaunchSubsystemStream }

// Start launches the app-server process, dials the WebSocket, and begins
// the read loop. On failure the caller must not call Start again — create
// a new Backend instead.
func (b *Backend) Start(ctx context.Context) error {
	// Derive a subsystem-scoped context from the daemon context. Cancelling it
	// (via Stop, or daemon shutdown cascading from the parent) tears down the
	// read loop and SIGKILLs the app-server process group.
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.done = make(chan struct{})

	res, serverErr, err := b.spawnServer(ctx)
	if err != nil {
		b.cancel()
		return err
	}

	t, err := codexclient.DialUDS(b.dialSock, serverDialTimeout)
	if err != nil {
		b.cancel()
		// Reap the process first: cmd.Wait blocks until the stderr copier
		// goroutine has flushed everything into serverErr, so the captured
		// output reflects what the app-server printed before it died.
		_ = res.Wait()
		b.cleanupSpawn(context.Background())
		slog.Error("stream backend: app-server dial failed",
			"subsystem", b.subsystemID, "sock", b.dialSock,
			"stderr", strings.TrimSpace(serverErr.String()))
		return err
	}
	b.spawnRes = res
	// The app-server speaks JSON-RPC over the UDS; its stdout pipe is never
	// read. Drain it to discard so a chatty app-server can't block on a full
	// stdout pipe. Ends when the process exits and Wait closes the pipe.
	go func() { _, _ = io.Copy(io.Discard, res.Stdout) }()
	b.conn = codexclient.NewConn(t, b.readTimeout)
	go func() {
		if err := b.conn.Run(b.ctx, b); err != nil {
			slog.Debug("stream backend: read loop closed", "subsystem", b.subsystemID, "err", err)
		}
	}()
	if err := codexclient.Initialize(b.conn); err != nil {
		_ = b.conn.Close()
		b.cancel()
		// Reap the app-server we just SIGKILLed via cancel, mirroring the
		// dial-failure path; otherwise the process is orphaned (waitProcess
		// is not started on this path and the pgid was never tracked).
		_ = res.Wait()
		b.cleanupSpawn(context.Background())
		return err
	}
	b.trackProcessGroups()
	go b.waitProcess()
	go b.reapExpiredSlots()
	return nil
}

// BindFrame implements subsystem.Subsystem. It resolves the frame's worktree,
// registers a per-frame binding, and rewrites Plan.Command to the CLI attach
// command. The Backend never invokes `thread/start` or `thread/resume`
// itself — the codex CLI owns the thread lifecycle. For cold-start recovery
// (persisted ThreadID present), Backend records the id up front so the
// broadcast thread/started routes straight to this frame. For fresh
// interactive sessions, Backend acquires the initState slot with an empty
// thread id, and handleThreadStarted adopts the CLI-created thread when it
// arrives (see docs/adr/0081-codex-frame-init-serialize.md).
func (b *Backend) BindFrame(ctx context.Context, req subsystem.BindRequest) (subsystem.BindResult, error) {
	result := subsystem.BindResult{Plan: req.Plan}
	startDir := req.Plan.StartDir
	initialModel, initialEffort := attachSettingsFromCommand(req.Plan.Command)

	// Worktree resolution: same logic as CLI backend. Track the newly-created
	// path so any error return after this point removes it — ReleaseFrame is
	// not called on a BindFrame failure and the worktree would otherwise
	// orphan on disk (registerBoundFrame's collision reject, resume-target
	// normalize failure, initState acquire timeout, etc).
	var worktreePath, createdWorktree string
	switch {
	case subsystem.IsManagedWorktreePath(startDir):
		worktreePath = startDir
		result.WorktreeStartDir = startDir
	case req.Plan.Options.Worktree.Enabled:
		names := subsystem.GenerateWorktreeNames(subsystem.WorktreeNameAttempts)
		wt, err := createWorktree(ctx, subsystem.WorktreeInput{
			RepoDir:        startDir,
			CandidateNames: names,
		})
		if err != nil {
			return subsystem.BindResult{}, err
		}
		startDir = wt.StartDir
		worktreePath = wt.StartDir
		createdWorktree = wt.StartDir
		result.Plan.StartDir = startDir
		result.WorktreeStartDir = wt.StartDir
		result.WorktreeName = wt.Name
	}
	bindOK := false
	defer func() {
		if !bindOK && createdWorktree != "" {
			removeWorktree(createdWorktree)
		}
	}()

	// Normalize the resume target (validate + translate rollout path).
	resumeTarget, err := normalizeResumeTarget(req.Plan.Stream.ResumeTarget, b.mounts)
	if err != nil {
		return subsystem.BindResult{}, err
	}
	persistedThreadID := strings.TrimSpace(resumeTarget.rpc.ThreadID)

	if persistedThreadID != "" {
		// Recovery path: id is known up front; register directly and let the
		// CLI attach via `codex resume <id> --remote`. The initState is not
		// touched — no adopt ambiguity because the incoming thread/started
		// carries an id already present in b.threads.
		if err := b.registerBoundFrame(req.FrameID, startDir, worktreePath, persistedThreadID, resumeTarget.rpc.RolloutPath, req.Plan.Stream.ColdStartSessionID); err != nil {
			return subsystem.BindResult{}, err
		}
		b.applyBindingSettings(req.FrameID, initialModel, initialEffort)
	} else {
		// Fresh path: acquire the init slot, register a pending binding
		// (threadID == ""), let the CLI create its own thread. handleThreadStarted
		// will adopt it. If we fail below, release the slot to avoid leaking.
		if err := b.acquirePendingSlot(ctx, req.FrameID); err != nil {
			return subsystem.BindResult{}, err
		}
		b.registerPendingFrame(req.FrameID, startDir, worktreePath, initialModel, initialEffort)
	}
	bindOK = true

	model, effort := b.bindingSettings(req.FrameID)
	result.Plan.Command = strings.Join(libcodex.RemoteAttachArgs(b.listenSock, persistedThreadID, startDir, model, effort), " ")
	result.Plan.Stdin = nil
	result.Plan.Stream.ResumeTarget = resumeTarget.rpc
	// ColdStartSessionID stays as caller provided (may be empty). It will be
	// populated later via SubsystemSessionReady payload in the fresh case;
	// recovery already has it in the persisted state.
	result.Plan.Stream.ColdStartSessionID = strings.TrimSpace(req.Plan.Stream.ColdStartSessionID)
	return result, nil
}

// registerBoundFrame stores a frameBinding with the thread id already known
// (recovery path). Both the frames map and the reverse-lookup threads map are
// populated atomically under b.mu.
//
// Rejects a collision on b.threads[threadID] instead of silently overwriting:
// letting a second frame steal the routing entry would strand every event
// meant for the earlier frame at frameForThread (ADR-0001 routing-isolation
// invariant). Callers should treat this as a hard failure — the two frames
// disagree about session state and there is no way to disambiguate.
func (b *Backend) registerBoundFrame(frameID state.FrameID, startDir, worktreePath, threadID, rolloutPath, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.threads[threadID]; ok && existing != frameID {
		return fmt.Errorf("stream backend: thread %q already bound to frame %q; refusing to rebind to %q",
			threadID, existing, frameID)
	}
	b.frames[frameID] = &frameBinding{
		frameID:      frameID,
		startDir:     startDir,
		worktreePath: worktreePath,
		threadID:     threadID,
		sessionID:    strings.TrimSpace(sessionID),
		rolloutPath:  strings.TrimSpace(rolloutPath),
		requestedID:  threadID,
		observedID:   threadID,
		resumePhase:  resumePhasePending,
	}
	b.threads[threadID] = frameID
	return nil
}

// registerPendingFrame stores a frameBinding with an empty threadID.
// handleThreadStarted's adopt path will fill in the thread id when the CLI
// broadcasts thread/started.
func (b *Backend) registerPendingFrame(frameID state.FrameID, startDir, worktreePath, model, effort string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.frames[frameID] = &frameBinding{
		frameID:      frameID,
		startDir:     startDir,
		worktreePath: worktreePath,
		model:        strings.TrimSpace(model),
		modelSet:     strings.TrimSpace(model) != "",
		effort:       strings.TrimSpace(effort),
		effortSet:    strings.TrimSpace(effort) != "",
	}
}

func (b *Backend) bindingSettings(frameID state.FrameID) (model, effort string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if binding := b.frames[frameID]; binding != nil {
		return binding.model, binding.effort
	}
	return "", ""
}

func (b *Backend) applyBindingSettings(frameID state.FrameID, model, effort string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if binding := b.frames[frameID]; binding != nil {
		if model = strings.TrimSpace(model); model != "" {
			binding.model = model
			binding.modelSet = true
		}
		if effort = strings.TrimSpace(effort); effort != "" {
			binding.effort = effort
			binding.effortSet = true
		}
	}
}

func attachSettingsFromCommand(command string) (model, effort string) {
	argv, err := agentlaunch.SplitArgs(strings.TrimSpace(command))
	if err != nil {
		return "", ""
	}
	cfg, err := libcodex.ParseCommand(argv)
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(cfg.Model), strings.TrimSpace(cfg.Effort)
}

// ReleaseFrame removes the frame from the registry and its thread mapping.
// Also drains the initState slot if this frame was still pending — otherwise
// the next BindFrame would block for no reason (or the reaper would need to
// wait a full deadline before reclaiming). The pending check is done under
// b.mu so a bound frame's release never touches initState (see releaseOwnSlot).
//
// Asynchronously removes the managed worktree if no other frame in this
// backend still references it. Without this the wrap/spawn failure path
// (interpret_spawn.go, bootstrap_coldstart.go) leaves .agent-grid/worktrees/<name>
// and its git worktree metadata on disk indefinitely — a repeat-failing
// launch would accumulate one orphan per retry. Mirrors the CLI backend's
// ReleaseFrame shape (client/runtime/subsystem/cli/backend.go).
func (b *Backend) ReleaseFrame(frameID state.FrameID) {
	b.mu.Lock()
	binding := b.frames[frameID]
	wasPending := binding != nil && binding.threadID == ""
	worktreePath := ""
	if binding != nil {
		worktreePath = binding.worktreePath
	}
	delete(b.frames, frameID)
	if binding != nil && binding.threadID != "" {
		delete(b.threads, binding.threadID)
	}
	stillUsed := false
	if worktreePath != "" {
		for _, other := range b.frames {
			if other.worktreePath == worktreePath {
				stillUsed = true
				break
			}
		}
	}
	b.mu.Unlock()
	if wasPending {
		b.releaseOwnSlot(frameID)
	}
	if worktreePath != "" && !stillUsed {
		removeWorktree(worktreePath)
	}
}

// removeWorktree is a package-level indirection so tests can observe the
// call without waiting on the real async git worktree removal. Production
// wires this to subsystem.RemoveWorktree.
var removeWorktree = subsystem.RemoveWorktree

// createWorktree indirects subsystem.CreateWorktree for the same reason —
// so tests can trigger the "worktree created; later step errors" path
// (validating the bindOK defer) without a real git repo on disk.
var createWorktree = subsystem.CreateWorktree

// Stop cancels the subsystem context (SIGKILLing the app-server process group
// via procgroup) and blocks until waitProcess has reaped it, so the call
// returns only once the spawned process is gone. A grace bound prevents a
// stuck Wait from blocking shutdown forever.
func (b *Backend) Stop(_ context.Context) {
	if b.cancel != nil {
		b.cancel()
	}
	if b.done == nil {
		return
	}
	select {
	case <-b.done:
	case <-time.After(stopGrace):
		slog.Warn("stream backend: Stop timed out waiting for reap", "subsystem", b.subsystemID)
	}
	b.cleanupSpawn(context.Background())
}

func (b *Backend) setSpawnCleanup(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	b.spawnCleanup = fn
}

func (b *Backend) cleanupSpawn(ctx context.Context) {
	if b.spawnCleanup == nil {
		return
	}
	b.spawnCleaned.Do(func() {
		if err := b.spawnCleanup(ctx); err != nil {
			slog.Warn("stream backend: app-server sandbox cleanup failed",
				"subsystem", b.subsystemID, "err", err)
		}
	})
}

// OnNotification implements codexclient.Handler.
func (b *Backend) OnNotification(method string, params json.RawMessage) {
	b.handleNotification(method, params)
}

// OnServerRequest implements codexclient.Handler.
func (b *Backend) OnServerRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	b.handleRequest(id, method, params)
}

func (b *Backend) waitProcess() {
	defer close(b.done)
	err := b.spawnRes.Wait()
	if b.spawnRes.PID != 0 {
		b.tracker.Untrack(b.spawnRes.PID)
	}
	if err != nil {
		slog.Error("stream backend exited", "subsystem", b.subsystemID, "err", err)
	} else {
		slog.Warn("stream backend exited", "subsystem", b.subsystemID)
	}
	_ = b.conn.Close()
	b.mu.Lock()
	frameIDs := make([]state.FrameID, 0, len(b.frames))
	for frameID := range b.frames {
		frameIDs = append(frameIDs, frameID)
	}
	b.mu.Unlock()
	var stopErr error
	if err != nil {
		stopErr = fmt.Errorf("stream backend stopped: %w", err)
	} else {
		stopErr = errors.New("stream backend stopped")
	}
	for _, frameID := range frameIDs {
		b.failFrame(frameID, stopErr)
	}
}

func (b *Backend) frameForThread(threadID string) state.FrameID {
	if threadID == "" {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.threads[threadID]
}

func (b *Backend) emit(frameID state.FrameID, kind state.SubsystemEventKind, payload state.SubsystemPayload) {
	b.runtime.Enqueue(state.EvSubsystem{
		ConnID:    0,
		FrameID:   frameID,
		Source:    state.SubsystemStream,
		Kind:      kind,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}
