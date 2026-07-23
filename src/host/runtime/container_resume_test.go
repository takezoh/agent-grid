package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/driver"
	"github.com/takezoh/agent-grid/host/event"
	"github.com/takezoh/agent-grid/host/runtime/framereg"
	"github.com/takezoh/agent-grid/host/state"
)

// TestContainerResume_sessionIDSurvivesRegistrationWindow is the end-to-end
// regression test for "claude --resume stopped working in containers". It ties
// the whole chain together: a containerized agent emits SessionStart (carrying
// session_id) while the daemon is still bringing up the per-frame endpoint, the
// hook must still be delivered, the claude driver must capture ClaudeSessionID,
// and after a persist/restore round-trip a sandboxed cold start must emit
// `claude --resume <id>`.
//
// Without the DeliverHookEvent retry the SessionStart hook is dropped (endpoint
// not listening yet), ClaudeSessionID is never captured/persisted, and the
// cold-start launch command has no --resume — exactly the user-visible failure.
func TestContainerResume_sessionIDSurvivesRegistrationWindow(t *testing.T) {
	dir := t.TempDir()
	sock := ContainerSockPath(dir)
	reg := framereg.New()
	fid := state.FrameID("f1")
	tok := "tok-resume"
	const sid = "claude-uuid-resume"
	reg.RegisterWithMounts(fid, tok, nil)

	evCh := make(chan state.Event, 1)
	epCh := make(chan *containerEndpoint, 1)
	go func() {
		// Endpoint comes up only after the agent has already emitted its first
		// hook — the registration window the spawn refactor opened.
		time.Sleep(120 * time.Millisecond)
		ep, err := startContainerEndpoint(sock, reg, func(ev state.Event) { evCh <- ev }, nil)
		if err != nil {
			epCh <- nil
			return
		}
		epCh <- ep
	}()

	payload, _ := json.Marshal(map[string]string{
		"session_id":      sid,
		"hook_event_name": "SessionStart",
	})
	// The agent's hook sender; with retry this blocks until the endpoint is up.
	_ = event.DeliverHookEvent(sock, tok, "SessionStart", time.Now(), payload)

	// Mirror reduceDriverHook → ClaudeDriver.Step to absorb the delivered hook.
	d := driver.NewClaudeDriver(t.TempDir(), "/data/events", driver.ClaudeOptions{}, "less")
	cs := d.NewState(time.Now())
	select {
	case ev := <-evCh:
		de, ok := ev.(state.EvDriverEvent)
		if !ok {
			t.Fatalf("expected EvDriverEvent, got %T", ev)
		}
		next, _, _ := d.Step(cs, state.FrameContext{IsRoot: true}, state.DEvHook{
			Event:          de.Event,
			Timestamp:      de.Timestamp,
			RoostSessionID: string(de.SenderID),
			Payload:        de.Payload,
		})
		cs = next
	case <-time.After(time.Second):
		// Hook never arrived (the bug): ClaudeSessionID stays empty.
	}
	if ep := <-epCh; ep != nil {
		ep.close()
	}

	// Persist/restore round-trip, then assert sandboxed cold start resumes.
	restored := d.Restore(d.Persist(cs), time.Now())
	plan, err := d.PrepareLaunch(restored, state.LaunchModeColdStart, "/repo", "claude", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch: %v", err)
	}
	if !strings.Contains(plan.Command, "--resume "+sid) {
		t.Fatalf("container resume unavailable: session id was not captured across the registration window; got %q", plan.Command)
	}
}
