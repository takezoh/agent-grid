package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func dispCfg() wfconfig.Config {
	return wfconfig.Config{
		Tracker: wfconfig.TrackerConfig{
			ActiveStates:   []string{"In Progress", "Todo"},
			TerminalStates: []string{"Done"},
		},
		Agent: wfconfig.AgentConfig{
			MaxConcurrentAgents: 3,
			MaxRetryBackoffMS:   60_000,
		},
	}
}

func makeIssue(id, state string) tracker.Issue {
	return tracker.Issue{ID: id, Identifier: "P-" + id, Title: "t", State: state}
}

// TestDispatchOnce_EligibleIssueSpawned verifies a basic eligible dispatch.
func TestDispatchOnce_EligibleIssueSpawned(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue 1 in running")
	}
}

// TestDispatchOnce_GlobalSlotsCap verifies only MaxConcurrentAgents issues are dispatched.
func TestDispatchOnce_GlobalSlotsCap(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "In Progress"),
		makeIssue("4", "In Progress"),
	}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgents = 2
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns (global cap), got %d", spawn.callCount())
	}
}

// TestDispatchOnce_PerStateSlotsCap verifies per-state limits are respected.
func TestDispatchOnce_PerStateSlotsCap(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "Todo"),
	}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgentsByState = map[string]int{"in progress": 1}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

	snap := st.Snapshot()
	// "In Progress" cap=1, "Todo" uses global (3) — so 1 + 1 = 2 total
	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns, got %d", spawn.callCount())
	}
	if _, ok := snap.Running["3"]; !ok {
		t.Error("want todo issue dispatched")
	}
}

// TestDispatchOnce_SpawnFailSchedulesRetry verifies spawn failure leads to retry.
func TestDispatchOnce_SpawnFailSchedulesRetry(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{err: errors.New("oops")}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg())

	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue 1 not in running after spawn fail")
	}
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released after spawn fail")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want retry entry after spawn fail")
	}
}

// TestHandleRetryFire_IssueNotFound releases the claim.
func TestHandleRetryFire_IssueNotFound(t *testing.T) {
	st := NewState()
	// Put issue in RetryAttempts to simulate retry state.
	st.EnqueueRetry(RetryEntry{IssueID: "1", Identifier: "P-1"})
	tr := &fakeTracker{issues: []tracker.Issue{}} // empty — not found
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	snap := st.Snapshot()
	if _, ok := snap.RetryAttempts["1"]; ok {
		t.Error("want retry cleared after not-found release")
	}
}

// TestHandleRetryFire_NotActive releases the claim.
func TestHandleRetryFire_NotActive(t *testing.T) {
	st := NewState()
	st.EnqueueRetry(RetryEntry{IssueID: "1"})
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "Done")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Error("want no spawn for non-active issue")
	}
}

// TestHandleRetryFire_EligibleAndSlots dispatches and marks running.
func TestHandleRetryFire_EligibleAndSlots(t *testing.T) {
	st := NewState()
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue in running after retry dispatch")
	}
}

// TestHandleRetryFire_NoSlots requeues the issue with attempt+1 and backoff delay (SPEC §8.4).
func TestHandleRetryFire_NoSlots(t *testing.T) {
	st := NewState()
	// Fill all global slots.
	for i := range 3 {
		id := string(rune('a' + i))
		iss := makeIssue(id, "In Progress")
		_ = st.Dispatch(iss, 1, LiveSession{}, time.Now())
	}
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cfg := dispCfg()
	const reqAttempt = 2
	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: reqAttempt}, tr, st, clk, fireCh, spawn.fn, cfg)

	if spawn.callCount() != 0 {
		t.Error("want no spawn when slots exhausted")
	}
	snap := st.Snapshot()
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want requeue when no slots")
	}
	// SPEC §8.4: attempt must be incremented so backoff grows.
	wantAttempt := reqAttempt + 1
	if entry.Attempt != wantAttempt {
		t.Errorf("want attempt %d, got %d", wantAttempt, entry.Attempt)
	}
	// SPEC §8.4: must use failure backoff, not the 1s continuation delay.
	wantDelay := backoffDelay(wantAttempt, cfg)
	wantDueAtMS := clk.Now().Add(wantDelay).UnixMilli()
	if entry.DueAtMS != wantDueAtMS {
		t.Errorf("want DueAtMS %d (backoff=%v), got %d", wantDueAtMS, wantDelay, entry.DueAtMS)
	}
	// The error must indicate slot exhaustion, not be nil (which would mark it as a continuation).
	if entry.Err == nil {
		t.Error("want non-nil Err for slot-exhaustion requeue")
	}
}

// TestHandleRetryFire_NoSlots_FiresWithIncrementedAttempt verifies the timer fires with attempt+1.
func TestHandleRetryFire_NoSlots_FiresWithIncrementedAttempt(t *testing.T) {
	st := NewState()
	for i := range 3 {
		id := string(rune('a' + i))
		_ = st.Dispatch(makeIssue(id, "In Progress"), 1, LiveSession{}, time.Now())
	}
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	// Advance clock to trigger the scheduled timer.
	clk.Advance(backoffDelay(3, dispCfg()))

	select {
	case fired := <-fireCh:
		if fired.Attempt != 3 {
			t.Errorf("want fired attempt=3, got %d", fired.Attempt)
		}
	default:
		t.Error("want timer to fire after backoff delay")
	}
}
