package driver

import (
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

func TestGrokPrepareCreateUsesStableExternalSessionID(t *testing.T) {
	d := NewGrokDriver("")
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	next, launch, err := d.PrepareCreate(d.NewState(now), "0123456789abcdef01234567", "/repo", "grok --model grok-build --effort high", state.LaunchOptions{InitialInput: []byte("hello")})
	if err != nil {
		t.Fatalf("PrepareCreate: %v", err)
	}
	gs := next.(GrokState)
	if len(gs.GrokSessionID) != 36 {
		t.Fatalf("GrokSessionID = %q, want UUID", gs.GrokSessionID)
	}
	for _, want := range []string{"--session-id " + gs.GrokSessionID, "--no-auto-update", "--model grok-build", "--effort high"} {
		if !strings.Contains(launch.Command, want) {
			t.Errorf("launch command %q missing %q", launch.Command, want)
		}
	}
	if string(launch.Options.InitialInput) != "hello" {
		t.Errorf("initial input = %q, want hello", launch.Options.InitialInput)
	}
}

func TestGrokPrepareLaunchResumesPersistedSession(t *testing.T) {
	d := NewGrokDriver("")
	gs := d.NewState(time.Now()).(GrokState)
	gs.GrokSessionID = "01234567-89ab-4cde-af01-23456789abcd"
	plan, err := d.PrepareLaunch(gs, state.LaunchModeColdStart, "/repo", "grok --no-auto-update --session-id stale", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch: %v", err)
	}
	if !strings.Contains(plan.Command, "--resume "+gs.GrokSessionID) || strings.Contains(plan.Command, "--session-id") {
		t.Errorf("resume plan = %q", plan.Command)
	}
}

func TestGrokPersistRestoreAndFork(t *testing.T) {
	d := NewGrokDriver("")
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	parent := GrokState{CommonState: CommonState{Status: state.StatusRunning, StatusChangedAt: now, StartDir: "/repo"}, GrokSessionID: "01234567-89ab-4cde-af01-23456789abcd", Model: "grok-build", Effort: "high"}
	restored := d.Restore(d.Persist(parent), now).(GrokState)
	if restored.GrokSessionID != parent.GrokSessionID || restored.Model != parent.Model || restored.Effort != parent.Effort {
		t.Fatalf("restore = %#v", restored)
	}
	command, ok := d.ForkCommand(parent, "grok")
	if !ok || !strings.Contains(command, "--resume "+parent.GrokSessionID) || !strings.Contains(command, "--fork-session") {
		t.Fatalf("ForkCommand = %q, %v", command, ok)
	}
	child := d.WithForkSessionID(d.ForkChildState(parent, now), "abcdef0123456789abcdef01").(GrokState)
	if child.GrokSessionID == parent.GrokSessionID || child.GrokSessionID == "" {
		t.Fatalf("child session = %q", child.GrokSessionID)
	}
	plan, err := d.PrepareLaunch(child, state.LaunchModeCreate, "/repo", command, state.LaunchOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Command, "--session-id "+child.GrokSessionID) {
		t.Fatalf("fork launch %q does not name child session %q", plan.Command, child.GrokSessionID)
	}
}

func TestGrokDoesNotInferMetadataFromTerminalEvents(t *testing.T) {
	d := NewGrokDriver("")
	gs := d.NewState(time.Now()).(GrokState)
	gs.Model, gs.Effort = "seed", "low"
	next, _, _ := d.Step(gs, state.FrameContext{IsRoot: true}, state.DEvFrameOsc{Title: "grok-4.5 high"})
	got := next.(GrokState)
	if got.Model != "seed" || got.Effort != "low" {
		t.Fatalf("terminal event inferred metadata: %#v", got)
	}
}
