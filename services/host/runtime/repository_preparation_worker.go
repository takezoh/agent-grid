package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

type PreparationController struct {
	mu sync.Mutex

	Store        PreparationStore
	FS           gitclone.FilesystemProcess
	Cloner       gitclone.Cloner
	Authority    CloneAuthority
	SessionsRoot string // repository sessions root (mount scope)
	Cap          *PreparationCapacity
	Enqueue      func(state.Event)
	Now          func() time.Time

	workers        map[state.PreparationID]*prepWorker
	pendingLaunch  map[state.PreparationID]pendingTerminalLaunch
	neverAvailSize map[state.PreparationID]int64 // bytes counted against never-available cap
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
		Store:          store,
		FS:             fs,
		Cloner:         cloner,
		SessionsRoot:   root,
		Cap:            NewPreparationCapacity(),
		Enqueue:        enqueue,
		Now:            time.Now,
		workers:        map[state.PreparationID]*prepWorker{},
		pendingLaunch:  map[state.PreparationID]pendingTerminalLaunch{},
		neverAvailSize: map[state.PreparationID]int64{},
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
	// cloneBytes=0: pre-clone size unknown; AdmitClone still enforces NeverAvailBytes cap.
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

// AccountClonePath charges never-available capacity for a published clone that
// is not yet a session-available workspace.
func (c *PreparationController) AccountClonePath(id state.PreparationID, path string) {
	if c == nil || path == "" {
		return
	}
	n := DirBytes(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.neverAvailSize[id]; ok {
		c.Cap.RemoveNeverAvailable(prev)
	}
	c.neverAvailSize[id] = n
	c.Cap.AddNeverAvailable(n)
}

// ReleaseNeverAvailableAccount drops capacity charge without deleting files
// (session became available and now owns the clone).
func (c *PreparationController) ReleaseNeverAvailableAccount(id state.PreparationID) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.neverAvailSize[id]; ok {
		c.Cap.RemoveNeverAvailable(prev)
		delete(c.neverAvailSize, id)
	}
}

// CleanupNeverAvailable removes non-available session clone/staging resources
// and releases capacity. Never deletes shared repository environments or
// session-available clones.
func (c *PreparationController) CleanupNeverAvailable(prep state.RepositoryPreparation) {
	if c == nil || prep.IsSessionAvailable() || prep.Stage == state.PrepStageAvailable {
		return
	}
	staging := filepath.Join(c.SessionsRoot, "staging", string(prep.ID))
	final := string(prep.ClonePath)
	if final == "" {
		final = filepath.Join(c.SessionsRoot, "clones", string(prep.ID))
	}
	_ = c.FS.RemoveAll(staging)
	// Only remove final when it is under sessions root and not session-available.
	if final != "" && c.SessionsRoot != "" {
		cleanRoot := filepath.Clean(c.SessionsRoot)
		cleanFinal := filepath.Clean(final)
		if cleanFinal == cleanRoot || hasSessionsPrefix(cleanFinal, cleanRoot) {
			_ = c.FS.RemoveAll(final)
		}
	}
	c.ReleaseNeverAvailableAccount(prep.ID)
}

func hasSessionsPrefix(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel)
}

// SweepNeverAvailableOrphans deletes never-available preparation clones whose
// durable records are older than PrepCapCleanupAge. Skips available sessions
// and does not touch repository-environments/.
func (c *PreparationController) SweepNeverAvailableOrphans() int {
	if c == nil || c.Store == nil {
		return 0
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	recs, err := c.Store.List()
	if err != nil {
		return 0
	}
	cleaned := 0
	for _, rec := range recs {
		prep := rec.Preparation
		if prep.IsSessionAvailable() || prep.Stage == state.PrepStageAvailable {
			continue
		}
		switch prep.Stage {
		case state.PrepStageFailed, state.PrepStageCancelled, state.PrepStageAbandoned,
			state.PrepStageInternalCorruption, state.PrepStageInterrupted:
		default:
			// In-flight stages are owned by live workers, not age sweep.
			continue
		}
		updated := rec.UpdatedAt
		if updated.IsZero() || now.Sub(updated) < PrepCapCleanupAge {
			continue
		}
		c.CleanupNeverAvailable(prep)
		_ = c.Store.Delete(prep.ID)
		cleaned++
	}
	return cleaned
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
		// Failed staging still occupies disk until removed; charge then free.
		c.AccountClonePath(prep.ID, w.staging)
		_ = c.FS.RemoveAll(w.staging)
		c.ReleaseNeverAvailableAccount(prep.ID)
		if pub.Code == gitclone.CodeCloneIdentityConflict {
			c.Enqueue(state.EvRepository{Kind: "quarantined", PreparationID: prep.ID})
			return
		}
		c.fail(prep, state.PrepFailureCloneFailed, pub.Code)
		return
	}
	// Published clone is not yet session-available → counts toward never-available cap.
	c.AccountClonePath(prep.ID, w.final)
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
				out = append(out, state.EvRepository{
					Kind: "quarantined", PreparationID: prep.ID, ObservedRevision: prep.Revision,
				})
			case gitclone.ReconcileRetryClone:
				out = append(out, state.EvRepository{Kind: "interrupted", PreparationID: prep.ID})
			case gitclone.ReconcileUnsupported:
				out = append(out, state.EvRepository{Kind: "interrupted", PreparationID: prep.ID})
			}
		case state.PrepStageFailed, state.PrepStageCancelled, state.PrepStageInterrupted,
			state.PrepStageInternalCorruption, state.PrepStageAvailable, state.PrepStageAbandoned:
			// Terminal or already-projected stages need no startup rewrite.
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
