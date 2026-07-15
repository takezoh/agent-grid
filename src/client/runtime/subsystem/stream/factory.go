package stream

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-grid/platform/lib/codex"
	"github.com/takezoh/agent-grid/platform/procgroup"
)

// FactoryConfig holds runtime-supplied dependencies the Stream Factory needs
// to instantiate Backends. The runtime is responsible for resolving paths
// and capabilities — the Factory itself encapsulates environment knowledge
// (host vs container) within the SubsystemID it constructs.
type FactoryConfig struct {
	// Runtime is the hook used by Backends to enqueue events.
	Runtime RuntimeHook
	// Dispatcher applies sandbox/container wrapping to each app-server launch.
	// Nil falls back to a direct (no-op) dispatch.
	Dispatcher agentlaunch.Dispatcher
	// ResolveSockPath returns the UDS path the per-session app-server binds
	// for the given launch. When useContainer is true the callback returns a
	// container-absolute path under ContainerRunDir and the backend derives the
	// host dial path from the launch's bind mounts; when false the callback
	// returns a host-absolute path in the daemon's data dir so both the
	// app-server bind and the daemon dial hit the same host inode. Paths are
	// unique per session to allow multiple concurrent app-server processes.
	// useContainer is passed explicitly (rather than re-derived from project)
	// so a per-launch SandboxOverride flips the routing without the callback
	// having to re-consult stale project metadata.
	ResolveSockPath func(sessionID state.SessionID, project string, useContainer bool) (listen string, err error)
	// IsContainer reports whether the given project's DEFAULT sandbox mode is
	// devcontainer. This is a project-scoped hint only — Factory.Ensure ANDs
	// it with the per-launch plan.Sandbox before deciding whether the
	// app-server actually runs inside the container (see the useContainer
	// computation there).
	IsContainer func(project string) bool
	// ResolveDriverBin returns the absolute path to the codex driver's
	// binary, or a non-nil error whose message names the config key an
	// operator can set to fix a missing-binary scenario. Factory.Ensure
	// calls it once per session-create so a bare-name PATH-lookup failure
	// surfaces as a fast, actionable session-create error instead of a
	// silent 15-second WebSocket dial timeout that ultimately traces to
	// the bridge shim's ENOENT on `exec("codex")`. Nil is treated as a
	// pure `exec.LookPath(driverName)` fallback for backward compatibility.
	ResolveDriverBin func(driverName string) (string, error)
	// ReadTimeout overrides the per-request JSON-RPC timeout.  Zero uses the
	// default (15 seconds).  Corresponds to the codex.read_timeout_ms config key.
	ReadTimeout time.Duration
	// HelperBinaryPath resolves helper binaries such as the bridge shim that
	// fronts Codex app-server dynamicTools.
	HelperBinaryPath func(name string) (string, error)
	// Tracker records the app-server process-group pgids so a future boot can
	// reap them if this daemon dies without a graceful Stop.
	// Nil disables crash-path tracking (e.g. tests, non-Linux).
	Tracker *procgroup.Tracker
}

// Factory creates Stream Backends keyed by session. One Backend (= one
// app-server process) exists per client Session. All frames (root + pushed
// frames) in the same Session share one Backend; different Sessions get
// separate Backends.
type Factory struct {
	cfg      FactoryConfig
	mu       sync.Mutex
	backends map[state.SubsystemID]*Backend
}

// NewFactory constructs a Stream Factory.
func NewFactory(cfg FactoryConfig) *Factory {
	return &Factory{cfg: cfg, backends: make(map[state.SubsystemID]*Backend)}
}

// Ensure implements subsystem.Factory.
func (f *Factory) Ensure(ctx context.Context, sessionID state.SessionID, project string, plan state.LaunchPlan) (subsystem.Subsystem, state.SubsystemID, error) {
	argv, err := agentlaunch.SplitArgs(plan.Command)
	if err != nil {
		return nil, "", fmt.Errorf("stream factory: parse command: %w", err)
	}
	cmdCfg, err := libcodex.ParseCommand(argv)
	if err != nil {
		return nil, "", err
	}
	// Resolve the codex driver binary to an absolute path BEFORE spawning
	// anything downstream. cmdCfg.ServerBin is the bare driver name at this
	// point (libcodex.ParseCommand pins argv[0] == DriverName); replacing
	// it with the resolved absolute path here ensures both the app-server
	// spawn (spawnServer → shimArgs / AppServerListenArgs) and the frame
	// attach argv (Backend.BindFrame → RemoteAttachArgs) hand exec /
	// exec.LookPath a slash-containing path, which bypasses PATH search
	// entirely and closes the daemon-launched-process ENOENT class.
	if f.cfg.ResolveDriverBin != nil {
		resolvedBin, resolveErr := f.cfg.ResolveDriverBin(cmdCfg.ServerBin)
		if resolveErr != nil {
			return nil, "", fmt.Errorf("stream factory: resolve driver bin: %w", resolveErr)
		}
		cmdCfg.ServerBin = resolvedBin
	}
	id := f.makeID(sessionID)

	f.mu.Lock()
	if b, ok := f.backends[id]; ok {
		f.mu.Unlock()
		return b, id, nil
	}
	f.mu.Unlock()

	// useContainer collapses the two axes that drive app-server placement into
	// a single bool: (a) the project's default sandbox mode from IsContainer,
	// and (b) the per-launch state.SandboxOverride from plan.Sandbox. Every
	// downstream site — listenSock resolution, shim selection, dispatcher
	// routing — reads this ONE value so a host override cannot silently split
	// the frame and app-server across host / container namespaces.
	useContainer := plan.Sandbox != state.SandboxOverrideHost
	if f.cfg.IsContainer != nil {
		useContainer = useContainer && f.cfg.IsContainer(project)
	} else {
		useContainer = false
	}

	listen, err := f.cfg.ResolveSockPath(sessionID, project, useContainer)
	if err != nil {
		return nil, "", fmt.Errorf("stream factory: resolve sock path: %w", err)
	}
	helperBin := ""
	if f.cfg.HelperBinaryPath != nil {
		if candidate, resolveErr := f.cfg.HelperBinaryPath("bridge"); resolveErr == nil {
			if _, statErr := os.Stat(candidate); statErr == nil {
				helperBin = candidate
			}
		}
	}
	b := New(
		f.cfg.Runtime,
		f.cfg.Dispatcher,
		id,
		sessionID,
		project,
		cmdCfg.ServerBin,
		cmdCfg.ServerArgs,
		cmdCfg.Model,
		cmdCfg.Effort,
		plan.Stream.SandboxPolicy == state.StreamSandboxPolicyExternal,
		plan.Stream.ApprovalPolicy == state.StreamApprovalPolicyAutoApprove,
		listen,
		f.cfg.ReadTimeout,
	)
	b.helperBin = helperBin
	b.isContainer = useContainer
	b.tracker = f.cfg.Tracker

	f.mu.Lock()
	if existing, ok := f.backends[id]; ok {
		f.mu.Unlock()
		return existing, id, nil
	}
	f.backends[id] = b
	f.mu.Unlock()

	if err := b.Start(ctx); err != nil {
		f.mu.Lock()
		delete(f.backends, id)
		f.mu.Unlock()
		return nil, "", err
	}
	return b, id, nil
}

func (f *Factory) FindFrameByThread(sessionID state.SessionID, threadID string) (state.FrameID, bool) {
	f.mu.Lock()
	b := f.backends[f.makeID(sessionID)]
	f.mu.Unlock()
	if b == nil {
		return "", false
	}
	if frameID := b.FrameForThread(threadID); frameID != "" {
		return frameID, true
	}
	return "", false
}

// Remove implements subsystem.Reaper. It stops the backend for the given
// subsystemID and removes it from the factory. Called when a session's last
// frame is released.
func (f *Factory) Remove(ctx context.Context, id state.SubsystemID) {
	f.mu.Lock()
	b, ok := f.backends[id]
	if ok {
		delete(f.backends, id)
	}
	f.mu.Unlock()
	if ok {
		b.Stop(ctx)
	}
}

// Range iterates all live backends. Used by the runtime for shutdown.
func (f *Factory) Range(fn func(*Backend) bool) {
	f.mu.Lock()
	snapshot := make([]*Backend, 0, len(f.backends))
	for _, b := range f.backends {
		snapshot = append(snapshot, b)
	}
	f.mu.Unlock()
	for _, b := range snapshot {
		if !fn(b) {
			return
		}
	}
}

// makeID derives the SubsystemID from the session identifier.
// Every client Session gets its own app-server, so the ID is keyed purely on
// sessionID. All frames (root + pushed frames) within the same session share
// the same ID and therefore the same Backend.
func (f *Factory) makeID(sessionID state.SessionID) state.SubsystemID {
	return state.SubsystemID("stream:session:" + string(sessionID))
}
