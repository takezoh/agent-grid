package stream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
)

type fakeRuntime struct {
	events []state.Event
}

func (f *fakeRuntime) Enqueue(e state.Event) { f.events = append(f.events, e) }

type cleanupDispatcher struct {
	argv         []string
	cleanupCalls atomic.Int32
	cleanupErr   error
}

func (d *cleanupDispatcher) Wrap(_ context.Context, _ string, plan agentlaunch.LaunchPlan) (agentlaunch.WrappedLaunch, error) {
	argv := d.argv
	if len(argv) == 0 {
		argv = plan.Argv
	}
	return agentlaunch.WrappedLaunch{
		Argv:     argv,
		StartDir: plan.StartDir,
		Cleanup: func(context.Context) error {
			d.cleanupCalls.Add(1)
			return d.cleanupErr
		},
	}, nil
}

func (d *cleanupDispatcher) AdoptFrame(context.Context, string, string) (func(context.Context) error, []agentlaunch.Mount, error) {
	return nil, nil, nil
}

func (d *cleanupDispatcher) EnsureProject(context.Context, string) error { return nil }

func (d *cleanupDispatcher) IsContainer(string) bool { return true }

type capturePlanDispatcher struct {
	capturedArgv []string
	capturedPlan agentlaunch.LaunchPlan
	forwardArgv  []string
}

func (d *capturePlanDispatcher) Wrap(_ context.Context, _ string, plan agentlaunch.LaunchPlan) (agentlaunch.WrappedLaunch, error) {
	d.capturedArgv = append([]string(nil), plan.Argv...)
	d.capturedPlan = plan
	argv := d.forwardArgv
	if len(argv) == 0 {
		argv = plan.Argv
	}
	return agentlaunch.WrappedLaunch{Argv: argv, StartDir: plan.StartDir}, nil
}

func (d *capturePlanDispatcher) AdoptFrame(context.Context, string, string) (func(context.Context) error, []agentlaunch.Mount, error) {
	return nil, nil, nil
}

func (d *capturePlanDispatcher) EnsureProject(context.Context, string) error { return nil }

func (d *capturePlanDispatcher) IsContainer(string) bool { return true }

func TestStopBeforeStartIsNoop(t *testing.T) {
	b, _ := newTestBackend()
	// Never Started: cancel and done are nil. Stop must not panic or block.
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)
}

func TestWaitProcessIntentionalShutdownDoesNotFailFrames(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	close(b.done)
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return context.Canceled }}
	b.waitProcess()
	if len(rt.events) != 0 {
		t.Fatalf("intentional Stop emitted subsystem failures: %#v", rt.events)
	}
}

func TestWaitProcessUnexpectedExitFailsEachBoundFrameOnce(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return errors.New("boom") }}

	b.waitProcess()
	if len(rt.events) != 1 {
		t.Fatalf("unexpected Wait events = %d, want one per bound frame: %#v", len(rt.events), rt.events)
	}
	event, ok := rt.events[0].(state.EvSubsystem)
	if !ok || event.FrameID != "f1" || event.Kind != state.SubsystemFailed {
		t.Fatalf("unexpected Wait event = %#v", rt.events[0])
	}
}

func TestWaitProcessWinsTerminalRaceAndLaterStopCannotReclassifyExit(t *testing.T) {
	b, rt := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.done = make(chan struct{})
	b.spawnRes = agentlaunch.SpawnResult{Wait: func() error { return errors.New("boom") }}

	b.waitProcess()
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	b.mu.Lock()
	observed, expected, cause := b.terminalObserved, b.terminalExpected, b.terminalCause
	b.mu.Unlock()
	if !observed || expected || cause != 0 {
		t.Fatalf("terminal observation = (observed=%v expected=%v cause=%v), want first unexpected observation retained",
			observed, expected, cause)
	}
	if len(rt.events) != 1 {
		t.Fatalf("later Stop reclassified or duplicated unexpected exit events: %#v", rt.events)
	}
}

func TestStopCancelsAndWaitsForReap(t *testing.T) {
	b, _ := newTestBackend()
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.done = make(chan struct{})
	// Emulate waitProcess: closes done once the subsystem ctx is cancelled.
	go func() {
		<-b.ctx.Done()
		close(b.done)
	}()

	start := time.Now()
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)
	if elapsed := time.Since(start); elapsed >= stopGrace {
		t.Fatalf("Stop blocked %v (>= grace %v); did not observe reap", elapsed, stopGrace)
	}
	select {
	case <-b.done:
	default:
		t.Fatal("Stop returned before done was closed")
	}
}

func TestStopRunsAppServerCleanupOnce(t *testing.T) {
	b, _ := newTestBackend()
	dispatcher := &cleanupDispatcher{}
	b.dispatcher = dispatcher
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.done = make(chan struct{})
	wrapped, err := dispatcher.Wrap(context.Background(), "", agentlaunch.LaunchPlan{})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	b.setSpawnCleanup(wrapped.Cleanup)
	go func() {
		<-b.ctx.Done()
		close(b.done)
	}()

	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	if got := dispatcher.cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestStopLogsAndSuppressesCleanupError(t *testing.T) {
	b, _ := newTestBackend()
	dispatcher := &cleanupDispatcher{cleanupErr: errors.New("cleanup failed")}
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.done = make(chan struct{})
	wrapped, err := dispatcher.Wrap(context.Background(), "", agentlaunch.LaunchPlan{})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	b.setSpawnCleanup(wrapped.Cleanup)
	close(b.done)

	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	if got := dispatcher.cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestSpawnServerRunsCleanupWhenProcessStartFails(t *testing.T) {
	b, _ := newTestBackend()
	dispatcher := &cleanupDispatcher{argv: []string{filepath.Join(t.TempDir(), "missing-app-server")}}
	b.dispatcher = dispatcher
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.cancel()

	_, _, err := b.spawnServer(context.Background())
	if err == nil {
		t.Fatal("spawnServer must fail for a missing app-server binary")
	}
	if got := dispatcher.cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestShimArgs_UsesContainerBridgePathInDevcontainer(t *testing.T) {
	b, _ := newTestBackend()
	b.helperBin = "/host/bin/bridge"
	b.isContainer = true
	b.listenSock = "/opt/agent-grid/run/codex.sock"
	b.serverBin = "codex"
	args := b.shimArgs()
	if len(args) == 0 {
		t.Fatal("shimArgs returned empty argv")
	}
	if got := args[0]; got != agentlaunch.ContainerBinaryPath {
		t.Fatalf("shim argv[0] = %q, want %q", got, agentlaunch.ContainerBinaryPath)
	}
}

func TestShimArgs_UsesHostHelperPathOutsideContainer(t *testing.T) {
	b, _ := newTestBackend()
	b.helperBin = "/host/bin/bridge"
	b.isContainer = false
	b.listenSock = "/tmp/codex.sock"
	b.serverBin = "codex"
	args := b.shimArgs()
	if len(args) == 0 {
		t.Fatal("shimArgs returned empty argv")
	}
	if got := args[0]; got != "/host/bin/bridge" {
		t.Fatalf("shim argv[0] = %q, want host helper path", got)
	}
}

func TestSpawnServer_UsesShimInContainerWithoutHostHelper(t *testing.T) {
	b, _ := newTestBackend()
	b.isContainer = true
	b.helperBin = ""
	b.listenSock = "/opt/agent-grid/run/codex.sock"
	b.serverBin = "codex"
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.cancel()

	dispatcher := &capturePlanDispatcher{
		forwardArgv: []string{filepath.Join(t.TempDir(), "missing-app-server")},
	}
	b.dispatcher = dispatcher

	_, _, err := b.spawnServer(context.Background())
	if err == nil {
		t.Fatal("spawnServer must fail for a missing shim binary in test dispatcher")
	}
	if len(dispatcher.capturedArgv) == 0 {
		t.Fatal("dispatcher did not capture argv")
	}
	if got := dispatcher.capturedArgv[0]; got != agentlaunch.ContainerBinaryPath {
		t.Fatalf("captured argv[0] = %q, want %q", got, agentlaunch.ContainerBinaryPath)
	}
}

func TestStartRunsCleanupWhenInitializeFails(t *testing.T) {
	dispatcher := &cleanupDispatcher{}
	sock := filepath.Join(t.TempDir(), "codex-x.sock")
	b := New(&fakeRuntime{}, dispatcher, "sid", "sess1", "/p",
		os.Args[0], []string{"--mode", "initfail"}, "", "", false, false,
		sock, 3*time.Second)

	err := b.Start(context.Background())
	if err == nil {
		t.Fatal("Start must fail when initialize fails")
	}
	if got := dispatcher.cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestStartThenStopRunsRegisteredCleanup(t *testing.T) {
	dispatcher := &cleanupDispatcher{}
	sock := filepath.Join(t.TempDir(), "codex-x.sock")
	b := New(&fakeRuntime{}, dispatcher, "sid", "sess1", "/p",
		os.Args[0], []string{"--mode", "ok"}, "", "", false, false,
		sock, 3*time.Second)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.Stop(context.Background(), subsystem.StopCauseRuntimeShutdown)

	if got := dispatcher.cleanupCalls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestBackendKind(t *testing.T) {
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", "", false, false, "/sock", 0)
	if b.Kind() != state.LaunchSubsystemStream {
		t.Errorf("Kind = %v", b.Kind())
	}
}

func TestReleaseFrameAndLookup(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	if got := b.frameForThread("t1"); got != "f1" {
		t.Errorf("frameForThread = %q", got)
	}
	if got := b.frameForThread(""); got != "" {
		t.Errorf("empty threadID should return empty FrameID, got %q", got)
	}
	if got := b.frameForThread("unknown"); got != "" {
		t.Errorf("unknown should return empty, got %q", got)
	}

	b.ReleaseFrame("f1")
	b.mu.Lock()
	_, frameOK := b.frames["f1"]
	_, threadOK := b.threads["t1"]
	b.mu.Unlock()
	if frameOK || threadOK {
		t.Errorf("ReleaseFrame did not clean up: frames=%v threads=%v", frameOK, threadOK)
	}

	// idempotent
	b.ReleaseFrame("nonexistent")
}

func TestResolveDialSock(t *testing.T) {
	t.Run("host mode (no mounts) dials the listen path", func(t *testing.T) {
		const listen = "/host/run/codex/codex-x.sock"
		if got := resolveDialSock(listen, agentlaunch.WrappedLaunch{}); got != listen {
			t.Errorf("resolveDialSock() = %q, want %q", got, listen)
		}
	})

	t.Run("container mode maps the listen path to its bind-mount host path", func(t *testing.T) {
		got := resolveDialSock("/opt/agent-grid/run/codex-x.sock", agentlaunch.WrappedLaunch{
			Mounts: []agentlaunch.Mount{{Host: "/home/u/.agent-grid/run/4342aed7adbf", Container: "/opt/agent-grid/run"}},
		})
		if want := "/home/u/.agent-grid/run/4342aed7adbf/codex-x.sock"; got != want {
			t.Errorf("resolveDialSock() = %q, want %q", got, want)
		}
	})

	t.Run("unmapped listen path falls back unchanged", func(t *testing.T) {
		const listen = "/elsewhere/codex-x.sock"
		got := resolveDialSock(listen, agentlaunch.WrappedLaunch{
			Mounts: []agentlaunch.Mount{{Host: "/h/run", Container: "/opt/agent-grid/run"}},
		})
		if got != listen {
			t.Errorf("resolveDialSock() = %q, want %q", got, listen)
		}
	})
}

func TestFactoryRange(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	f.backends["a"] = &Backend{subsystemID: "a"}
	f.backends["b"] = &Backend{subsystemID: "b"}
	seen := map[state.SubsystemID]bool{}
	f.Range(func(b *Backend) bool {
		seen[b.subsystemID] = true
		return true
	})
	if len(seen) != 2 {
		t.Errorf("Range visited %d, want 2", len(seen))
	}
	// early termination
	count := 0
	f.Range(func(*Backend) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("early-stop visited %d, want 1", count)
	}
}
