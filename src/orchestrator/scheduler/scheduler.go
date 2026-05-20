package scheduler

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
)

// Scheduler runs the polling loop per SPEC §16.2.
// dispatch and candidate fetch are stubs in P1c; P3 fills them in.
type Scheduler struct {
	workflowPath string
	interval     time.Duration
	state        *State
}

// New returns a Scheduler. cfg.Polling.IntervalMS determines the tick interval.
func New(workflowPath string, cfg wfconfig.Config) *Scheduler {
	return &Scheduler{
		workflowPath: workflowPath,
		interval:     time.Duration(cfg.Polling.IntervalMS) * time.Millisecond,
		state:        NewState(),
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
// Startup preflight must be called by the caller before Run; Run performs
// per-tick re-validation only.
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

// tickOnce runs one poll cycle: re-load, re-resolve, re-preflight, then (stub) dispatch.
func (s *Scheduler) tickOnce(_ context.Context) {
	wf, err := workflowfile.Load(s.workflowPath)
	if err != nil {
		slog.Error("dispatch skipped", "reason", err)
		// P3/P6: reconcile would run here even on preflight failure.
		return
	}
	cfg, err := wfconfig.Resolve(wf.Config, filepath.Dir(s.workflowPath))
	if err != nil {
		slog.Error("dispatch skipped", "reason", err)
		return
	}
	if err := Preflight(cfg); err != nil {
		slog.Error("dispatch skipped", "reason", err)
		// P3/P6: reconcile runs here regardless.
		return
	}
	slog.Info("tick: no candidates fetched, no dispatch (stub)")
	// P3: poll tracker, build candidate list, dispatch agents.
}
