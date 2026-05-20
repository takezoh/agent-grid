package scheduler

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// schedulerTrackerAPI is the tracker surface used by the scheduler.
// Satisfied by *orchestrator/tracker.Tracker; fakes implement it in tests.
type schedulerTrackerAPI interface {
	RefreshStates(ctx context.Context, ids []string) ([]ptrackerv.Issue, error)
	TerminalIssues(ctx context.Context) ([]ptrackerv.Issue, error)
}

// schedulerWorkspaceAPI is the workspace surface used by the scheduler.
// Satisfied by *orchestrator/workspace.Manager; fakes implement it in tests.
type schedulerWorkspaceAPI interface {
	Remove(ctx context.Context, identifier string) error
}

// Scheduler runs the polling loop per SPEC §16.2.
type Scheduler struct {
	workflowPath string
	interval     time.Duration
	state        *State
	tracker      schedulerTrackerAPI
	workspace    schedulerWorkspaceAPI
	clock        func() time.Time
}

// New returns a Scheduler.
func New(workflowPath string, cfg wfconfig.Config, tr schedulerTrackerAPI, ws schedulerWorkspaceAPI) *Scheduler {
	return &Scheduler{
		workflowPath: workflowPath,
		interval:     time.Duration(cfg.Polling.IntervalMS) * time.Millisecond,
		state:        NewState(),
		tracker:      tr,
		workspace:    ws,
		clock:        time.Now,
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	slog.Info("scheduler starting", "interval_ms", s.interval.Milliseconds())
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return nil
		case <-ticker.C:
			s.tickOnce(ctx)
		}
	}
}

// tickOnce runs one poll cycle: load config, reconcile, preflight, then (stub) dispatch.
// Reconcile runs before preflight per SPEC §8.1 step 1.
func (s *Scheduler) tickOnce(ctx context.Context) {
	wf, err := workflowfile.Load(s.workflowPath)
	if err != nil {
		slog.Error("tick: config load failed, skipping cycle", "reason", err)
		return
	}
	cfg, err := wfconfig.Resolve(wf.Config, filepath.Dir(s.workflowPath))
	if err != nil {
		slog.Error("tick: config resolve failed, skipping cycle", "reason", err)
		return
	}

	s.reconcile(ctx, cfg)

	if err := Preflight(cfg); err != nil {
		slog.Error("dispatch skipped", "reason", err)
		return
	}
	slog.Info("tick: no candidates fetched, no dispatch (stub)")
	// P3: poll tracker, build candidate list, dispatch agents.
}
