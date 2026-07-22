package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	rsubsystem "github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/pathmap"
)

// minimalDriver is a zero-behaviour driver for testing bootstrap paths.
type minimalDriver struct{}

// successfulCleanupResult keeps cleanup callbacks typed as func() error while
// allowing tests to exercise their success path without unparam false positives.
var successfulCleanupResult error

func (minimalDriver) Name() string        { return "minimal-test" }
func (minimalDriver) DisplayName() string { return "minimal-test" }
func (minimalDriver) Status(_ state.DriverState) state.Status {
	return state.StatusIdle
}
func (minimalDriver) NewState(_ time.Time) state.DriverState        { return state.DriverStateBase{} }
func (minimalDriver) Persist(_ state.DriverState) map[string]string { return nil }
func (minimalDriver) Restore(_ map[string]string, _ time.Time) state.DriverState {
	return state.DriverStateBase{}
}
func (minimalDriver) View(_ state.DriverState) state.View { return state.View{} }
func (minimalDriver) Step(prev state.DriverState, _ state.FrameContext, _ state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	return prev, nil, state.View{}
}
func (minimalDriver) PrepareLaunch(_ state.DriverState, _ state.LaunchMode, project, command string, _ state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	return state.LaunchPlan{Command: command, StartDir: project}, nil
}
func (minimalDriver) StartDir(_ state.DriverState) string                          { return "" }
func (minimalDriver) WithStartDir(s state.DriverState, _ string) state.DriverState { return s }

// TestRegisterContainerFrame_warmSaveIsSynchronous guards the 029 F4 fix:
// warm Save runs synchronously inside registerContainerFrame so it cannot
// race a follow-up Delete from executeKillSessionWindow. Before the fix,
// Save was fired off in a goroutine and could win against a kill-path Delete
// for the same frame's warm file, leaving a stale token on disk.
func TestRegisterContainerFrame_warmSaveIsSynchronous(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{Backend: newFakeBackend(), DataDir: dir})
	t.Cleanup(r.shutdownContainerEndpoints)
	if r.warmFrames == nil {
		t.Fatal("warm-frame store not initialised with DataDir set")
	}
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/p",
		Frames: []state.SessionFrame{{ID: "f1", Project: "/p", Command: "shell"}},
	}

	r.registerContainerFrame("f1", "/p", dir, "tok-1", pathmap.Mounts{{Host: "/h", Container: "/c"}})

	// Synchronous contract: by the time registerContainerFrame returns the
	// warm state for the frame must be visible to LoadAll. No sleep, no poll.
	states, err := r.warmFrames.LoadAll()
	if err != nil {
		t.Fatalf("warmFrames.LoadAll: %v", err)
	}
	var got *WarmFrameState
	for i := range states {
		if states[i].FrameID == "f1" {
			got = &states[i]
			break
		}
	}
	if got == nil {
		t.Fatal("warm save did not land synchronously: f1 missing from LoadAll")
		return
	}
	if got.ContainerToken != "tok-1" {
		t.Errorf("warm token = %q, want tok-1", got.ContainerToken)
	}
}

func TestStoreAndInvokeFrameCleanup(t *testing.T) {
	r := New(Config{})

	var called atomic.Bool
	r.storeFrameCleanup("f1", func() error {
		called.Store(true)
		return nil
	})

	r.invokeFrameCleanup("f1")

	// invokeFrameCleanup runs the fn in a goroutine; wait briefly.
	deadline := time.Now().Add(200 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !called.Load() {
		t.Error("cleanup fn was not called after invokeFrameCleanup")
	}

	// Second invoke for same frame should be a no-op (already deleted).
	called.Store(false)
	r.invokeFrameCleanup("f1")
	time.Sleep(20 * time.Millisecond)
	if called.Load() {
		t.Error("cleanup fn called twice for same frame")
	}
}

func TestInvokeFrameCleanup_noopWhenNil(t *testing.T) {
	r := New(Config{})
	// No cleanup registered; must not panic.
	r.invokeFrameCleanup("unknown")
}

func TestDrainFrameCleanups(t *testing.T) {
	r := New(Config{})

	var count atomic.Int32
	for _, id := range []state.FrameID{"f1", "f2", "f3"} {
		r.storeFrameCleanup(id, func() error {
			count.Add(1)
			return nil
		})
	}

	r.drainFrameCleanups()

	if got := count.Load(); got != 3 {
		t.Errorf("drain called %d cleanups, want 3", got)
	}

	// Map must be empty after drain.
	remaining := len(r.sandboxCleanups)
	if remaining != 0 {
		t.Errorf("frameCleanups has %d entries after drain, want 0", remaining)
	}
}

func TestInvokeFrameCleanup_errorLogged(t *testing.T) {
	r := New(Config{})
	r.storeFrameCleanup("ferr", func() error {
		return errors.New("container stop failed")
	})
	// Must not panic; the error is logged internally.
	r.invokeFrameCleanup("ferr")
	time.Sleep(20 * time.Millisecond)
}

func TestDirectLauncher_adoptFrame_noop(t *testing.T) {
	l := DirectLauncher{}
	cleanup, _, err := l.AdoptFrame(context.Background(), state.FrameID("f1"), "/workspace/foo")
	if err != nil {
		t.Fatalf("AdoptFrame returned error: %v", err)
	}
	if cleanup != nil {
		t.Error("DirectLauncher.AdoptFrame should return nil cleanup")
	}
}

// TestCtxCancel_doesNotDrainCleanups verifies that cancelling the runtime
// context (= daemon SIGINT / detach) does not invoke frame cleanup callbacks.
// Containers must survive so backend frames stay alive for warm-restart adoption.
// The explicit shutdown path drains via EffReleaseFrameSandboxes (see
// TestEffReleaseFrameSandboxes_drainsCleanups).
func TestCtxCancel_doesNotDrainCleanups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called atomic.Bool
	r := New(Config{Backend: newFakeBackend()})
	r.storeFrameCleanup("f-shutdown", func() error {
		called.Store(true)
		return nil
	})

	go func() { _ = r.Run(ctx) }()
	cancel()
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop within timeout")
	}

	// Allow a brief window for any async goroutines to run.
	time.Sleep(50 * time.Millisecond)
	if called.Load() {
		t.Error("frame cleanup must NOT be called on ctx cancel (warm-restart requires containers to survive)")
	}
}

// TestEffReleaseFrameSandboxes_drainsCleanups verifies that executing
// EffReleaseFrameSandboxes runs all registered per-frame cleanup closures.
// This is the explicit shutdown path (reduceShutdown emits this effect).
func TestEffReleaseFrameSandboxes_drainsCleanups(t *testing.T) {
	var count atomic.Int32
	r := New(Config{Backend: newFakeBackend()})
	for _, id := range []state.FrameID{"f1", "f2", "f3"} {
		r.storeFrameCleanup(id, func() error {
			count.Add(1)
			return nil
		})
	}

	r.execute(state.EffReleaseFrameSandboxes{})
	deadline := time.Now().Add(time.Second)
	for count.Load() != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("EffReleaseFrameSandboxes called %d cleanups, want 3", got)
	}
}

// TestSpawnFrameWindow_cleanupCalledOnSpawnError verifies that when WrapLaunch
// returns a Cleanup callback but SpawnFrame subsequently fails, the Cleanup is
// still invoked — preventing sandbox resource leaks (ref-count, containers).
func TestSpawnFrameWindow_cleanupCalledOnSpawnError(t *testing.T) {
	saved := state.GetRegistry()
	t.Cleanup(func() {
		state.ClearRegistry()
		for _, d := range saved {
			state.Register(d)
		}
	})
	if _, ok := saved[minimalDriver{}.Name()]; !ok {
		state.Register(minimalDriver{})
	}

	var cleanupCalled atomic.Bool
	fakeLauncher := &testLauncher{
		cleanup: func() error {
			cleanupCalled.Store(true)
			return nil
		},
	}

	backend := newFakeBackend()
	backend.spawnErr = errors.New("backend spawn failed")

	r := New(Config{Backend: backend, Launcher: fakeLauncher})
	frame := state.SessionFrame{
		ID:      "frame-spawn-err",
		Command: "minimal-test",
		Project: "/test/project",
		Driver:  state.DriverStateBase{},
	}

	err := r.spawnFrameWindow("sess-1", state.SandboxOverrideAuto, frame)
	if err == nil {
		t.Fatal("expected error from spawnFrameWindow, got nil")
	}
	if !cleanupCalled.Load() {
		t.Error("Cleanup was not called after SpawnFrame failure")
	}
}

// testLauncher is a WrapLaunch stub that injects a caller-supplied cleanup.
type testLauncher struct {
	cleanup func() error
}

func (l *testLauncher) WrapLaunch(_ state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	return WrappedLaunch{Command: plan.Command, StartDir: plan.StartDir, Env: env, Cleanup: l.cleanup}, nil
}

func (l *testLauncher) AdoptFrame(_ context.Context, _ state.FrameID, _ string) (func() error, pathmap.Mounts, error) {
	return nil, nil, nil
}

func (l *testLauncher) EnsureProject(_ context.Context, _ string) error { return nil }

func (l *testLauncher) IsContainer(_ string) bool { return false }

// TestEffKillSessionWindow_doesNotInvokeCleanup asserts that
// EffKillFrame is responsible for frame-backend kill only
// — sandbox cleanup (Manager.ReleaseFrame → DestroyInstance) is driven
// by EffReleaseFrameSandbox emitted from the reducer for the same frame.
// Splitting the responsibilities lets EvFrameVanished release the
// container even though it skips the frame kill (`killWindow=false`).
func TestEffKillSessionWindow_doesNotInvokeCleanup(t *testing.T) {
	var called atomic.Bool
	backend := noopBackend{}
	r := New(Config{Backend: backend})

	frameID := state.FrameID("f-kill")
	r.storeFrameCleanup(frameID, func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffKillFrame{FrameID: frameID})

	time.Sleep(50 * time.Millisecond)
	if called.Load() {
		t.Error("cleanup must not be called by EffKillFrame alone; expected EffReleaseFrameSandbox to drive cleanup")
	}
}

// TestEffReleaseFrameSandbox_invokesCleanup asserts the new effect fires
// the per-frame cleanup closure (devcontainer.makeCleanup =
// Manager.ReleaseFrame → 0 なら DestroyInstance). reducer emits this
// effect for every evicted frame regardless of killWindow so
// EvFrameVanished and reduceFrameCommandExited (abnormal exit) routes
// also free the container.
func TestEffReleaseFrameSandbox_invokesCleanup(t *testing.T) {
	var called atomic.Bool
	backend := noopBackend{}
	r := New(Config{Backend: backend})

	frameID := state.FrameID("f-release")
	r.storeFrameCleanup(frameID, func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffReleaseFrameSandbox{FrameID: frameID})

	deadline := time.Now().Add(200 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !called.Load() {
		t.Error("cleanup not called after EffReleaseFrameSandbox")
	}
}

func TestEffReleaseFrameSandbox_releasesSubsystemFrame(t *testing.T) {
	frameID := state.FrameID("f-vanished")
	sub := &fakeSubsystem{id: "stream:vanished", kind: state.LaunchSubsystemStream}
	reaped := make(chan state.SubsystemID, 1)
	r := New(Config{Backend: noopBackend{}})
	r.frameSubsystems[frameID] = sub
	r.frameSubsystemIDs[frameID] = sub.id
	r.subsystemFactories[state.LaunchSubsystemStream] = &reapingFakeFactory{sub: sub, reaped: reaped}

	r.execute(state.EffReleaseFrameSandbox{FrameID: frameID})

	if got := atomic.LoadInt32(&sub.releaseN); got != 1 {
		t.Fatalf("subsystem ReleaseFrame calls = %d, want 1", got)
	}
	select {
	case got := <-reaped:
		if got != sub.id {
			t.Fatalf("reaped subsystem = %q, want %q", got, sub.id)
		}
	case <-time.After(time.Second):
		t.Fatal("last frame release did not reap the stream subsystem")
	}
}

type reapingFakeFactory struct {
	sub    *fakeSubsystem
	reaped chan<- state.SubsystemID
}

func (f *reapingFakeFactory) Ensure(_ context.Context, _ state.SessionID, _ string, _ state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	return f.sub, f.sub.id, nil
}

func (f *reapingFakeFactory) Remove(ctx context.Context, id state.SubsystemID) {
	f.sub.Stop(ctx, rsubsystem.StopCauseLastFrameRelease)
	f.reaped <- id
}

// TestRequestShutdown_returnsAfterEffReleaseFrameSandboxes asserts the
// signal-handler contract: RequestShutdown waits until the reducer receives
// the cleanup result. Without this, signal handling could take its fallback
// cancellation path before containers had a chance to teardown.
func TestRequestShutdown_returnsAfterEffReleaseFrameSandboxes(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()
	done := make(chan ShutdownResult, 1)
	go func() {
		done <- r.RequestShutdown(time.Second)
	}()
	select {
	case result := <-done:
		if result != ShutdownResultCommitted {
			t.Fatalf("RequestShutdown result = %q", result)
		}
	case <-time.After(time.Second):
		t.Fatal("RequestShutdown did not return after cleanup result")
	}
	<-runErr
}

// TestRequestShutdown_timesOutWhenLoopNeverDrains guards against the
// reverse failure: a wedged event loop must not pin the signal handler
// forever — RequestShutdown must return after the timeout so cancel()
// still runs and systemd's TimeoutStopSec= is honoured.
func TestRequestShutdown_timesOutWhenLoopNeverDrains(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	start := time.Now()
	r.RequestShutdown(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("RequestShutdown returned after %v; want ≤500ms", elapsed)
	}
}

// TestRequestShutdown_enqueueTimeout_releasesConcurrentWaiters guards
// R2-F1: if the EventShutdown send into eventCh exceeds timeout (a
// wedged event loop with a full buffer), the first caller used to leave
// shutdownAck non-nil and uncloseable. A second caller would then park
// on that stale ack forever. The fix closes the ack and clears the slot
// before returning so retries succeed and waiters unblock.
func TestRequestShutdown_enqueueTimeout_releasesConcurrentWaiters(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})

	// Saturate eventCh so the blocking send inside RequestShutdown
	// cannot make progress within the timeout.
	for i := 0; i < cap(r.eventCh); i++ {
		r.eventCh <- state.EvEvent{Event: "filler"}
	}

	done1 := make(chan struct{})
	go func() {
		r.RequestShutdown(50 * time.Millisecond)
		close(done1)
	}()
	// A second waiter that arrives while the first is parked on the
	// timer must NOT inherit the stale ack — once the first call
	// surrenders, the second should also unblock promptly (either by
	// running its own enqueue attempt or by sharing the closed ack).
	done2 := make(chan struct{})
	time.Sleep(5 * time.Millisecond)
	go func() {
		r.RequestShutdown(50 * time.Millisecond)
		close(done2)
	}()
	for _, ch := range []chan struct{}{done1, done2} {
		select {
		case <-ch:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("RequestShutdown waiter did not unblock after enqueue timeout")
		}
	}
}

// TestRequestShutdown_secondCallSharesAck asserts that a second caller
// (e.g. SIGTERM arriving twice) does not enqueue a duplicate shutdown
// event and waits on the same ack as the first.
func TestRequestShutdown_secondCallSharesAck(t *testing.T) {
	persist := &recordingPersist{}
	r := New(Config{Backend: noopBackend{}, Persist: persist})
	release := make(chan struct{})
	r.sandboxCleanups["join"] = func() error { <-release; return successfulCleanupResult }
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()
	done1 := make(chan ShutdownResult, 1)
	go func() {
		done1 <- r.RequestShutdown(time.Second)
	}()
	time.Sleep(20 * time.Millisecond)
	done2 := make(chan ShutdownResult, 1)
	go func() {
		done2 <- r.RequestShutdown(time.Second)
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)
	for _, ch := range []chan ShutdownResult{done1, done2} {
		select {
		case result := <-ch:
			if result != ShutdownResultCommitted {
				t.Fatalf("joined result = %q", result)
			}
		case <-time.After(time.Second):
			t.Fatal("both RequestShutdown calls should return after the single ack")
		}
	}
	persist.mu.Lock()
	saves := persist.saves
	persist.mu.Unlock()
	if saves != 1 {
		t.Fatalf("joined shutdown Save calls = %d, want 1", saves)
	}
	<-runErr
}

func TestRequestShutdownDeadlineTerminatesWithNonCooperativeCleanup(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	release := make(chan struct{})
	r.sandboxCleanups["blocked"] = func() error {
		<-release
		return successfulCleanupResult
	}
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(context.Background()) }()

	result := r.RequestShutdown(50 * time.Millisecond)
	if result != ShutdownResultDeadlineExceeded {
		t.Fatalf("RequestShutdown result = %q, want %q", result, ShutdownResultDeadlineExceeded)
	}
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("Runtime.Done did not close after cleanup deadline")
	}
	close(release)
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error after Runtime-owned termination: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after Runtime.Done closed")
	}
}

type failingShutdownPersist struct{ recordingPersist }

func (p *failingShutdownPersist) Save([]SessionSnapshot) error {
	return errors.New("injected Save failure")
}

func TestRequestShutdownSaveFailureRollsBackWithoutCleanupOrTermination(t *testing.T) {
	persist := &failingShutdownPersist{}
	r := New(Config{Backend: noopBackend{}, Persist: persist})
	var cleanupCalls atomic.Int32
	r.sandboxCleanups["preserved"] = func() error {
		cleanupCalls.Add(1)
		return successfulCleanupResult
	}
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx) }()

	result := r.RequestShutdown(time.Second)
	if result != ShutdownResultCommitFailed {
		t.Fatalf("RequestShutdown result = %q, want %q", result, ShutdownResultCommitFailed)
	}
	if cleanupCalls.Load() != 0 {
		t.Fatal("Save failure started cleanup")
	}
	select {
	case <-r.Done():
		t.Fatal("Save failure terminated retryable runtime")
	default:
	}
	if got := r.TestPublishedState().Lifecycle; got != state.LifecycleRunning {
		t.Fatalf("lifecycle after Save failure = %v, want Running", got)
	}
	cancel()
	<-runErr
}

type prefixFailPersist struct {
	store map[string]SessionSnapshot
}

func (p *prefixFailPersist) Save(snapshots []SessionSnapshot) error {
	for i, snapshot := range snapshots {
		if i == 1 {
			return errors.New("injected failure after committed prefix")
		}
		p.store[snapshot.ID] = snapshot
	}
	return nil
}

func (p *prefixFailPersist) Delete(id string) error {
	delete(p.store, id)
	return nil
}

func (p *prefixFailPersist) Load() ([]SessionSnapshot, error) {
	out := make([]SessionSnapshot, 0, len(p.store))
	for _, snapshot := range p.store {
		out = append(out, snapshot)
	}
	return out, nil
}

func TestRequestShutdownPartialSaveKeepsMixedStoreAndSkipsTeardown(t *testing.T) {
	persist := &prefixFailPersist{store: map[string]SessionSnapshot{
		"s1": {ID: "s1", Project: "/old/s1"},
		"s2": {ID: "s2", Project: "/old/s2"},
	}}
	r := New(Config{Backend: noopBackend{}, Persist: persist})
	for _, id := range []state.SessionID{"s1", "s2"} {
		r.state.Sessions[id] = state.Session{ID: id, Project: "/new/" + string(id)}
	}
	var cleanupCalls atomic.Int32
	r.sandboxCleanups["preserved"] = func() error {
		cleanupCalls.Add(1)
		return successfulCleanupResult
	}
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx) }()

	if result := r.RequestShutdown(time.Second); result != ShutdownResultCommitFailed {
		t.Fatalf("RequestShutdown result = %q, want %q", result, ShutdownResultCommitFailed)
	}
	if cleanupCalls.Load() != 0 {
		t.Fatal("partial Save failure started teardown")
	}
	if len(persist.store) != 2 {
		t.Fatalf("partial Save changed session membership: %#v", persist.store)
	}
	newVersions := 0
	for _, id := range []string{"s1", "s2"} {
		snapshot, ok := persist.store[id]
		if !ok {
			t.Fatalf("partial Save deleted %s", id)
		}
		switch snapshot.Project {
		case "/new/" + id:
			newVersions++
		case "/old/" + id:
		default:
			t.Fatalf("session %s has invalid mixed-store version %q", id, snapshot.Project)
		}
	}
	if newVersions != 1 {
		t.Fatalf("new per-session versions = %d, want one committed prefix: %#v", newVersions, persist.store)
	}
	if got := r.TestPublishedState().Lifecycle; got != state.LifecycleRunning {
		t.Fatalf("lifecycle after partial Save = %v, want Running", got)
	}
	cancel()
	<-runErr
}

func TestRuntimeRestartColdLoadsCommittedCodexSessionAsWaiting(t *testing.T) {
	dataDir := t.TempDir()
	rolloutPath := filepath.Join(dataDir, "rollout-thread-restart.jsonl")
	if err := os.WriteFile(rolloutPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	codex := state.GetDriver("codex")
	if codex == nil {
		t.Fatal("codex driver is not registered")
	}
	driverState := codex.Restore(map[string]string{
		"status":       "running",
		"thread_id":    "thread-restart",
		"rollout_path": rolloutPath,
	}, time.Now())
	persist := NewFilePersist(dataDir)
	before := New(Config{Backend: noopBackend{}, Persist: persist})
	before.state.Sessions["session-restart"] = state.Session{
		ID:      "session-restart",
		Project: "/repo",
		Frames: []state.SessionFrame{{
			ID:      "frame-restart",
			Project: "/repo",
			Command: "codex",
			Driver:  driverState,
		}},
	}
	runErr := make(chan error, 1)
	go func() { runErr <- before.Run(context.Background()) }()
	if result := before.RequestShutdown(time.Second); result != ShutdownResultCommitted {
		t.Fatalf("shutdown result = %q, want committed", result)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("pre-restart runtime exited with error: %v", err)
	}

	after := New(Config{Backend: noopBackend{}, Persist: persist})
	if err := after.LoadSnapshot(true); err != nil {
		t.Fatalf("cold LoadSnapshot: %v", err)
	}
	restored, ok := after.state.Sessions["session-restart"]
	if !ok || len(restored.Frames) != 1 {
		t.Fatalf("Codex session disappeared across runtime restart: %#v", after.state.Sessions)
	}
	frame := restored.Frames[0]
	if got := codex.Status(frame.Driver); got != state.StatusWaiting {
		t.Fatalf("restored Codex status = %v, want Waiting", got)
	}
	metadata := codex.Persist(frame.Driver)
	if metadata["thread_id"] != "thread-restart" || metadata["rollout_path"] != rolloutPath {
		t.Fatalf("restored Codex resume metadata = %#v", metadata)
	}
}
