package runtime_test

import (
	"context"
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
