package scheduler

import (
	"errors"
	"sync"
	"testing"

	"github.com/takezoh/agent-roost/platform/tracker"
)

func testIssue(id, identifier string) tracker.Issue {
	return tracker.Issue{ID: id, Identifier: identifier, Title: "t"}
}

func TestStateDispatch_AddsToRunningAndClaimed(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	session := LiveSession{SessionID: "s1"}

	if err := s.Dispatch(issue, 1, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Claimed["id1"]; !ok {
		t.Error("expected id1 in claimed")
	}
	run, ok := snap.Running["id1"]
	if !ok {
		t.Fatal("expected id1 in running")
	}
	if run.Issue.Identifier != "PROJ-1" {
		t.Errorf("got identifier %q, want PROJ-1", run.Issue.Identifier)
	}
	if run.Attempt != 1 {
		t.Errorf("got attempt %d, want 1", run.Attempt)
	}
}

func TestStateDispatch_DuplicateRejected(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	session := LiveSession{SessionID: "s1"}

	if err := s.Dispatch(issue, 1, session); err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	err := s.Dispatch(issue, 2, session)
	if !errors.Is(err, ErrDuplicateDispatch) {
		t.Errorf("expected ErrDuplicateDispatch, got %v", err)
	}
}

func TestStateDispatch_ClearsPriorRetry(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	s.EnqueueRetry(RetryEntry{IssueID: "id1", Kind: RetryBackoff})

	if err := s.Dispatch(issue, 2, LiveSession{}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.RetryAttempts["id1"]; ok {
		t.Error("expected retry entry cleared on Dispatch")
	}
}

func TestStateWorkerExitNormal_RemovesRunningAndReturnsContinuation(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	if err := s.Dispatch(issue, 1, LiveSession{}); err != nil {
		t.Fatal(err)
	}

	entry, ok := s.WorkerExitNormal("id1")
	if !ok {
		t.Fatal("expected WorkerExitNormal to return ok=true")
	}
	if entry.Kind != RetryContinuation {
		t.Errorf("got kind %v, want RetryContinuation", entry.Kind)
	}
	if entry.Attempt != 1 {
		t.Errorf("got attempt %d, want 1", entry.Attempt)
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["id1"]; ok {
		t.Error("expected id1 removed from running")
	}
}

func TestStateWorkerExitAbnormal_RemovesRunningAndReturnsBackoff(t *testing.T) {
	s := NewState()
	issue := testIssue("id2", "PROJ-2")
	if err := s.Dispatch(issue, 1, LiveSession{}); err != nil {
		t.Fatal(err)
	}

	someErr := errors.New("agent crashed")
	entry, ok := s.WorkerExitAbnormal("id2", someErr, 1)
	if !ok {
		t.Fatal("expected WorkerExitAbnormal to return ok=true")
	}
	if entry.Kind != RetryBackoff {
		t.Errorf("got kind %v, want RetryBackoff", entry.Kind)
	}
	if entry.Attempt != 2 {
		t.Errorf("got attempt %d, want 2 (attempt+1)", entry.Attempt)
	}
	if !errors.Is(entry.Err, someErr) {
		t.Errorf("got err %v, want someErr", entry.Err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["id2"]; ok {
		t.Error("expected id2 removed from running")
	}
}

func TestStateReleaseClaim_RemovesFromAllMaps(t *testing.T) {
	s := NewState()
	issue := testIssue("id3", "PROJ-3")
	if err := s.Dispatch(issue, 1, LiveSession{}); err != nil {
		t.Fatal(err)
	}
	s.EnqueueRetry(RetryEntry{IssueID: "id3"})

	s.ReleaseClaim("id3")

	snap := s.Snapshot()
	if _, ok := snap.Running["id3"]; ok {
		t.Error("expected id3 removed from running")
	}
	if _, ok := snap.Claimed["id3"]; ok {
		t.Error("expected id3 removed from claimed")
	}
	if _, ok := snap.RetryAttempts["id3"]; ok {
		t.Error("expected id3 removed from retryAttempts")
	}
}

func TestStateSnapshot_IsDeepCopy(t *testing.T) {
	s := NewState()
	issue := testIssue("id4", "PROJ-4")
	if err := s.Dispatch(issue, 1, LiveSession{}); err != nil {
		t.Fatal(err)
	}

	snap := s.Snapshot()
	delete(snap.Running, "id4")
	delete(snap.Claimed, "id4")

	snap2 := s.Snapshot()
	if _, ok := snap2.Running["id4"]; !ok {
		t.Error("State.running was mutated by modifying Snapshot")
	}
	if _, ok := snap2.Claimed["id4"]; !ok {
		t.Error("State.claimed was mutated by modifying Snapshot")
	}
}

func TestStateConcurrentDispatch_NoDuplicate(t *testing.T) {
	s := NewState()
	issue := testIssue("id5", "PROJ-5")

	const goroutines = 20
	results := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx] = s.Dispatch(issue, 1, LiveSession{})
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrDuplicateDispatch) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful Dispatch, got %d", successes)
	}
}
