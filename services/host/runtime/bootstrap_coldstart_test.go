package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

type trackingLauncher struct {
	mu         sync.Mutex
	calls      map[string]int
	delay      time.Duration
	failOn     string
	wrapCalled bool
}

func (l *trackingLauncher) WrapLaunch(_ state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	l.mu.Lock()
	l.wrapCalled = true
	l.mu.Unlock()
	return WrappedLaunch{Command: plan.Command, StartDir: plan.StartDir, Env: env}, nil
}

func (l *trackingLauncher) IsContainer(_ string) bool { return false }

func (l *trackingLauncher) EnsureProject(_ context.Context, projectPath string) error {
	if l.delay > 0 {
		time.Sleep(l.delay)
	}
	l.mu.Lock()
	l.calls[projectPath]++
	l.mu.Unlock()
	if projectPath == l.failOn {
		return errors.New("injected failure")
	}
	return nil
}

func registerMinimalDriver(t *testing.T) {
	t.Helper()
	saved := state.GetRegistry()
	t.Cleanup(func() {
		state.ClearRegistry()
		for _, d := range saved {
			state.Register(d)
		}
	})
	if _, ok := saved[minimalDriver{}.Name()]; !ok {
		state.Register(minimalDriver{})
	}
}

func TestRecreateAll_SpawnFailureLeavesSessionInState(t *testing.T) {
	registerMinimalDriver(t)
	backend := newFakeBackend()
	backend.spawnErr = errors.New("injected spawn failure")
	persist := &recordingPersist{}
	r := New(Config{
		Backend:  backend,
		Launcher: &trackingLauncher{calls: make(map[string]int)},
		Persist:  persist,
	})
	r.state.Sessions["s1"] = state.Session{
		ID:      "s1",
		Project: "/proj/a",
		Frames: []state.SessionFrame{{
			ID: "f1", Project: "/proj/a", Command: "minimal-test",
			Driver: state.DriverStateBase{},
		}},
	}

	if err := r.RecreateAll(); err != nil {
		t.Fatalf("RecreateAll: %v", err)
	}
	if _, ok := r.state.Sessions["s1"]; !ok {
		t.Error("session s1 was removed from state, want it preserved")
	}
	persist.mu.Lock()
	saves := persist.saves
	persist.mu.Unlock()
	if saves != 0 {
		t.Errorf("Persist.Save called %d times, want 0", saves)
	}
}

func TestSkipColdStartSpawn(t *testing.T) {
	now := time.Now()
	codex := state.GetDriver("codex")
	generic := state.GetDriver("generic")
	if codex == nil || generic == nil {
		t.Fatal("codex and generic drivers must be registered (see TestMain)")
	}
	mk := func(drv state.Driver, command string, bag map[string]string) state.SessionFrame {
		return state.SessionFrame{ID: "f1", Command: command, Driver: drv.Restore(bag, now)}
	}
	const thread = "019e727e-fde4-7432-9036-ae6604ce1b27"
	tests := []struct {
		name     string
		frame    state.SessionFrame
		wantSkip bool
	}{
		{
			name:     "stopped codex with resumable thread is relaunched",
			frame:    mk(codex, "codex", map[string]string{"status": "stopped", "thread_id": thread}),
			wantSkip: false,
		},
		{
			name:     "stopped codex without locator is skipped",
			frame:    mk(codex, "codex", map[string]string{"status": "stopped"}),
			wantSkip: true,
		},
		{
			name:     "stopped codex with rollout path is relaunched",
			frame:    mk(codex, "codex", map[string]string{"status": "stopped", "rollout_path": "/repo/rollout.jsonl"}),
			wantSkip: false,
		},
		{
			name:     "idle codex is not skipped",
			frame:    mk(codex, "codex", map[string]string{"status": "idle", "thread_id": thread}),
			wantSkip: false,
		},
		{
			name:     "stopped generic is skipped",
			frame:    mk(generic, "generic", map[string]string{"status": "stopped"}),
			wantSkip: true,
		},
		{
			name:     "running generic is not skipped",
			frame:    mk(generic, "generic", map[string]string{"status": "running"}),
			wantSkip: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skipColdStartSpawn(tt.frame); got != tt.wantSkip {
				t.Errorf("skipColdStartSpawn() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}

// TestRecreateAll_ContinuesPastFailingFrame guards the regression where one
// frame's cold-start spawn failure (e.g. a codex resume against a vanished
// thread) aborted the loop and stranded its healthy siblings. With two frames
// and every spawn failing, both must still be attempted.
func TestRecreateAll_ContinuesPastFailingFrame(t *testing.T) {
	registerMinimalDriver(t)
	backend := newFakeBackend()
	backend.spawnErr = errors.New("injected spawn failure")
	r := New(Config{
		Backend:  backend,
		Launcher: &trackingLauncher{calls: make(map[string]int)},
		Persist:  &recordingPersist{},
	})
	r.state.Sessions["s1"] = state.Session{
		ID:      "s1",
		Project: "/proj/a",
		Frames: []state.SessionFrame{
			{ID: "f1", Project: "/proj/a", Command: "minimal-test", Driver: state.DriverStateBase{}},
			{ID: "f2", Project: "/proj/a", Command: "minimal-test", Driver: state.DriverStateBase{}},
		},
	}

	if err := r.RecreateAll(); err != nil {
		t.Fatalf("RecreateAll: %v", err)
	}
	backend.mu.Lock()
	calls := backend.spawnCalls
	backend.mu.Unlock()
	if calls != 2 {
		t.Errorf("SpawnFrame calls = %d, want 2 (a failed frame must not abort its siblings)", calls)
	}
	if _, ok := r.state.Sessions["s1"]; !ok {
		t.Error("session s1 should be preserved after partial spawn failure")
	}
}

func TestSpawnFrameWindow_StreamSubsystemInjectsEndpointDir(t *testing.T) {
	t.Skip("endpoint-dir injection was removed with the codex helper")
}
