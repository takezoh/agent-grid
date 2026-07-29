package state

// Pure repository preparation lifecycle (no lease/process/fs I/O).

const ErrCodeStaleRevision = "stale_revision"

func reduceRepository(s State, e EvRepository) (State, []Effect) {
	switch e.Kind {
	case "create":
		return reduceRepositoryCreate(s, e)
	case "clone_started":
		return reduceRepositoryCloneStarted(s, e)
	case "clone_ready":
		return reduceRepositoryCloneReady(s, e)
	case "devcontainer_started":
		return reduceRepositoryDevcontainerStarted(s, e)
	case "failed":
		return reduceRepositoryFailed(s, e)
	case "cancelled":
		return reduceRepositoryCancelled(s, e)
	case "interrupted":
		return reduceRepositoryInterrupted(s, e)
	case "quarantined":
		return reduceRepositoryQuarantined(s, e)
	case "retry":
		return reduceRepositoryRetry(s, e)
	case "commit_available":
		return reduceRepositoryCommitAvailable(s, e)
	case "abandon":
		return reduceRepositoryAbandon(s, e)
	default:
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureIllegalTransition, "unknown repository event kind")
	}
}

func reduceRepositoryCreate(s State, e EvRepository) (State, []Effect) {
	if e.IntentID == "" || e.PreparationID == "" || e.ServerID == "" || !e.Repository.Valid() {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureInvalidIdentity, "fully-qualified identities required")
	}
	if existing, ok := s.CreationIntents[e.IntentID]; ok {
		if existing.Fingerprint != e.Fingerprint {
			return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureIllegalTransition, "intent fingerprint conflict")
		}
		if prep, ok := findPrepByIntent(s, e.IntentID); ok {
			return s, []Effect{okResp(e.ConnID, e.ReqID, prepSnapshot(prep))}
		}
	}
	if _, exists := s.RepositoryPreparations[e.PreparationID]; exists {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureIllegalTransition, "preparation already exists")
	}
	intent := CreationIntent{ID: e.IntentID, ServerID: e.ServerID, Repository: e.Repository, Fingerprint: e.Fingerprint}
	prep := RepositoryPreparation{
		ID: e.PreparationID, IntentID: e.IntentID, ServerID: e.ServerID, Repository: e.Repository,
		Stage: PrepStageConfirmed, EnvironmentDisposition: EnvDispositionNone,
		Revision: 1, LastOwnedWorkerGeneration: 1,
	}
	s = putCreationIntent(s, intent)
	s = putPreparation(s, prep)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		EffRepositoryStartClone{PreparationID: prep.ID, ServerID: prep.ServerID, Repository: prep.Repository, Generation: 1},
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func reduceRepositoryCloneStarted(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	switch prep.Stage {
	case PrepStageConfirmed, PrepStageFailed, PrepStageCancelled, PrepStageInterrupted:
	default:
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	if prep.RetryOrigin != RetryOriginNone && prep.RetryOrigin != RetryOriginClone {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	return advancePrep(s, prep, PrepStageCloneInProgress, PrepFailureNone, RetryOriginNone, EnvDispositionNone, e.ConnID, e.ReqID)
}

func reduceRepositoryCloneReady(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage != PrepStageCloneInProgress {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	if e.ClonePath == "" {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureInvalidIdentity, "clone path required")
	}
	prep.ClonePath, prep.CloneDurable = e.ClonePath, true
	prep.Stage, prep.FailureCode, prep.RetryOrigin, prep.Revision = PrepStageCloneReady, PrepFailureNone, RetryOriginNone, prep.Revision+1
	s = putPreparation(s, prep)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		EffRepositoryEnsureEnvironment{PreparationID: prep.ID, ServerID: prep.ServerID, Repository: prep.Repository, Generation: prep.LastOwnedWorkerGeneration},
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func reduceRepositoryDevcontainerStarted(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if (prep.Stage != PrepStageCloneReady && prep.Stage != PrepStageFailed && prep.Stage != PrepStageCancelled) || !prep.CloneDurable {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	return advancePrep(s, prep, PrepStageDevcontainerInProgress, PrepFailureNone, RetryOriginNone, EnvDispositionEnsuring, e.ConnID, e.ReqID)
}

func reduceRepositoryFailed(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage.IsTerminal() {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	code, origin, env := e.Code, RetryOriginClone, prep.EnvironmentDisposition
	if code == PrepFailureNone {
		code = PrepFailureCloneFailed
	}
	if prep.Stage == PrepStageDevcontainerInProgress || prep.Stage == PrepStageCloneReady {
		origin, env = RetryOriginDevcontainer, EnvDispositionFailed
		if code == PrepFailureCloneFailed {
			code = PrepFailureDevcontainerFail
		}
	}
	return advancePrep(s, prep, PrepStageFailed, code, origin, env, e.ConnID, e.ReqID)
}

func reduceRepositoryCancelled(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage.IsTerminal() {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	origin := RetryOriginClone
	if prep.Stage == PrepStageDevcontainerInProgress || prep.Stage == PrepStageCloneReady {
		origin = RetryOriginDevcontainer
	}
	prep.Stage, prep.FailureCode, prep.RetryOrigin, prep.Revision = PrepStageCancelled, PrepFailureCancelled, origin, prep.Revision+1
	s = putPreparation(s, prep)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		EffRepositoryStopOwnedWork{PreparationID: prep.ID, Generation: prep.LastOwnedWorkerGeneration},
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func reduceRepositoryInterrupted(s State, e EvRepository) (State, []Effect) {
	prep, ok := FindPreparation(s, e.PreparationID)
	if !ok {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureInvalidIdentity, "preparation not found")
	}
	switch prep.Stage {
	case PrepStageConfirmed, PrepStageCloneInProgress, PrepStageDevcontainerInProgress:
	default:
		return s, []Effect{okResp(e.ConnID, e.ReqID, prepSnapshot(prep))}
	}
	origin := RetryOriginClone
	if prep.Stage == PrepStageDevcontainerInProgress {
		origin = RetryOriginDevcontainer
	}
	return advancePrep(s, prep, PrepStageInterrupted, PrepFailureInterrupted, origin, prep.EnvironmentDisposition, e.ConnID, e.ReqID)
}

func reduceRepositoryQuarantined(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.IsSessionAvailable() {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	return advancePrep(s, prep, PrepStageInternalCorruption, PrepFailureInternalCorrupt, RetryOriginNone, prep.EnvironmentDisposition, e.ConnID, e.ReqID)
}

func reduceRepositoryRetry(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage != PrepStageFailed && prep.Stage != PrepStageCancelled && prep.Stage != PrepStageInterrupted {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	prep.LastOwnedWorkerGeneration++
	prep.FailureCode, prep.Revision = PrepFailureNone, prep.Revision+1
	var start Effect
	if prep.RetryOrigin == RetryOriginDevcontainer {
		prep.Stage, prep.EnvironmentDisposition = PrepStageCloneReady, EnvDispositionNone
		start = EffRepositoryEnsureEnvironment{PreparationID: prep.ID, ServerID: prep.ServerID, Repository: prep.Repository, Generation: prep.LastOwnedWorkerGeneration}
	} else {
		prep.Stage, prep.CloneDurable, prep.ClonePath = PrepStageConfirmed, false, ""
		prep.EnvironmentReady, prep.EnvironmentDisposition = false, EnvDispositionNone
		start = EffRepositoryStartClone{PreparationID: prep.ID, ServerID: prep.ServerID, Repository: prep.Repository, Generation: prep.LastOwnedWorkerGeneration}
	}
	prep.RetryOrigin = RetryOriginNone
	s = putPreparation(s, prep)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		start,
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func reduceRepositoryCommitAvailable(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage != PrepStageDevcontainerInProgress && prep.Stage != PrepStageCloneReady {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	if !e.CloneDurable || !e.EnvironmentReady || !e.LaunchSucceeded {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureIncompleteCommit, "clone, environment, and launch evidence required")
	}
	if e.SessionID == "" || e.ClonePath == "" || e.EnvironmentID == "" {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureInvalidIdentity, "session, clone path, and environment ids required")
	}
	if SessionIsAvailableProjection(s, e.SessionID) {
		return s, rejectPrep(e.ConnID, e.ReqID, PrepFailureIllegalTransition, "session already available")
	}
	prep.Stage, prep.SessionID = PrepStageAvailable, e.SessionID
	prep.ClonePath, prep.EnvironmentID = e.ClonePath, e.EnvironmentID
	prep.CloneDurable, prep.EnvironmentReady, prep.LaunchSucceeded = true, true, true
	if e.EnvironmentReused {
		prep.EnvironmentDisposition = EnvDispositionReused
	} else {
		prep.EnvironmentDisposition = EnvDispositionReady
	}
	prep.FailureCode, prep.RetryOrigin, prep.Revision = PrepFailureNone, RetryOriginNone, prep.Revision+1
	s = putPreparation(s, prep)
	s.Sessions = cloneSessions(s.Sessions)
	if s.Sessions == nil {
		s.Sessions = map[SessionID]Session{}
	}
	s.Sessions[e.SessionID] = Session{
		ID: e.SessionID, Project: "github://" + string(prep.ServerID) + "/" + string(prep.Repository.Owner) + "/" + string(prep.Repository.Name),
		Command: e.Command,
	}
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		EffBroadcastSessionsChanged{},
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func reduceRepositoryAbandon(s State, e EvRepository) (State, []Effect) {
	prep, ok, effs := loadPrepFenced(s, e.PreparationID, e.ObservedRevision, e.ConnID, e.ReqID)
	if !ok {
		return s, effs
	}
	if prep.Stage == PrepStageAvailable {
		return s, illegal(prep, e.ConnID, e.ReqID)
	}
	s, _ = advancePrep(s, prep, PrepStageAbandoned, prep.FailureCode, prep.RetryOrigin, prep.EnvironmentDisposition, e.ConnID, e.ReqID)
	prep, _ = FindPreparation(s, e.PreparationID)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		EffRepositoryStopOwnedWork{PreparationID: prep.ID, Generation: prep.LastOwnedWorkerGeneration},
		okResp(e.ConnID, e.ReqID, prepSnapshot(prep)),
	}
}

func advancePrep(s State, prep RepositoryPreparation, stage PreparationStage, code PreparationFailureCode, origin RetryOrigin, env EnvironmentDisposition, conn ConnID, req string) (State, []Effect) {
	prep.Stage, prep.FailureCode, prep.RetryOrigin, prep.EnvironmentDisposition = stage, code, origin, env
	prep.Revision++
	s = putPreparation(s, prep)
	return s, []Effect{
		EffRepositoryPersist{PreparationID: prep.ID, Revision: prep.Revision},
		okResp(conn, req, prepSnapshot(prep)),
	}
}

func loadPrepFenced(s State, id PreparationID, observed uint64, conn ConnID, req string) (RepositoryPreparation, bool, []Effect) {
	prep, ok := FindPreparation(s, id)
	if !ok {
		return RepositoryPreparation{}, false, rejectPrep(conn, req, PrepFailureInvalidIdentity, "preparation not found")
	}
	if observed != prep.Revision {
		return prep, false, []Effect{errResp(conn, req, ErrCodeStaleRevision, "observed revision does not match")}
	}
	return prep, true, nil
}

func findPrepByIntent(s State, intent CreationIntentID) (RepositoryPreparation, bool) {
	for _, p := range s.RepositoryPreparations {
		if p.IntentID == intent {
			return p, true
		}
	}
	return RepositoryPreparation{}, false
}

func illegal(prep RepositoryPreparation, conn ConnID, req string) []Effect {
	return rejectPrep(conn, req, PrepFailureIllegalTransition, "illegal preparation transition from "+string(prep.Stage))
}

func rejectPrep(conn ConnID, req string, code PreparationFailureCode, msg string) []Effect {
	return []Effect{errResp(conn, req, string(code), msg)}
}

func prepSnapshot(p RepositoryPreparation) map[string]any {
	return map[string]any{
		"preparation_id": string(p.ID), "intent_id": string(p.IntentID), "server_id": string(p.ServerID),
		"repository_id": string(p.Repository.ID), "owner": string(p.Repository.Owner), "name": string(p.Repository.Name),
		"stage": string(p.Stage), "failure_code": string(p.FailureCode), "retry_origin": string(p.RetryOrigin),
		"environment_disposition": string(p.EnvironmentDisposition), "revision": p.Revision,
		"session_id": string(p.SessionID), "clone_path": string(p.ClonePath), "environment_id": string(p.EnvironmentID),
	}
}
