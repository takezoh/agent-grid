package driver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/driver/drivertest"
	"github.com/takezoh/agent-grid/services/host/state"
)

func agyHook(t *testing.T, event string, fields map[string]any, at time.Time) state.DEvHook {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return state.DEvHook{Event: event, Timestamp: at, Payload: raw}
}

func TestAgyHookLifecycle(t *testing.T) {
	now := time.Unix(100, 0)
	d := NewAgyDriver()
	s := d.NewState(now)
	ctx := state.FrameContext{IsRoot: true}
	fixture := drivertest.FakeAgyLifecycle(now.Add(time.Second))
	s, _, _ = d.Step(s, ctx, fixture[0])
	if got := d.Status(s); got != state.StatusRunning {
		t.Fatalf("PreInvocation status = %v, want Running", got)
	}

	s, _, _ = d.Step(s, ctx, fixture[1])
	if got := d.Status(s); got != state.StatusWaiting {
		t.Fatalf("fully-idle Stop status = %v, want Waiting", got)
	}
}

func TestAgyNonIdleAndStaleStopDoNotClearRunning(t *testing.T) {
	now := time.Unix(200, 0)
	d := NewAgyDriver()
	ctx := state.FrameContext{IsRoot: true}
	s := d.NewState(now)
	s, _, _ = d.Step(s, ctx, agyHook(t, "PreInvocation", map[string]any{
		"invocationNum": 4,
	}, now.Add(time.Second)))

	for _, payload := range []map[string]any{
		{"executionNum": 4, "fullyIdle": false},
		{"executionNum": 3, "fullyIdle": true},
	} {
		s, _, _ = d.Step(s, ctx, agyHook(t, "Stop", payload, now.Add(2*time.Second)))
		if got := d.Status(s); got != state.StatusRunning {
			t.Fatalf("Stop %#v status = %v, want Running", payload, got)
		}
	}
}

func TestAgyEscHasNoSyntheticTimeoutTransition(t *testing.T) {
	now := time.Unix(300, 0)
	d := NewAgyDriver()
	ctx := state.FrameContext{IsRoot: true}
	s := d.NewState(now)
	s, _, _ = d.Step(s, ctx, agyHook(t, "PreInvocation", map[string]any{
		"invocationNum": 1,
	}, now.Add(time.Second)))
	s, _, _ = d.Step(s, ctx, state.DEvTick{Now: now.Add(24 * time.Hour), Watched: true})
	if got := d.Status(s); got != state.StatusRunning {
		t.Fatalf("elapsed time changed agy status to %v, want Running", got)
	}
}
