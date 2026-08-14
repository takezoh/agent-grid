package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/runtime"
	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

type fixedAuthority struct{ cred gitclone.Credential }

func (f fixedAuthority) Acquire(_ context.Context, _ state.ExecutionServerID, _ state.GitHubRepositoryIdentity) (gitclone.Credential, string, error) {
	return f.cred, gitclone.CodeOK, nil
}

func testRepo() state.GitHubRepositoryIdentity {
	return state.GitHubRepositoryIdentity{ID: "42", Owner: "acme", Name: "widgets"}
}

func startRuntime(t *testing.T, cfg runtime.Config) *runtime.Runtime {
	t.Helper()
	r := runtime.New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.Done():
		case <-time.After(2 * time.Second):
		}
	})
	return r
}

func prepConfig(t *testing.T, backend runtime.FrameBackend, cloner gitclone.Cloner) runtime.Config {
	t.Helper()
	if cloner == nil {
		cloner = &gitclone.FakeGitClone{}
	}
	return runtime.Config{
		Backend: backend, RepositorySessionsRoot: t.TempDir(), Cloner: cloner,
		CloneAuthority:   fixedAuthority{cred: gitclone.Credential{Username: "x", Token: "tok"}},
		PreparationStore: runtime.NewMemoryPreparationStore(),
	}
}

func TestPreparationCapacityAdmissionBeforeMutation(t *testing.T) {
	repo := testRepo()
	c := runtime.NewPreparationCapacity()
	c.BeginActive("s1", repo, "a")
	c.BeginActive("s1", repo, "b")
	if code := c.AdmitClone("s1", repo, "c", 0); code != "capacity_repository_active_full" {
		t.Fatalf("repo cap: %q", code)
	}
	c2 := runtime.NewPreparationCapacity()
	for i := 0; i < runtime.PrepCapActivePerServer; i++ {
		id := state.CreationIntentID(string(rune('a' + i)))
		r := state.GitHubRepositoryIdentity{ID: state.GitHubRepositoryID(string(rune('1' + i))), Owner: "o", Name: "n"}
		c2.BeginActive("s1", r, id)
	}
	if code := c2.AdmitClone("s1", repo, "q", 0); code != "capacity_active_full" {
		t.Fatalf("active full: %q", code)
	}
	c3 := runtime.NewPreparationCapacity()
	c3.AddNeverAvailable(runtime.PrepCapNeverAvailableBytes)
	if code := c3.AdmitClone("s1", repo, "d", 1); code != "capacity_disk_full" {
		t.Fatalf("disk: %q", code)
	}
	c4 := runtime.NewPreparationCapacity()
	c4.BeginActive("s1", repo, "same")
	if code := c4.AdmitClone("s1", repo, "same", 0); code != "intent_worker_busy" {
		t.Fatalf("intent busy: %q", code)
	}
}

func TestPreparationIdempotencyDistinctIntents(t *testing.T) {
	cloner := &gitclone.FakeGitClone{}
	backend := &recordingBackend{}
	r := startRuntime(t, prepConfig(t, backend, cloner))
	repo := testRepo()
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c1", Kind: "create", IntentID: "intent-a", PreparationID: "prep-a",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-a", Command: "codex --model selected",
	})
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c2", Kind: "create", IntentID: "intent-b", PreparationID: "prep-b",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-b", Command: "codex --model selected",
	})
	waitPrep(t, r, "prep-a", state.PrepStageAvailable, 5*time.Second)
	waitPrep(t, r, "prep-b", state.PrepStageAvailable, 5*time.Second)
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c1b", Kind: "create", IntentID: "intent-a", PreparationID: "prep-a-dup",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-a", Command: "codex --model selected",
	})
	time.Sleep(50 * time.Millisecond)
	snap := r.TestPublishedState()
	if len(snap.RepositoryPreparations) != 2 {
		t.Fatalf("preparations=%d want 2", len(snap.RepositoryPreparations))
	}
	if !state.SessionIsAvailableProjection(snap, "sess-prep-a") {
		t.Fatal("session a not available")
	}
	p1, p2 := snap.RepositoryPreparations["prep-a"], snap.RepositoryPreparations["prep-b"]
	if p1.ClonePath == p2.ClonePath {
		t.Fatal("clones must be independent")
	}
	// Through the accessor: two clones ran concurrently, so reading the slice
	// header directly races with whichever goroutine appended last.
	if calls := cloner.CloneCalls(); len(calls) < 2 {
		t.Fatalf("clone calls=%d", len(calls))
	}
	if len(backend.spawned) != 2 || backend.spawned[0].command != "codex --model selected" || backend.spawned[1].command != "codex --model selected" {
		t.Fatalf("repository command profile was not propagated to launch: %#v", backend.spawned)
	}
}

func TestCancelAndLaunchFailureDoNotPublishSession(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	cloner := &gitclone.FakeGitClone{Hook: func(req gitclone.CloneRequest) gitclone.CloneResult {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return gitclone.CloneResult{Code: gitclone.CodeOK}
	}}
	r := startRuntime(t, prepConfig(t, newOKBackend(), cloner))
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c", Kind: "create", IntentID: "i", PreparationID: "p",
		ServerID: "srv", Repository: testRepo(), Fingerprint: "f", Command: "codex",
	})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("clone not started")
	}
	waitPrep(t, r, "p", state.PrepStageCloneInProgress, 2*time.Second)
	prep := r.TestPublishedState().RepositoryPreparations["p"]
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "x", Kind: "cancelled", PreparationID: "p", ObservedRevision: prep.Revision,
	})
	waitPrep(t, r, "p", state.PrepStageCancelled, 2*time.Second)
	close(release)
	time.Sleep(80 * time.Millisecond)
	if state.SessionIsAvailableProjection(r.TestPublishedState(), "sess-p") {
		t.Fatal("cancelled prep must not publish session")
	}

	r2 := startRuntime(t, prepConfig(t, failBackend{}, nil))
	r2.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c", Kind: "create", IntentID: "i2", PreparationID: "p2",
		ServerID: "srv", Repository: testRepo(), Fingerprint: "f2", Command: "codex",
	})
	waitPrep(t, r2, "p2", state.PrepStageFailed, 5*time.Second)
	if len(r2.TestPublishedState().Sessions) != 0 {
		t.Fatal("no session on launch failure")
	}
}

func TestRestartReconciliation(t *testing.T) {
	store := runtime.NewMemoryPreparationStore()
	_ = store.Save(runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "p", IntentID: "i", ServerID: "srv", Repository: testRepo(),
			Stage: state.PrepStageCloneInProgress, Revision: 2, LastOwnedWorkerGeneration: 1,
		},
		Intent: state.CreationIntent{ID: "i", ServerID: "srv", Repository: testRepo(), Fingerprint: "f", Command: "codex"},
	})
	ctrl := runtime.NewPreparationController(store, gitclone.OSFilesystem{}, &gitclone.FakeGitClone{}, t.TempDir(), nil)
	evs := ctrl.ReconcileStartup([]state.RepositoryPreparation{{ID: "p", Stage: state.PrepStageCloneInProgress, Revision: 2}})
	if len(evs) != 1 {
		t.Fatalf("events=%v", evs)
	}
	if e, ok := evs[0].(state.EvRepository); !ok || e.Kind != "interrupted" {
		t.Fatalf("ev=%v", evs[0])
	}
}

func waitPrep(t *testing.T, r *runtime.Runtime, id state.PreparationID, stage state.PreparationStage, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		snap := r.TestPublishedState()
		if p, ok := snap.RepositoryPreparations[id]; ok && p.Stage == stage {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	p := r.TestPublishedState().RepositoryPreparations[id]
	t.Fatalf("prep %s stage=%q want %q full=%+v", id, p.Stage, stage, p)
}

func TestNeverAvailableAccountingAndAbandonCleanup(t *testing.T) {
	root := t.TempDir()
	fs := gitclone.OSFilesystem{}
	store := runtime.NewMemoryPreparationStore()
	ctrl := runtime.NewPreparationController(store, fs, &gitclone.FakeGitClone{}, root, nil)
	// Simulate published clone charged to never-available cap.
	clone := filepath.Join(root, "clones", "prep-na")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "blob"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctrl.AccountClonePath("prep-na", clone)
	if ctrl.Cap.NeverAvailBytes < 10 {
		t.Fatalf("never-available bytes not charged: %d", ctrl.Cap.NeverAvailBytes)
	}
	// Fill remaining headroom so AdmitClone rejects on disk capacity.
	remaining := runtime.PrepCapNeverAvailableBytes - ctrl.Cap.NeverAvailBytes
	if remaining > 0 {
		ctrl.Cap.AddNeverAvailable(remaining)
	}
	if code := ctrl.Cap.AdmitClone("s", testRepo(), "other", 1); code != "capacity_disk_full" {
		t.Fatalf("want capacity_disk_full, got %q", code)
	}
	// Terminal abandon cleanup removes never-available clone and releases charge.
	// Undo the artificial fill first so we assert only the clone-path account.
	if remaining > 0 {
		ctrl.Cap.RemoveNeverAvailable(remaining)
	}
	before := ctrl.Cap.NeverAvailBytes
	ctrl.CleanupNeverAvailable(state.RepositoryPreparation{
		ID: "prep-na", Stage: state.PrepStageAbandoned, ClonePath: state.ClonePathID(clone),
	})
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Fatalf("never-available clone must be deleted: %v", err)
	}
	if ctrl.Cap.NeverAvailBytes >= before && before > 0 {
		t.Fatalf("capacity not released: before=%d after=%d", before, ctrl.Cap.NeverAvailBytes)
	}
	// Available session clones must not be deleted.
	avail := filepath.Join(root, "clones", "prep-ok")
	if err := os.MkdirAll(avail, 0o755); err != nil {
		t.Fatal(err)
	}
	ctrl.CleanupNeverAvailable(state.RepositoryPreparation{
		ID: "prep-ok", Stage: state.PrepStageAvailable, SessionID: "sess-ok",
		ClonePath: state.ClonePathID(avail),
	})
	if _, err := os.Stat(avail); err != nil {
		t.Fatalf("available clone must survive: %v", err)
	}
}

func TestSweepNeverAvailableOrphansByAge(t *testing.T) {
	root := t.TempDir()
	store := runtime.NewMemoryPreparationStore()
	clone := filepath.Join(root, "clones", "old")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "old", IntentID: "i", ServerID: "s", Repository: testRepo(),
			Stage: state.PrepStageFailed, ClonePath: state.ClonePathID(clone), Revision: 2,
		},
		Intent:    state.CreationIntent{ID: "i", ServerID: "s", Repository: testRepo(), Fingerprint: "f", Command: "codex"},
		UpdatedAt: time.Now().Add(-runtime.PrepCapCleanupAge - time.Hour),
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	// Save overwrites UpdatedAt to now; force age via direct field re-save pattern:
	// Memory store sets UpdatedAt on Save — use Sweep with Now far in the future.
	ctrl := runtime.NewPreparationController(store, gitclone.OSFilesystem{}, &gitclone.FakeGitClone{}, root, nil)
	ctrl.AccountClonePath("old", clone)
	ctrl.Now = func() time.Time { return time.Now().Add(runtime.PrepCapCleanupAge + 2*time.Hour) }
	n := ctrl.SweepNeverAvailableOrphans()
	if n != 1 {
		t.Fatalf("cleaned=%d want 1", n)
	}
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Fatal("orphan clone must be removed")
	}
	if _, err := store.Load("old"); err == nil {
		t.Fatal("store record must be deleted")
	}
}

func TestRuntimeReconcileAndTickSweepProductionPath(t *testing.T) {
	root := t.TempDir()
	store := runtime.NewMemoryPreparationStore()
	clone := filepath.Join(root, "clones", "stale")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// In-progress durable prep reloaded by ReconcileRepositoryPreparations.
	if err := store.Save(runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "live", IntentID: "i-live", ServerID: "s", Repository: testRepo(),
			Stage: state.PrepStageCloneInProgress, Revision: 2, LastOwnedWorkerGeneration: 1,
		},
		Intent: state.CreationIntent{ID: "i-live", ServerID: "s", Repository: testRepo(), Fingerprint: "f", Command: "codex"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "stale", IntentID: "i-stale", ServerID: "s", Repository: testRepo(),
			Stage: state.PrepStageFailed, ClonePath: state.ClonePathID(clone), Revision: 3,
		},
		Intent: state.CreationIntent{ID: "i-stale", ServerID: "s", Repository: testRepo(), Fingerprint: "g", Command: "codex"},
	}); err != nil {
		t.Fatal(err)
	}

	r := runtime.New(runtime.Config{
		RepositorySessionsRoot: root,
		PreparationStore:       store,
		Backend:                newOKBackend(),
	})
	// Production boot path: reconcile after construction (mirrors bootSession).
	r.ReconcileRepositoryPreparations()
	snap := r.TestState()
	live, ok := snap.RepositoryPreparations["live"]
	if !ok || live.Stage != state.PrepStageInterrupted {
		t.Fatalf("boot reconcile must interrupt in-progress prep: ok=%v %+v", ok, live)
	}
	// Force age for the failed orphan and exercise the production tick sweep entry.
	ctrl := r.TestPreparationController()
	if ctrl == nil {
		t.Fatal("prep controller must be wired")
	}
	ctrl.AccountClonePath("stale", clone)
	ctrl.Now = func() time.Time { return time.Now().Add(runtime.PrepCapCleanupAge + 2*time.Hour) }
	r.TestSetRepositoryPrepSweepInterval(time.Millisecond)
	before := r.TestRepositoryPrepSweepCount()
	r.TestMaybeSweepRepositoryPreparations(time.Now())
	if r.TestRepositoryPrepSweepCount() <= before {
		t.Fatal("tick sweep path must invoke SweepNeverAvailableOrphans")
	}
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Fatal("tick sweep must remove aged never-available orphan")
	}
	// Throttle: immediate second call must not re-enter (count stays same).
	mid := r.TestRepositoryPrepSweepCount()
	r.TestMaybeSweepRepositoryPreparations(time.Now())
	if r.TestRepositoryPrepSweepCount() != mid {
		t.Fatal("throttled tick must not re-sweep immediately")
	}
}

func TestAbandonPendingLaunchReleasesNeverAvailableCapacity(t *testing.T) {
	root := t.TempDir()
	fs := gitclone.OSFilesystem{}
	ctrl := runtime.NewPreparationController(runtime.NewMemoryPreparationStore(), fs, &gitclone.FakeGitClone{}, root, nil)
	clone := filepath.Join(root, "clones", "prep-launch-fail")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "blob"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	const prepID state.PreparationID = "prep-launch-fail"
	ctrl.AccountClonePath(prepID, clone)
	charged := ctrl.Cap.NeverAvailBytes
	if charged < 10 {
		t.Fatalf("expected charge, got %d", charged)
	}
	// Launch-failure abandon must free the never-available account even when the
	// pending-launch map entry is already gone (worker finished; only capacity remains).
	ctrl.AbandonPendingLaunch(prepID)
	if ctrl.Cap.NeverAvailBytes != 0 {
		t.Fatalf("AbandonPendingLaunch must release never-available capacity: before=%d after=%d", charged, ctrl.Cap.NeverAvailBytes)
	}
	// Second abandon is a no-op (no double-free / panic).
	ctrl.AbandonPendingLaunch(prepID)
	if ctrl.Cap.NeverAvailBytes != 0 {
		t.Fatalf("double abandon leaked capacity: %d", ctrl.Cap.NeverAvailBytes)
	}
}

type recordedSpawn struct {
	frameID  string
	startDir string
	command  string
}

type recordingBackend struct{ spawned []recordedSpawn }

func newOKBackend() runtime.FrameBackend { return &recordingBackend{} }
func (b *recordingBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string, cols, rows uint16) error {
	b.spawned = append(b.spawned, recordedSpawn{frameID: frameID, startDir: startDir, command: command})
	return nil
}
func (b *recordingBackend) KillFrame(string) error                    { return nil }
func (b *recordingBackend) RespawnFrame(string, string) error         { return nil }
func (b *recordingBackend) FrameExitStatus(string) (bool, int, error) { return false, -1, nil }
func (b *recordingBackend) SendKeys(string, string) error             { return nil }
func (b *recordingBackend) SendKey(string, string) error              { return nil }
func (b *recordingBackend) SendEnter(string) error                    { return nil }
func (b *recordingBackend) LoadBuffer(string, string) error           { return nil }
func (b *recordingBackend) PasteBuffer(string, string) error          { return nil }
func (b *recordingBackend) ResolveID(string) (string, error)          { return "", nil }
func (b *recordingBackend) CaptureFrame(string, int) (string, error)  { return "", nil }
func (b *recordingBackend) SetEnv(string, string) error               { return nil }
func (b *recordingBackend) UnsetEnv(string) error                     { return nil }
func (b *recordingBackend) ShowEnvironment() (string, error)          { return "", nil }

type failBackend struct{}

func (failBackend) SpawnFrame(string, string, string, string, map[string]string, uint16, uint16) error {
	return context.DeadlineExceeded
}
func (failBackend) KillFrame(string) error                    { return nil }
func (failBackend) RespawnFrame(string, string) error         { return nil }
func (failBackend) FrameExitStatus(string) (bool, int, error) { return false, -1, nil }
func (failBackend) SendKeys(string, string) error             { return nil }
func (failBackend) SendKey(string, string) error              { return nil }
func (failBackend) SendEnter(string) error                    { return nil }
func (failBackend) LoadBuffer(string, string) error           { return nil }
func (failBackend) PasteBuffer(string, string) error          { return nil }
func (failBackend) ResolveID(string) (string, error)          { return "", nil }
func (failBackend) CaptureFrame(string, int) (string, error)  { return "", nil }
func (failBackend) SetEnv(string, string) error               { return nil }
func (failBackend) UnsetEnv(string) error                     { return nil }
func (failBackend) ShowEnvironment() (string, error)          { return "", nil }
