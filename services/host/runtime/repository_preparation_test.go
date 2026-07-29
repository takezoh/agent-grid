package runtime_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/runtime"
	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
	"github.com/takezoh/agent-grid/services/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-grid/services/platform/sandbox/devcontainer"
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

func prepConfig(t *testing.T, backend runtime.FrameBackend, cloner gitclone.Cloner, dc runtime.RepositoryDevcontainer) runtime.Config {
	t.Helper()
	if cloner == nil {
		cloner = &gitclone.FakeGitClone{}
	}
	if dc == nil {
		dc = runtime.NewFakeRepositoryDevcontainer()
	}
	return runtime.Config{
		Backend: backend, RepositorySessionsRoot: t.TempDir(), Cloner: cloner,
		CloneAuthority:             fixedAuthority{cred: gitclone.Credential{Username: "x", Token: "tok"}},
		RepositoryDevcontainer:     dc,
		RepositoryConfigGeneration: "gen-1",
		PreparationStore:           runtime.NewMemoryPreparationStore(),
		RepositoryEnvironmentStore: runtime.NewMemoryRepositoryEnvironmentStore(),
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

func TestPreparationIdempotencyDistinctIntentsAndEnvReuse(t *testing.T) {
	cloner := &gitclone.FakeGitClone{}
	dc := runtime.NewFakeRepositoryDevcontainer()
	r := startRuntime(t, prepConfig(t, newOKBackend(), cloner, dc))
	repo := testRepo()
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c1", Kind: "create", IntentID: "intent-a", PreparationID: "prep-a",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-a",
	})
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c2", Kind: "create", IntentID: "intent-b", PreparationID: "prep-b",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-b",
	})
	waitPrep(t, r, "prep-a", state.PrepStageAvailable, 5*time.Second)
	waitPrep(t, r, "prep-b", state.PrepStageAvailable, 5*time.Second)
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c1b", Kind: "create", IntentID: "intent-a", PreparationID: "prep-a-dup",
		ServerID: "srv", Repository: repo, Fingerprint: "fp-a",
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
	if p1.EnvironmentID == "" || p1.EnvironmentID != p2.EnvironmentID {
		t.Fatalf("env reuse p1=%q p2=%q", p1.EnvironmentID, p2.EnvironmentID)
	}
	if p1.ClonePath == p2.ClonePath {
		t.Fatal("clones must be independent")
	}
	if len(cloner.Calls) < 2 {
		t.Fatalf("clone calls=%d", len(cloner.Calls))
	}
}

func TestRepositoryDevcontainerScopeContract(t *testing.T) {
	dc := runtime.NewFakeRepositoryDevcontainer()
	mgr := &runtime.RepositoryEnvironmentManager{
		Store: runtime.NewMemoryRepositoryEnvironmentStore(), Devcontainer: dc,
		ConfigGen: "gen-a", SessionsRoot: "/sessions",
	}
	repo := testRepo()
	r1, err := mgr.Ensure(context.Background(), "srv", repo, "/sessions/clones/a")
	if err != nil || r1.Code != runtime.EnvCodeOK || r1.Reused {
		t.Fatalf("first: %+v err=%v", r1, err)
	}
	if r1.MountScope != "/sessions" || r1.FrameCWD != "/sessions/clones/a" {
		t.Fatalf("scope: %+v", r1)
	}
	r2, err := mgr.Ensure(context.Background(), "srv", repo, "/sessions/clones/b")
	if err != nil || r2.Code != runtime.EnvCodeOK || !r2.Reused || r2.EnvironmentID != r1.EnvironmentID {
		t.Fatalf("reuse: %+v err=%v", r2, err)
	}
	other := state.GitHubRepositoryIdentity{ID: "99", Owner: "acme", Name: "other"}
	r3, err := mgr.Ensure(context.Background(), "srv", other, "/sessions/clones/c")
	if err != nil || r3.EnvironmentID == r1.EnvironmentID || r3.Reused {
		t.Fatalf("cross-repo: %+v", r3)
	}
	mgr.ConfigGen = "gen-b"
	r4, err := mgr.Ensure(context.Background(), "srv", repo, "/sessions/clones/d")
	if err != nil || r4.EnvironmentID == r1.EnvironmentID || r4.Reused {
		t.Fatalf("gen: %+v", r4)
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
	r := startRuntime(t, prepConfig(t, newOKBackend(), cloner, nil))
	r.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c", Kind: "create", IntentID: "i", PreparationID: "p",
		ServerID: "srv", Repository: testRepo(), Fingerprint: "f",
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

	r2 := startRuntime(t, prepConfig(t, failBackend{}, nil, nil))
	r2.Enqueue(state.EvRepository{
		ConnID: 1, ReqID: "c", Kind: "create", IntentID: "i2", PreparationID: "p2",
		ServerID: "srv", Repository: testRepo(), Fingerprint: "f2",
	})
	waitPrep(t, r2, "p2", state.PrepStageFailed, 5*time.Second)
	if len(r2.TestPublishedState().Sessions) != 0 {
		t.Fatal("no session on launch failure")
	}
}

func TestRestartReconciliationAndFakeVsRealEnv(t *testing.T) {
	store := runtime.NewMemoryPreparationStore()
	_ = store.Save(runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "p", IntentID: "i", ServerID: "srv", Repository: testRepo(),
			Stage: state.PrepStageCloneInProgress, Revision: 2, LastOwnedWorkerGeneration: 1,
		},
		Intent: state.CreationIntent{ID: "i", ServerID: "srv", Repository: testRepo(), Fingerprint: "f"},
	})
	ctrl := runtime.NewPreparationController(store, gitclone.OSFilesystem{}, &gitclone.FakeGitClone{}, t.TempDir(), nil)
	evs := ctrl.ReconcileStartup([]state.RepositoryPreparation{{ID: "p", Stage: state.PrepStageCloneInProgress, Revision: 2}})
	if len(evs) != 1 {
		t.Fatalf("events=%v", evs)
	}
	if e, ok := evs[0].(state.EvRepository); !ok || e.Kind != "interrupted" {
		t.Fatalf("ev=%v", evs[0])
	}
	// Always-on unit contract: fake FailCode is a typed unavailable outcome (no error).
	// Real/absent authority path lives in repository_devcontainer_e2e_test.go FakeVsReal.
	fake := runtime.NewFakeRepositoryDevcontainer()
	fake.FailCode = runtime.EnvCodeUnavailable
	res, err := fake.Ensure(context.Background(), runtime.RepositoryEnvironmentRequest{
		ServerID: "s", Repository: testRepo(), ConfigGen: "g", SessionsRoot: "/s", ClonePath: "/s/c",
	})
	if err != nil || res.Code != runtime.EnvCodeUnavailable {
		t.Fatalf("fake unavailable: %+v err=%v", res, err)
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
		Intent:    state.CreationIntent{ID: "i", ServerID: "s", Repository: testRepo(), Fingerprint: "f"},
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

func TestFileRepositoryEnvironmentStoreFindReadyRestart(t *testing.T) {
	root := t.TempDir()
	fs := gitclone.OSFilesystem{}
	s1 := runtime.NewFileRepositoryEnvironmentStore(root, fs)
	rec := runtime.RepositoryEnvironmentRecord{
		ID: "env-1", ServerID: "srv", RepositoryID: "42", ContainerID: "ctr",
		ConfigGen: "gen-1", SessionsRoot: root, Ready: true, Disposition: state.EnvDispositionReady,
	}
	if err := s1.Save(rec); err != nil {
		t.Fatal(err)
	}
	// New store instance models daemon restart with same durable root.
	s2 := runtime.NewFileRepositoryEnvironmentStore(root, fs)
	got, ok, err := s2.FindReady("srv", "42", "gen-1")
	if err != nil || !ok || got.ID != "env-1" || got.ContainerID != "ctr" {
		t.Fatalf("FindReady after restart: ok=%v err=%v got=%+v", ok, err, got)
	}
	if _, ok, err := s2.FindReady("srv", "42", "gen-other"); err != nil || ok {
		t.Fatal("wrong generation must miss")
	}
	// Manager reuses via FindReady after restart-shaped store.
	mgr := &runtime.RepositoryEnvironmentManager{
		Store: s2, Devcontainer: runtime.NewFakeRepositoryDevcontainer(),
		ConfigGen: "gen-1", SessionsRoot: root,
	}
	res, err := mgr.Ensure(context.Background(), "srv", testRepo(), filepath.Join(root, "clones", "a"))
	if err != nil || res.Code != runtime.EnvCodeOK || !res.Reused || res.EnvironmentID != "env-1" {
		t.Fatalf("reuse: %+v err=%v", res, err)
	}
}

func TestEnsureSingleFlightPerEnvironmentKey(t *testing.T) {
	dc := runtime.NewFakeRepositoryDevcontainer()
	// Slow first Ensure so concurrent callers pile up.
	started := make(chan struct{})
	release := make(chan struct{})
	var first bool
	dcHook := &slowDevcontainer{inner: dc, started: started, release: release, first: &first}
	mgr := &runtime.RepositoryEnvironmentManager{
		Store: runtime.NewMemoryRepositoryEnvironmentStore(), Devcontainer: dcHook,
		ConfigGen: "g", SessionsRoot: "/sessions",
	}
	repo := testRepo()
	type result struct {
		res runtime.RepositoryEnvironmentResult
		err error
	}
	out := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r, e := mgr.Ensure(context.Background(), "srv", repo, "/sessions/c")
			out <- result{r, e}
		}()
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("ensure not started")
	}
	close(release)
	r1 := <-out
	r2 := <-out
	if r1.err != nil || r2.err != nil {
		t.Fatalf("errs %v %v", r1.err, r2.err)
	}
	if r1.res.EnvironmentID == "" || r1.res.EnvironmentID != r2.res.EnvironmentID {
		t.Fatalf("ids %q %q", r1.res.EnvironmentID, r2.res.EnvironmentID)
	}
	// Only one underlying create; second is reuse (either from store under lock or same flight).
	if len(dc.Calls) != 1 {
		t.Fatalf("devcontainer calls=%d want 1 (single-flight)", len(dc.Calls))
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
		Intent: state.CreationIntent{ID: "i-live", ServerID: "s", Repository: testRepo(), Fingerprint: "f"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(runtime.PreparationRecord{
		Preparation: state.RepositoryPreparation{
			ID: "stale", IntentID: "i-stale", ServerID: "s", Repository: testRepo(),
			Stage: state.PrepStageFailed, ClonePath: state.ClonePathID(clone), Revision: 3,
		},
		Intent: state.CreationIntent{ID: "i-stale", ServerID: "s", Repository: testRepo(), Fingerprint: "g"},
	}); err != nil {
		t.Fatal(err)
	}

	r := runtime.New(runtime.Config{
		RepositorySessionsRoot: root,
		PreparationStore:       store,
		RepositoryDevcontainer: runtime.UnavailableRepositoryDevcontainer{},
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

func TestPlatformRepositoryDevcontainerIsolationAndFailures(t *testing.T) {
	sessions := t.TempDir()
	clone := filepath.Join(sessions, "clones", "c1")
	if err := os.MkdirAll(filepath.Join(clone, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, ".devcontainer", "devcontainer.json"), []byte(`{"image":"t:latest"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var mountSources []string
	var identities []string
	dc := &runtime.PlatformRepositoryDevcontainer{
		EnsureFn: func(_ context.Context, projectPath string, opts sandbox.StartOptions) (string, error) {
			mountSources = append(mountSources, projectPath)
			identities = append(identities, opts.Isolation.ContainerKey(projectPath))
			if opts.Isolation.IsShared() {
				t.Fatal("shared isolation forbidden")
			}
			// Mount source is sessions root; identity is not the clone path.
			if projectPath == clone {
				t.Fatal("must not use clone path as mount source")
			}
			if opts.Isolation.ContainerKey(projectPath) == clone {
				t.Fatal("must not key container by clone path")
			}
			return "ctr-platform-1", nil
		},
	}
	req := runtime.RepositoryEnvironmentRequest{
		ServerID: "srv", Repository: testRepo(), ConfigGen: "gen-1",
		SessionsRoot: sessions, ClonePath: clone,
	}
	res, err := dc.Ensure(context.Background(), req)
	if err != nil || res.Code != runtime.EnvCodeOK || res.ContainerID != "ctr-platform-1" {
		t.Fatalf("ensure: %+v err=%v", res, err)
	}
	if res.MountScope != sessions || res.FrameCWD != clone {
		t.Fatalf("scope: %+v", res)
	}
	wantIdentity := runtime.RepositoryEnvironmentProjectPath(sessions, "srv", testRepo().ID, "gen-1")
	if len(mountSources) != 1 || mountSources[0] != sessions {
		t.Fatalf("mount source=%v want sessions root %s", mountSources, sessions)
	}
	if len(identities) != 1 || identities[0] != wantIdentity {
		t.Fatalf("identity=%v want %s", identities, wantIdentity)
	}
	// Second ensure with different clone reuses same mount source + identity.
	req.ClonePath = filepath.Join(sessions, "clones", "c2")
	_, _ = dc.Ensure(context.Background(), req)
	if len(mountSources) != 2 || mountSources[1] != sessions || identities[1] != wantIdentity {
		t.Fatalf("reuse: mounts=%v ids=%v", mountSources, identities)
	}

	// Typed failure propagation from manager boundary.
	failing := &runtime.PlatformRepositoryDevcontainer{
		EnsureFn: func(context.Context, string, sandbox.StartOptions) (string, error) {
			return "", fmt.Errorf("docker create exploded")
		},
	}
	fr, err := failing.Ensure(context.Background(), req)
	if err == nil || fr.Code != runtime.EnvCodeFailed {
		t.Fatalf("want failed: %+v err=%v", fr, err)
	}
	// Nil EnsureFn → typed unavailable (composition-root direct mode).
	unavail := &runtime.PlatformRepositoryDevcontainer{}
	ur, err := unavail.Ensure(context.Background(), req)
	if err != nil || ur.Code != runtime.EnvCodeUnavailable {
		t.Fatalf("unavailable: %+v err=%v", ur, err)
	}
	ur2, err := runtime.UnavailableRepositoryDevcontainer{}.Ensure(context.Background(), req)
	if err != nil || ur2.Code != runtime.EnvCodeUnavailable {
		t.Fatalf("typed unavailable adapter: %+v err=%v", ur2, err)
	}
}

// TestPlatformRepositoryDevcontainerBindMountAndFrameCWD proves the manager is
// given the sessions root as bind-mount source (not empty repository-env) and
// that frame cwd translates under that mount — not metadata-only.
func TestPlatformRepositoryDevcontainerBindMountAndFrameCWD(t *testing.T) {
	sessions := t.TempDir()
	clone := filepath.Join(sessions, "clones", "prep-bind")
	dcDir := filepath.Join(clone, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const wsFolder = "/workspaces/sessions"
	specJSON := fmt.Sprintf(`{"image":"t:latest","workspaceFolder":%q}`, wsFolder)
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(specJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotPath string
	var gotOpts sandbox.StartOptions
	dc := &runtime.PlatformRepositoryDevcontainer{
		EnsureFn: func(_ context.Context, projectPath string, opts sandbox.StartOptions) (string, error) {
			gotPath = projectPath
			gotOpts = opts
			return "ctr-bind-1", nil
		},
	}
	req := runtime.RepositoryEnvironmentRequest{
		ServerID: "srv", Repository: testRepo(), ConfigGen: "gen-bind",
		SessionsRoot: sessions, ClonePath: clone,
	}
	res, err := dc.Ensure(context.Background(), req)
	if err != nil || res.Code != runtime.EnvCodeOK {
		t.Fatalf("ensure: %+v err=%v", res, err)
	}
	if gotPath != sessions {
		t.Fatalf("Ensure projectPath (mount source) = %q, want sessions root %q", gotPath, sessions)
	}
	wantIdentity := runtime.RepositoryEnvironmentProjectPath(sessions, "srv", testRepo().ID, "gen-bind")
	if gotOpts.Isolation.ContainerIdentity != wantIdentity {
		t.Fatalf("ContainerIdentity = %q, want %q", gotOpts.Isolation.ContainerIdentity, wantIdentity)
	}
	if gotOpts.Isolation.ContainerKey(gotPath) != wantIdentity {
		t.Fatalf("ContainerKey = %q, want identity %q", gotOpts.Isolation.ContainerKey(gotPath), wantIdentity)
	}
	if gotOpts.Isolation.DevcontainerDir != dcDir {
		t.Fatalf("DevcontainerDir = %q, want clone-local %q", gotOpts.Isolation.DevcontainerDir, dcDir)
	}
	// Actual bind-mount source from resolved spec (not only returned MountScope).
	spec, err := sandboxdc.LoadSpec(gotPath, gotOpts.Isolation.DevcontainerDir)
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	spec.ContainerIdentity = gotOpts.Isolation.ContainerIdentity
	binds := spec.BindMounts()
	foundSessions := false
	for _, b := range binds {
		if b.Source == sessions {
			foundSessions = true
			break
		}
	}
	if !foundSessions {
		t.Fatalf("BindMounts = %+v, want sessions root %q as source", binds, sessions)
	}
	// Empty repository-env identity dir must not be the workspace mount source.
	envDir := wantIdentity
	for _, b := range binds {
		if b.Source == envDir {
			t.Fatalf("must not bind-mount empty repository-env identity dir %q", envDir)
		}
	}
	// Frame cwd translation under mounted sessions root.
	translated := runtime.TranslateRepositoryFrameCWD(sessions, clone, wsFolder)
	wantCWD := wsFolder + "/clones/prep-bind"
	if translated != wantCWD {
		t.Fatalf("translated frame cwd = %q, want %q", translated, wantCWD)
	}
	if res.FrameCWD != clone || res.MountScope != sessions {
		t.Fatalf("result metadata: %+v", res)
	}
	// Create args must stamp identity label, not sessions-root collision key.
	args := spec.BuildCreateArgs(spec.Image)
	foundLabel := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--label" && args[i+1] == "reactor-project="+wantIdentity {
			foundLabel = true
			break
		}
	}
	if !foundLabel {
		t.Fatalf("create args missing identity project label; args=%v", args)
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

type slowDevcontainer struct {
	inner   *runtime.FakeRepositoryDevcontainer
	started chan struct{}
	release chan struct{}
	first   *bool
}

func (s *slowDevcontainer) Ensure(ctx context.Context, req runtime.RepositoryEnvironmentRequest) (runtime.RepositoryEnvironmentResult, error) {
	if !*s.first {
		*s.first = true
		close(s.started)
		<-s.release
	}
	return s.inner.Ensure(ctx, req)
}

type recordingBackend struct{ spawned []string }

func newOKBackend() runtime.FrameBackend { return &recordingBackend{} }
func (b *recordingBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string, cols, rows uint16) error {
	b.spawned = append(b.spawned, frameID+"|"+startDir)
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
