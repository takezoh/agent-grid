package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

type PreparationController struct {
	mu sync.Mutex

	Store       PreparationStore
	FS          gitclone.FilesystemProcess
	Cloner      gitclone.Cloner
	Authority   CloneAuthority
	SessionsRoot string // repository sessions root (mount scope)
	Cap         *PreparationCapacity
	Enqueue     func(state.Event)

	workers map[state.PreparationID]*prepWorker
	pendingLaunch map[state.PreparationID]pendingTerminalLaunch
}

type prepWorker struct {
	generation uint64
	intent     state.CreationIntentID
	server     state.ExecutionServerID
	repo       state.GitHubRepositoryIdentity
	cancel     context.CancelFunc
	pid        int
	staging    string
	final      string
	marker     string
}

type pendingTerminalLaunch struct {
	SessionID     state.SessionID
	ClonePath     state.ClonePathID
	EnvironmentID state.RepositoryEnvironmentID
	Command       string
	Revision      uint64
	EnvReused     bool
	Generation    uint64
}

func NewPreparationController(store PreparationStore, fs gitclone.FilesystemProcess, cloner gitclone.Cloner, root string, enqueue func(state.Event)) *PreparationController {
	if fs == nil {
		fs = gitclone.OSFilesystem{}
	}
	if cloner == nil {
		cloner = gitclone.Service{}
	}
	return &PreparationController{
		Store:        store,
		FS:           fs,
		Cloner:       cloner,
		SessionsRoot: root,
		Cap:          NewPreparationCapacity(),
		Enqueue:      enqueue,
		workers:      map[state.PreparationID]*prepWorker{},
		pendingLaunch: map[state.PreparationID]pendingTerminalLaunch{},
	}
}

func (c *PreparationController) Persist(prep state.RepositoryPreparation, intent state.CreationIntent) error {
	if c.Store == nil {
		return nil
	}
	return c.Store.Save(PreparationRecord{Preparation: prep, Intent: intent, Marker: markerFor(prep)})
}

func markerFor(p state.RepositoryPreparation) string {
	return fmt.Sprintf("%s:%d:%d", p.ID, p.LastOwnedWorkerGeneration, p.Revision)
}

func (c *PreparationController) TryAdmit(prep state.RepositoryPreparation) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	code := c.Cap.AdmitClone(prep.ServerID, prep.Repository, prep.IntentID, 0)
	if code == "" {
		c.Cap.BeginActive(prep.ServerID, prep.Repository, prep.IntentID)
		return true
	}
	if code == "capacity_active_full" {
		c.Cap.Enqueue()
		slog.Info("repository preparation queued", "prep", prep.ID, "code", code)
		return false
	}
	if c.Enqueue != nil {
		c.Enqueue(state.EvRepository{
			Kind: "failed", PreparationID: prep.ID, ObservedRevision: prep.Revision,
			Code: state.PrepFailureCloneFailed,
		})
	}
	return false
}

func (c *PreparationController) ReleaseAdmit(prep state.RepositoryPreparation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Cap.EndActive(prep.ServerID, prep.Repository, prep.IntentID)
}

func (c *PreparationController) StartCloneAdmitted(prep state.RepositoryPreparation, intent state.CreationIntent) {
	if c.Enqueue == nil {
		return
	}
	c.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	w := &prepWorker{
		generation: prep.LastOwnedWorkerGeneration,
		intent:     prep.IntentID,
		server:     prep.ServerID,
		repo:       prep.Repository,
		cancel:     cancel,
		marker:     markerFor(prep),
	}
	w.staging = filepath.Join(c.SessionsRoot, "staging", string(prep.ID))
	w.final = filepath.Join(c.SessionsRoot, "clones", string(prep.ID))
	c.workers[prep.ID] = w
	c.mu.Unlock()
	go c.runClone(ctx, prep, intent, w)
}

func (c *PreparationController) runClone(ctx context.Context, prep state.RepositoryPreparation, intent state.CreationIntent, w *prepWorker) {
	defer c.finishWorker(prep.ID, w)

	_ = c.FS.RemoveAll(w.staging)
	if err := c.FS.MkdirAll(filepath.Dir(w.staging), 0o755); err != nil {
		c.fail(prep, state.PrepFailureCloneFailed, err.Error())
		return
	}
	var cred gitclone.Credential
	if c.Authority != nil {
		var code string
		var err error
		cred, code, err = c.Authority.Acquire(ctx, prep.ServerID, prep.Repository)
		if err != nil || code != "" && code != gitclone.CodeOK {
			if code == "" {
				code = gitclone.CodeAuthorityUnavailable
			}
			c.fail(prep, state.PrepFailureCloneFailed, code)
			return
		}
	}
	res := c.Cloner.Clone(ctx, gitclone.CloneRequest{
		Owner: string(prep.Repository.Owner), Name: string(prep.Repository.Name),
		DestDir: w.staging, Cred: cred,
	})
	if ctx.Err() != nil || res.Code == gitclone.CodeCancelled {
		c.fail(prep, state.PrepFailureCancelled, res.Code)
		return
	}
	if res.Code != gitclone.CodeOK {
		_ = c.FS.RemoveAll(w.staging)
		c.fail(prep, state.PrepFailureCloneFailed, res.Code)
		return
	}
	pub := gitclone.PublishClone(gitclone.PublishRequest{
		StagingDir: w.staging, FinalDir: w.final, Marker: w.marker, FS: c.FS,
	})
	if pub.Code != gitclone.CodeAtomicPublishOK {
		_ = c.FS.RemoveAll(w.staging)
		if pub.Code == gitclone.CodeCloneIdentityConflict {
			c.Enqueue(state.EvRepository{Kind: "quarantined", PreparationID: prep.ID})
			return
		}
		c.fail(prep, state.PrepFailureCloneFailed, pub.Code)
		return
	}
	if c.Store != nil {
		_ = c.Store.Save(PreparationRecord{
			Preparation: prep, Intent: intent, Marker: w.marker,
			StagingDir: w.staging, FinalDir: w.final,
		})
	}
	c.Enqueue(state.EvRepository{
		Kind: "clone_ready", PreparationID: prep.ID,
		ClonePath: state.ClonePathID(w.final),
	})
}

func (c *PreparationController) fail(prep state.RepositoryPreparation, code state.PreparationFailureCode, diag string) {
	slog.Info("repository preparation failed", "prep", prep.ID, "code", code, "diag", diag)
	c.Enqueue(state.EvRepository{
		Kind: "failed", PreparationID: prep.ID, Code: code,
	})
}

func (c *PreparationController) finishWorker(id state.PreparationID, w *prepWorker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.workers[id]; ok && cur == w {
		delete(c.workers, id)
	}
	c.Cap.EndActive(w.server, w.repo, w.intent)
}

func (c *PreparationController) StopOwnedWork(id state.PreparationID, generation uint64) {
	c.mu.Lock()
	w, ok := c.workers[id]
	if !ok || w.generation != generation {
		c.mu.Unlock()
		return
	}
	cancel := w.cancel
	pid := w.pid
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if pid > 0 {
		_ = c.FS.KillProcess(pid)
	}
}

func (c *PreparationController) RegisterPendingLaunch(id state.PreparationID, pending pendingTerminalLaunch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pendingLaunch[id] = pending
}

func (c *PreparationController) ConsumePendingLaunch(id state.PreparationID) (pendingTerminalLaunch, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pendingLaunch[id]
	if ok {
		delete(c.pendingLaunch, id)
	}
	return p, ok
}

func (c *PreparationController) AbandonPendingLaunch(id state.PreparationID) {
	c.mu.Lock()
	p, ok := c.pendingLaunch[id]
	if ok {
		delete(c.pendingLaunch, id)
	}
	w := c.workers[id]
	c.mu.Unlock()
	if w != nil {
		c.StopOwnedWork(id, w.generation)
		_ = c.FS.RemoveAll(w.staging)
		if p.ClonePath == "" {
			_ = c.FS.RemoveAll(w.final)
		}
	}
}

func (c *PreparationController) ReconcileStartup(preps []state.RepositoryPreparation) []state.Event {
	var out []state.Event
	for _, prep := range preps {
		switch prep.Stage {
		case state.PrepStageConfirmed, state.PrepStageCloneInProgress, state.PrepStageDevcontainerInProgress:
			out = append(out, state.EvRepository{Kind: "interrupted", PreparationID: prep.ID})
		case state.PrepStageCloneReady:
			staging := filepath.Join(c.SessionsRoot, "staging", string(prep.ID))
			final := string(prep.ClonePath)
			if final == "" {
				final = filepath.Join(c.SessionsRoot, "clones", string(prep.ID))
			}
			r := gitclone.ReconcileCloneEvidence(gitclone.ReconcileRequest{
				StagingDir: staging, FinalDir: final, ExpectedMarker: markerFor(prep),
				Root: c.SessionsRoot, FS: c.FS,
			})
			switch r.Disposition {
			case gitclone.ReconcileCloneReady:
			case gitclone.ReconcileConflict, gitclone.ReconcileQuarantine:
				out = append(out, state.EvRepository{Kind: "quarantined", PreparationID: prep.ID})
			case gitclone.ReconcileRetryClone:
				out = append(out, state.EvRepository{Kind: "interrupted", PreparationID: prep.ID})
			}
		}
	}
	return out
}

func readDirNames(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range ents {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			out = append(out, name)
		}
	}
	return out, nil
}
