package state

import "testing"

func testRepo() GitHubRepositoryIdentity {
	return GitHubRepositoryIdentity{ID: "gh-repo-1", Owner: "acme", Name: "widgets"}
}

func createPrep(t *testing.T) (State, RepositoryPreparation) {
	t.Helper()
	s, effs := Reduce(New(), EvRepository{
		ConnID: 1, ReqID: "c1", Kind: "create", IntentID: "intent-1", PreparationID: "prep-1",
		ServerID: "srv-local", Repository: testRepo(), Fingerprint: "fp-1", Command: "codex",
	})
	mustOK(t, effs)
	if !hasEff[EffRepositoryStartClone](effs) || !hasEff[EffRepositoryPersist](effs) {
		t.Fatal("create must persist and request clone")
	}
	prep, _ := FindPreparation(s, "prep-1")
	if prep.Stage != PrepStageConfirmed || len(s.Sessions) != 0 {
		t.Fatalf("create = %+v", prep)
	}
	return s, prep
}

func toCloneReady(t *testing.T, s State, rev uint64) (State, RepositoryPreparation) {
	t.Helper()
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "s1", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: rev})
	prep, _ := FindPreparation(s, "prep-1")
	s, effs := Reduce(s, EvRepository{ConnID: 1, ReqID: "s2", Kind: "clone_ready", PreparationID: "prep-1", ObservedRevision: prep.Revision, ClonePath: "clone/path-1"})
	mustOK(t, effs)
	if !hasEff[EffRepositoryLaunchSession](effs) {
		t.Fatal("clone-ready must request the session launch")
	}
	prep, _ = FindPreparation(s, "prep-1")
	return s, prep
}

func TestRepositoryLifecycleSoleAvailability(t *testing.T) {
	s, prep := createPrep(t)
	s, prep = toCloneReady(t, s, prep.Revision)
	_, effs := Reduce(s, EvRepository{
		ConnID: 1, ReqID: "bad", Kind: "commit_available", PreparationID: "prep-1", ObservedRevision: prep.Revision,
		SessionID: "sess-1", ClonePath: "c", CloneDurable: true,
	})
	if _, ok := findEff[EffSendError](effs); !ok {
		t.Fatal("incomplete commit must fail")
	}
	s, effs = Reduce(s, EvRepository{
		ConnID: 1, ReqID: "ok", Kind: "commit_available", PreparationID: "prep-1", ObservedRevision: prep.Revision,
		SessionID: "sess-1", ClonePath: "c", Command: "codex",
		CloneDurable: true, LaunchSucceeded: true,
	})
	mustOK(t, effs)
	prep, _ = FindPreparation(s, "prep-1")
	if prep.Stage != PrepStageAvailable || !SessionIsAvailableProjection(s, "sess-1") || !hasEff[EffBroadcastSessionsChanged](effs) {
		t.Fatalf("%+v", prep)
	}
}

func TestRepositoryTransitionsAndFences(t *testing.T) {
	s, prep := createPrep(t)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "a", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "f", Kind: "failed", PreparationID: "prep-1", ObservedRevision: prep.Revision, Code: PrepFailureCloneFailed})
	prep, _ = FindPreparation(s, "prep-1")
	if prep.Stage != PrepStageFailed || prep.RetryOrigin != RetryOriginClone {
		t.Fatal(prep)
	}
	s, effs := Reduce(s, EvRepository{ConnID: 1, ReqID: "r", Kind: "retry", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	if !hasEff[EffRepositoryStartClone](effs) {
		t.Fatal("reclone")
	}
	prep, _ = FindPreparation(s, "prep-1")
	s, prep = toCloneReady(t, s, prep.Revision)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "f2", Kind: "failed", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, effs = Reduce(s, EvRepository{ConnID: 1, ReqID: "r2", Kind: "retry", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	if hasEff[EffRepositoryStartClone](effs) || !hasEff[EffRepositoryLaunchSession](effs) {
		t.Fatal("launch retry must resume from the durable clone")
	}

	s, prep = createPrep(t)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "c", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, effs = Reduce(s, EvRepository{ConnID: 1, ReqID: "x", Kind: "cancelled", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	if prep, _ = FindPreparation(s, "prep-1"); prep.Stage != PrepStageCancelled || !hasEff[EffRepositoryStopOwnedWork](effs) {
		t.Fatal(prep)
	}
	s, prep = createPrep(t)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "i", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "n", Kind: "interrupted", PreparationID: "prep-1"})
	prep, _ = FindPreparation(s, "prep-1")
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "q", Kind: "quarantined", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	if prep, _ = FindPreparation(s, "prep-1"); prep.Stage != PrepStageInternalCorruption || len(s.Sessions) != 0 {
		t.Fatal(prep)
	}
	s, prep = createPrep(t)
	s2, effs := Reduce(s, EvRepository{ConnID: 1, ReqID: "stale", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: prep.Revision + 9})
	if err, ok := findEff[EffSendError](effs); !ok || err.Code != ErrCodeStaleRevision || hasEff[EffRepositoryPersist](effs) {
		t.Fatal(effs)
	}
	if p2, _ := FindPreparation(s2, "prep-1"); p2.Revision != prep.Revision {
		t.Fatal("stale mutate")
	}
	// Stale quarantine must not rewrite stage (ObservedRevision fencing).
	s, prep = createPrep(t)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "cs", Kind: "clone_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	sQ, qEffs := Reduce(s, EvRepository{ConnID: 1, ReqID: "stale-q", Kind: "quarantined", PreparationID: "prep-1", ObservedRevision: prep.Revision + 3})
	if err, ok := findEff[EffSendError](qEffs); !ok || err.Code != ErrCodeStaleRevision || hasEff[EffRepositoryPersist](qEffs) {
		t.Fatal(qEffs)
	}
	if pQ, _ := FindPreparation(sQ, "prep-1"); pQ.Stage != PrepStageCloneInProgress || pQ.Revision != prep.Revision {
		t.Fatalf("stale quarantine mutated: %+v", pQ)
	}
	s, _ = Reduce(New(), EvRepository{ConnID: 1, ReqID: "1", Kind: "create", IntentID: "a", PreparationID: "pa", ServerID: "s", Repository: testRepo(), Fingerprint: "a", Command: "codex"})
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "2", Kind: "create", IntentID: "b", PreparationID: "pb", ServerID: "s", Repository: testRepo(), Fingerprint: "b", Command: "codex"})
	if len(s.RepositoryPreparations) != 2 {
		t.Fatal("coalesce")
	}
	if !(SessionTarget{Source: TargetSourceRepository, Repository: RepositoryTarget{Repository: testRepo()}}).Valid() {
		t.Fatal("repo target")
	}
	if (SessionTarget{Source: TargetSourceProject, Project: ProjectTarget{ServerID: "s", Project: "p"}, Repository: RepositoryTarget{Repository: testRepo()}}).Valid() {
		t.Fatal("collapse")
	}
}

// TestCancelCloneReadyRetryOriginAndResume pins that cancelling after the clone
// is durable records retry_origin=launch, so the retry resumes the session
// launch instead of re-cloning.
func TestCancelCloneReadyRetryOriginAndResume(t *testing.T) {
	s, prep := createPrep(t)
	s, prep = toCloneReady(t, s, prep.Revision)
	if prep.Stage != PrepStageCloneReady {
		t.Fatalf("stage=%s", prep.Stage)
	}
	s, effs := Reduce(s, EvRepository{
		ConnID: 1, ReqID: "x", Kind: "cancelled",
		PreparationID: "prep-1", ObservedRevision: prep.Revision,
	})
	prep, _ = FindPreparation(s, "prep-1")
	if prep.Stage != PrepStageCancelled || prep.RetryOrigin != RetryOriginLaunch {
		t.Fatalf("cancel must set retry_origin=launch: %+v", prep)
	}
	if !hasEff[EffRepositoryStopOwnedWork](effs) {
		t.Fatal("cancel must stop owned work")
	}
	s, effs = Reduce(s, EvRepository{
		ConnID: 1, ReqID: "r", Kind: "retry",
		PreparationID: "prep-1", ObservedRevision: prep.Revision,
	})
	if hasEff[EffRepositoryStartClone](effs) {
		t.Fatal("launch retry must not re-enter clone-in-progress")
	}
	if !hasEff[EffRepositoryLaunchSession](effs) {
		t.Fatal("launch retry must resume the session launch")
	}
	prep, _ = FindPreparation(s, "prep-1")
	if prep.Stage != PrepStageCloneReady {
		t.Fatalf("retry lands on clone-ready (not clone-in-progress): %s", prep.Stage)
	}
}

func hasEff[T Effect](effs []Effect) bool { _, ok := findEff[T](effs); return ok }
