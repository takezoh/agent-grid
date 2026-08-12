package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	rsubsystem "github.com/takezoh/agent-grid/services/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/services/host/state"
)

// minimalDriver is a zero-behaviour driver for testing bootstrap paths.
type minimalDriver struct{}

// blockingSubsystem stalls Stop until release is closed, so the shutdown
// drain's deadline behaviour can be exercised deterministically.
type blockingSubsystem struct {
	fakeSubsystem
	release <-chan struct{}
}

func (b *blockingSubsystem) Stop(ctx context.Context, cause rsubsystem.StopCause) {
	<-b.release
	b.fakeSubsystem.Stop(ctx, cause)
}

// registerBlockingSubsystem installs a subsystem whose Stop blocks until
// release is closed. Returns nothing: tests assert via shutdown results.
func registerBlockingSubsystem(r *Runtime, id state.SubsystemID, release <-chan struct{}) {
	r.subsystems[id] = &blockingSubsystem{
		fakeSubsystem: fakeSubsystem{id: id, kind: state.LaunchSubsystemStream},
		release:       release,
	}
}

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
func (minimalDriver) PrepareLaunch(_ state.DriverState, _ state.LaunchMode, project, command string, _ state.LaunchOptions) (state.LaunchPlan, error) {
	return state.LaunchPlan{Command: command, StartDir: project}, nil
}
func (minimalDriver) StartDir(_ state.DriverState) string                          { return "" }
func (minimalDriver) WithStartDir(s state.DriverState, _ string) state.DriverState { return s }

func TestEffReleaseFrame_releasesSubsystemFrame(t *testing.T) {
	frameID := state.FrameID("f-vanished")
	sub := &fakeSubsystem{id: "stream:vanished", kind: state.LaunchSubsystemStream}
	reaped := make(chan state.SubsystemID, 1)
	r := New(Config{Backend: noopBackend{}})
	r.frameSubsystems[frameID] = sub
	r.frameSubsystemIDs[frameID] = sub.id
	r.subsystemFactories[state.LaunchSubsystemStream] = &reapingFakeFactory{sub: sub, reaped: reaped}

	r.execute(state.EffReleaseFrame{FrameID: frameID})

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

// TestRequestShutdown_returnsAfterEffReleaseFrames asserts the
// signal-handler contract: RequestShutdown waits until the reducer receives
// the cleanup result. Without this, signal handling could take its fallback
// cancellation path before containers had a chance to teardown.
func TestRequestShutdown_returnsAfterEffReleaseFrames(t *testing.T) {
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
	registerBlockingSubsystem(r, "join", release)
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
	registerBlockingSubsystem(r, "blocked", release)
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
	stopped := &fakeSubsystem{id: "preserved", kind: state.LaunchSubsystemStream}
	r.subsystems["preserved"] = stopped
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx) }()

	result := r.RequestShutdown(time.Second)
	if result != ShutdownResultCommitFailed {
		t.Fatalf("RequestShutdown result = %q, want %q", result, ShutdownResultCommitFailed)
	}
	if atomic.LoadInt32(&stopped.stopN) != 0 {
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
	stopped := &fakeSubsystem{id: "preserved", kind: state.LaunchSubsystemStream}
	r.subsystems["preserved"] = stopped
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- r.Run(ctx) }()

	if result := r.RequestShutdown(time.Second); result != ShutdownResultCommitFailed {
		t.Fatalf("RequestShutdown result = %q, want %q", result, ShutdownResultCommitFailed)
	}
	if atomic.LoadInt32(&stopped.stopN) != 0 {
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
