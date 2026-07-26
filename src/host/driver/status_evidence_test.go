package driver

import (
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/state"
)

func TestStatusEvidenceActionRequiredWinsOverTerminalRunning(t *testing.T) {
	now := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	common := CommonState{Status: state.StatusWaiting, StatusChangedAt: now}
	common.ObserveStatus(StatusSourceHook, state.StatusPending, now)
	common.ObserveStatus(StatusSourceTerminal, state.StatusRunning, now.Add(time.Second))
	if common.Status != state.StatusPending {
		t.Fatalf("status = %v, want pending", common.Status)
	}
	common.ObserveStatus(StatusSourceHook, state.StatusRunning, now.Add(2*time.Second))
	if common.Status != state.StatusRunning {
		t.Fatalf("resolved status = %v, want running", common.Status)
	}
}

func TestStatusEvidenceNewerWaitingClearsRunning(t *testing.T) {
	now := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	common := CommonState{Status: state.StatusWaiting, StatusChangedAt: now}
	common.ObserveStatus(StatusSourceHook, state.StatusRunning, now)
	common.ObserveStatus(StatusSourceTerminal, state.StatusWaiting, now.Add(time.Second))
	if common.Status != state.StatusWaiting {
		t.Fatalf("status = %v, want waiting", common.Status)
	}
}

func TestStatusEvidenceDoesNotChangeWithoutExplicitEvidence(t *testing.T) {
	now := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	common := CommonState{Status: state.StatusRunning, StatusChangedAt: now}
	common.ResolveStatus(now.Add(24 * time.Hour))
	if common.Status != state.StatusRunning {
		t.Fatalf("status changed by elapsed time: %v", common.Status)
	}
}

func TestProgressErrorIsActivityNotProcessStopped(t *testing.T) {
	if got := progressStatus(2); got != state.StatusRunning {
		t.Fatalf("progress state 2 = %v, want Running", got)
	}
}

func TestStatusEvidenceStoppedProcessWinsOverLaterTerminalEvidence(t *testing.T) {
	now := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	common := CommonState{Status: state.StatusRunning, StatusChangedAt: now}
	common.ObserveStatus(StatusSourceProcess, state.StatusStopped, now)
	common.ObserveStatus(StatusSourceTerminal, state.StatusRunning, now.Add(time.Second))
	if common.Status != state.StatusStopped {
		t.Fatalf("status = %v, want stopped", common.Status)
	}
}
