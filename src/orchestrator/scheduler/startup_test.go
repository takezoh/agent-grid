package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

func newStartupScheduler(tr schedulerTrackerAPI, ws schedulerWorkspaceAPI) *Scheduler {
	return &Scheduler{
		state:     NewState(),
		tracker:   tr,
		workspace: ws,
		clock:     time.Now,
	}
}

func TestStartupCleanup_RemovesTerminalWorkspaces(t *testing.T) {
	ws := &fakeWorkspace{}
	tr := &fakeTracker{
		terminalIssues: []ptrackerv.Issue{
			{ID: "t1", Identifier: "PROJ-10"},
			{ID: "t2", Identifier: "PROJ-11"},
		},
	}
	s := newStartupScheduler(tr, ws)
	s.StartupCleanup(context.Background())

	if len(ws.removed) != 2 {
		t.Fatalf("expected 2 removes, got %d: %v", len(ws.removed), ws.removed)
	}
}

func TestStartupCleanup_FetchFailureContinues(t *testing.T) {
	ws := &fakeWorkspace{}
	tr := &fakeTracker{terminalErr: errors.New("api down")}
	s := newStartupScheduler(tr, ws)

	// must not panic or return error
	s.StartupCleanup(context.Background())

	if len(ws.removed) != 0 {
		t.Error("expected no removes when fetch fails")
	}
}
