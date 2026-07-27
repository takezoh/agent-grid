package view

import (
	"testing"
	"time"
)

func TestCompositeStatusHumanInputSuppressesSpinnerAtPresentationBoundary(t *testing.T) {
	now := time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC)
	got := CompositeStatus(StatusRunning, now, true, false, nil)
	if got.Lifecycle != LifecycleActive || !got.HumanInput {
		t.Fatalf("composite = %+v", got)
	}
	// The wire retains the independent facts. The shared display precedence
	// selects human_input before active, making spinner+! unrepresentable.
	if got.Failure != FailureNone || got.ChangedAt == nil || !got.ChangedAt.Equal(now) {
		t.Fatalf("composite metadata = %+v", got)
	}
}

func TestCompositeStatusBellIsOrthogonalToHumanInput(t *testing.T) {
	signal := &StatusSignal{Kind: "bell", Epoch: "epoch-a", Sequence: 7}
	got := CompositeStatus(StatusPending, time.Time{}, true, false, signal)
	if got.Signal != signal || !got.HumanInput {
		t.Fatalf("composite = %+v", got)
	}
}

func TestCompositeStatusCarriesDriverFailureIndependently(t *testing.T) {
	got := CompositeStatus(StatusStopped, time.Time{}, false, true, nil)
	if got.Lifecycle != LifecycleStopped || got.Failure != FailureFailed {
		t.Fatalf("composite = %+v", got)
	}
}
