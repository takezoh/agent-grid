package state

import "testing"

func testRepo() GitHubRepositoryIdentity {
	return GitHubRepositoryIdentity{ID: "gh-repo-1", Owner: "acme", Name: "widgets"}
}

func createPrep(t *testing.T) (State, RepositoryPreparation) {
	t.Helper()
	s, effs := Reduce(New(), EvRepository{
		ConnID: 1, ReqID: "c1", Kind: "create", IntentID: "intent-1", PreparationID: "prep-1",
		ServerID: "srv-local", Repository: testRepo(), Fingerprint: "fp-1",
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
	if !hasEff[EffRepositoryEnsureEnvironment](effs) {
		t.Fatal("clone-ready must ensure environment")
	}
	prep, _ = FindPreparation(s, "prep-1")
	return s, prep
}

func TestRepositoryLifecycleSoleAvailability(t *testing.T) {
	s, prep := createPrep(t)
	s, prep = toCloneReady(t, s, prep.Revision)
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "d1", Kind: "devcontainer_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	_, effs := Reduce(s, EvRepository{
		ConnID: 1, ReqID: "bad", Kind: "commit_available", PreparationID: "prep-1", ObservedRevision: prep.Revision,
		SessionID: "sess-1", ClonePath: "c", EnvironmentID: "e", CloneDurable: true, LaunchSucceeded: true,
	})
	if _, ok := findEff[EffSendError](effs); !ok {
		t.Fatal("incomplete commit must fail")
	}
	s, effs = Reduce(s, EvRepository{
		ConnID: 1, ReqID: "ok", Kind: "commit_available", PreparationID: "prep-1", ObservedRevision: prep.Revision,
		SessionID: "sess-1", ClonePath: "c", EnvironmentID: "e", Command: "codex",
		CloneDurable: true, EnvironmentReady: true, LaunchSucceeded: true,
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
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "d", Kind: "devcontainer_started", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "f2", Kind: "failed", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	prep, _ = FindPreparation(s, "prep-1")
	s, effs = Reduce(s, EvRepository{ConnID: 1, ReqID: "r2", Kind: "retry", PreparationID: "prep-1", ObservedRevision: prep.Revision})
	if hasEff[EffRepositoryStartClone](effs) || !hasEff[EffRepositoryEnsureEnvironment](effs) {
		t.Fatal("devcontainer retry")
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
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "n", Kind: "interrupted", PreparationID: "prep-1"})
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "q", Kind: "quarantined", PreparationID: "prep-1"})
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
	s, _ = Reduce(New(), EvRepository{ConnID: 1, ReqID: "1", Kind: "create", IntentID: "a", PreparationID: "pa", ServerID: "s", Repository: testRepo(), Fingerprint: "a"})
	s, _ = Reduce(s, EvRepository{ConnID: 1, ReqID: "2", Kind: "create", IntentID: "b", PreparationID: "pb", ServerID: "s", Repository: testRepo(), Fingerprint: "b"})
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

func hasEff[T Effect](effs []Effect) bool { _, ok := findEff[T](effs); return ok }
