package runtime

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/takezoh/agent-grid/services/host/state"
)

func (r *Runtime) executeRepositoryEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffRepositoryPersist:
		r.persistPreparation(e.PreparationID)
	case state.EffRepositoryStartClone:
		r.startPreparationClone(e)
	case state.EffRepositoryEnsureEnvironment:
		r.ensurePreparationEnvironment(e)
	case state.EffRepositoryStopOwnedWork:
		if r.prepCtrl != nil {
			r.prepCtrl.StopOwnedWork(e.PreparationID, e.Generation)
			// Terminal abandon only: cancel/retry must keep durable clone for
			// RetryOriginDevcontainer. Age sweep handles long-lived orphans.
			if prep, ok := state.FindPreparation(r.state, e.PreparationID); ok && prep.Stage == state.PrepStageAbandoned {
				r.prepCtrl.CleanupNeverAvailable(prep)
			}
		}
	}
}

func (r *Runtime) persistPreparation(id state.PreparationID) {
	if r.prepCtrl == nil || r.prepCtrl.Store == nil {
		return
	}
	prep, ok := state.FindPreparation(r.state, id)
	if !ok {
		return
	}
	intent := r.state.CreationIntents[prep.IntentID]
	if err := r.prepCtrl.Persist(prep, intent); err != nil {
		slog.Error("runtime: preparation persist failed", "prep", id, "err", err)
	}
}

func (r *Runtime) startPreparationClone(e state.EffRepositoryStartClone) {
	if r.prepCtrl == nil {
		r.Enqueue(state.EvRepository{
			Kind: "failed", PreparationID: e.PreparationID, Code: state.PrepFailureCloneFailed,
		})
		return
	}
	prep, ok := state.FindPreparation(r.state, e.PreparationID)
	if !ok {
		return
	}
	if prep.LastOwnedWorkerGeneration != e.Generation {
		return
	}
	if !r.prepCtrl.TryAdmit(prep) {
		return
	}
	r.dispatch(state.EvRepository{
		Kind: "clone_started", PreparationID: prep.ID, ObservedRevision: prep.Revision,
	})
	prep, ok = state.FindPreparation(r.state, e.PreparationID)
	if !ok || prep.Stage != state.PrepStageCloneInProgress {
		r.prepCtrl.ReleaseAdmit(prep)
		return
	}
	intent := r.state.CreationIntents[prep.IntentID]
	r.prepCtrl.StartCloneAdmitted(prep, intent)
}

func (r *Runtime) ensurePreparationEnvironment(e state.EffRepositoryEnsureEnvironment) {
	prep, ok := state.FindPreparation(r.state, e.PreparationID)
	if !ok {
		return
	}
	if prep.LastOwnedWorkerGeneration != e.Generation {
		return
	}
	if r.envMgr == nil {
		r.dispatch(r.fenceRepo(state.EvRepository{
			Kind: "failed", PreparationID: prep.ID, Code: state.PrepFailureDevcontainerFail,
		}))
		return
	}
	r.dispatch(r.fenceRepo(state.EvRepository{
		Kind: "devcontainer_started", PreparationID: prep.ID,
	}))
	prep, ok = state.FindPreparation(r.state, e.PreparationID)
	if !ok || prep.Stage != state.PrepStageDevcontainerInProgress {
		return
	}
	go r.runEnsureEnvironment(prep)
}

func (r *Runtime) runEnsureEnvironment(prep state.RepositoryPreparation) {
	ctx := context.Background()
	clonePath := string(prep.ClonePath)
	res, err := r.envMgr.Ensure(ctx, prep.ServerID, prep.Repository, clonePath)
	if err != nil || res.Code != EnvCodeOK {
		r.enqueuePreparationEvent(state.EvRepository{
			Kind: "failed", PreparationID: prep.ID, Code: state.PrepFailureDevcontainerFail,
		})
		return
	}
	if r.prepCtrl != nil && r.prepCtrl.SessionsRoot != "" {
		if res.MountScope != "" && filepath.Clean(res.MountScope) != filepath.Clean(r.prepCtrl.SessionsRoot) {
			r.enqueuePreparationEvent(state.EvRepository{
				Kind: "failed", PreparationID: prep.ID, Code: state.PrepFailureDevcontainerFail,
			})
			return
		}
	}
	sessionID := state.SessionID("sess-" + string(prep.ID))
	cmd := "shell"
	if r.prepCtrl != nil {
		r.prepCtrl.RegisterPendingLaunch(prep.ID, pendingTerminalLaunch{
			SessionID: sessionID, ClonePath: state.ClonePathID(clonePath),
			EnvironmentID: res.EnvironmentID, Command: cmd,
			EnvReused: res.Reused, Generation: prep.LastOwnedWorkerGeneration,
		})
	}
	r.launchPreparationTerminal(prep, sessionID, clonePath, res)
}

func (r *Runtime) launchPreparationTerminal(prep state.RepositoryPreparation, sessionID state.SessionID, clonePath string, envRes RepositoryEnvironmentResult) {
	if r.cfg.Backend == nil {
		r.failPendingLaunch(prep.ID)
		return
	}
	frameID := string(prep.ID) + "-head"
	err := r.cfg.Backend.SpawnFrame(frameID, frameID, "shell", clonePath, nil, 80, 24)
	if err != nil {
		r.failPendingLaunch(prep.ID)
		return
	}
	r.enqueuePreparationEvent(state.EvRepository{
		Kind:              "commit_available",
		PreparationID:     prep.ID,
		SessionID:         sessionID,
		ClonePath:         state.ClonePathID(clonePath),
		EnvironmentID:     envRes.EnvironmentID,
		Command:           "shell",
		CloneDurable:      true,
		EnvironmentReady:  true,
		LaunchSucceeded:   true,
		EnvironmentReused: envRes.Reused,
	})
	if r.prepCtrl != nil {
		// Session now owns the clone; drop never-available accounting without delete.
		r.prepCtrl.ReleaseNeverAvailableAccount(prep.ID)
		r.prepCtrl.ConsumePendingLaunch(prep.ID)
	}
}

func (r *Runtime) failPendingLaunch(id state.PreparationID) {
	if r.prepCtrl != nil {
		r.prepCtrl.AbandonPendingLaunch(id)
	}
	r.enqueuePreparationEvent(state.EvRepository{
		Kind: "failed", PreparationID: id, Code: state.PrepFailureDevcontainerFail,
	})
}

func (r *Runtime) fenceRepo(ev state.EvRepository) state.EvRepository {
	if prep, ok := state.FindPreparation(r.state, ev.PreparationID); ok {
		ev.ObservedRevision = prep.Revision
	}
	return ev
}

func (r *Runtime) enqueuePreparationEvent(ev state.EvRepository) {
	if snap := r.published.Load(); snap != nil {
		if prep, ok := state.FindPreparation(*snap, ev.PreparationID); ok {
			ev.ObservedRevision = prep.Revision
		}
	}
	r.Enqueue(ev)
}

func (r *Runtime) ReconcileRepositoryPreparations() {
	if r.prepCtrl == nil {
		return
	}
	preps := make([]state.RepositoryPreparation, 0, len(r.state.RepositoryPreparations))
	for _, p := range r.state.RepositoryPreparations {
		preps = append(preps, p)
	}
	if r.prepCtrl.Store != nil {
		if recs, err := r.prepCtrl.Store.List(); err == nil {
			for _, rec := range recs {
				if _, ok := r.state.RepositoryPreparations[rec.Preparation.ID]; !ok {
					r.state = statePutPrep(r.state, rec)
				}
				preps = append(preps, rec.Preparation)
			}
		}
	}
	for _, ev := range r.prepCtrl.ReconcileStartup(preps) {
		next, effects := state.Reduce(r.state, ev)
		r.state = next
		for _, eff := range effects {
			if pe, ok := eff.(state.EffRepositoryPersist); ok {
				r.persistPreparation(pe.PreparationID)
			}
		}
	}
	// Age-based orphan sweep for never-available clones (not session-available).
	_ = r.prepCtrl.SweepNeverAvailableOrphans()
	r.publishState(r.state)
}

func statePutPrep(s state.State, rec PreparationRecord) state.State {
	if s.RepositoryPreparations == nil {
		s.RepositoryPreparations = map[state.PreparationID]state.RepositoryPreparation{}
	}
	if s.CreationIntents == nil {
		s.CreationIntents = map[state.CreationIntentID]state.CreationIntent{}
	}
	cp := make(map[state.PreparationID]state.RepositoryPreparation, len(s.RepositoryPreparations)+1)
	for k, v := range s.RepositoryPreparations {
		cp[k] = v
	}
	cp[rec.Preparation.ID] = rec.Preparation
	s.RepositoryPreparations = cp
	ci := make(map[state.CreationIntentID]state.CreationIntent, len(s.CreationIntents)+1)
	for k, v := range s.CreationIntents {
		ci[k] = v
	}
	ci[rec.Intent.ID] = rec.Intent
	s.CreationIntents = ci
	return s
}
