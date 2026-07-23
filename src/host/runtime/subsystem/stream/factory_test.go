package stream

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/host/state"
)

func TestFactoryMakeIDIsSessionKeyed(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{}}
	id := f.makeID("abc123")
	if want := state.SubsystemID("stream:session:abc123"); id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

func TestFactoryMakeIDDifferentSessionsDifferentIDs(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{}}
	idA := f.makeID("session-a")
	idB := f.makeID("session-b")
	if idA == idB {
		t.Fatalf("different sessions must not collide: %q", idA)
	}
}

func TestFactory_EnsureSharesBackendWithinSession(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	sessID := state.SessionID("sess-shared")
	sharedID := state.SubsystemID("stream:session:sess-shared")
	sentinel := &Backend{subsystemID: sharedID}
	f.backends[sharedID] = sentinel

	plan := state.LaunchPlan{Command: "codex"}
	subA, idA, errA := f.Ensure(context.Background(), sessID, "/workspace/a", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, idB, errB := f.Ensure(context.Background(), sessID, "/workspace/b", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if idA != sharedID || idB != sharedID {
		t.Errorf("same session: IDs must match; A=%q B=%q want=%q", idA, idB, sharedID)
	}
	if subA != sentinel || subB != sentinel {
		t.Errorf("Ensure returned different Backend instances for the same session")
	}
	if got := len(f.backends); got != 1 {
		t.Errorf("backend count = %d, want 1 (one app-server per session)", got)
	}
}

func TestFactory_EnsureDifferentSessionsDifferentBackends(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	idA := state.SubsystemID("stream:session:sess-a")
	idB := state.SubsystemID("stream:session:sess-b")
	backendA := &Backend{subsystemID: idA}
	backendB := &Backend{subsystemID: idB}
	f.backends[idA] = backendA
	f.backends[idB] = backendB

	plan := state.LaunchPlan{Command: "codex"}
	subA, gotIDA, errA := f.Ensure(context.Background(), "sess-a", "/workspace/a", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, gotIDB, errB := f.Ensure(context.Background(), "sess-b", "/workspace/b", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if gotIDA == gotIDB {
		t.Fatalf("different sessions must get different IDs: %q", gotIDA)
	}
	if subA == subB {
		t.Errorf("different sessions must get different Backend instances")
	}
	if subA != backendA {
		t.Errorf("session-a: got wrong Backend")
	}
	if subB != backendB {
		t.Errorf("session-b: got wrong Backend")
	}
}

func TestFactory_RemoveStopsAndDeletesBackend(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	stopped := false
	b := &Backend{
		subsystemID: "stream:session:sess-rm",
		cancel:      func() {},
		done:        make(chan struct{}),
	}
	close(b.done) // simulate already stopped
	_ = stopped
	f.backends["stream:session:sess-rm"] = b

	f.Remove(context.Background(), "stream:session:sess-rm")

	if _, ok := f.backends["stream:session:sess-rm"]; ok {
		t.Errorf("backend not removed after Remove")
	}
}

func TestBackend_BindThreadRegistersMultipleFrameBindings(t *testing.T) {
	b := New(nil, nil, "stream:session:sess1", "sess1", "/workspace/agent-roost",
		"codex", nil, "", "", false, false,
		"/opt/agent-grid/run/codex.sock",
		0,
	)

	frameA := state.FrameID("frame-a")
	frameB := state.FrameID("frame-b")

	b.frames[frameA] = &frameBinding{frameID: frameA, threadID: "thread-a"}
	b.threads["thread-a"] = frameA
	b.frames[frameB] = &frameBinding{frameID: frameB, threadID: "thread-b"}
	b.threads["thread-b"] = frameB

	if got := len(b.frames); got != 2 {
		t.Fatalf("frame bindings = %d, want 2 (one app-server, two frames)", got)
	}
	if b.frameForThread("thread-a") != frameA {
		t.Errorf("thread-a → frame mapping lost")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("thread-b → frame mapping lost")
	}

	b.ReleaseFrame(frameA)
	if _, exists := b.frames[frameA]; exists {
		t.Errorf("released frameA still in frames map")
	}
	if _, exists := b.threads["thread-a"]; exists {
		t.Errorf("released frameA's thread-a still in threads map")
	}
	if _, exists := b.frames[frameB]; !exists {
		t.Errorf("frameB was unexpectedly removed when releasing frameA")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("frameB → thread-b mapping lost after releasing A")
	}
}

// TestFactory_EnsurePropagatesHostOverrideToAppServer pins the invariant that
// a per-launch state.SandboxOverrideHost must reach the app-server dispatch
// via plan.ForceHost, even when the project's default sandbox mode is
// devcontainer. Without this, Factory.Ensure computes b.isContainer from
// IsContainer(project) alone; spawn.go then hands the app-server plan to
// SandboxDispatcher without ForceHost; the dispatcher routes to Devcontainer;
// the app-server binds a container-absolute listenSock while the frame runs
// on host — the two cannot share the UDS and the codex frame exits 1 within
// seconds. Observed in production at 2026-07-15T03:22:11 UTC (session
// 1d7d4775ee9a9679c638b3fe, frame 1a516a1f495d8cc47a137e8e).
func TestFactory_EnsurePropagatesHostOverrideToAppServer(t *testing.T) {
	disp := &capturePlanDispatcher{
		forwardArgv: []string{filepath.Join(t.TempDir(), "missing-app-server")},
	}
	sockPath := filepath.Join(t.TempDir(), "codex.sock")
	f := NewFactory(FactoryConfig{
		Runtime:    &fakeRuntime{},
		Dispatcher: disp,
		// Project's default sandbox mode is devcontainer.
		IsContainer: func(_ string) bool { return true },
		// Capture useContainer via a stub — post-fix this must be false because
		// plan.Sandbox=Host must AND against the project's default sandbox mode
		// (which is true here).
		ResolveSockPath: func(_ state.SessionID, _ string, useContainer bool) (string, error) {
			if useContainer {
				t.Errorf("resolveSockPath received useContainer=true; want false when plan.Sandbox=SandboxOverrideHost")
			}
			return sockPath, nil
		},
	})

	plan := state.LaunchPlan{
		Command: "codex",
		Sandbox: state.SandboxOverrideHost, // user overrides sandboxed project to run on host
	}
	_, _, err := f.Ensure(context.Background(), "sess-host", "/proj/sandboxed", plan)
	// Start is expected to fail — the fake dispatcher forwards a non-existent
	// binary and agentlaunch.Spawn will error. The capture happens BEFORE
	// Spawn, so a failed Start does not invalidate the assertion.
	if err == nil {
		t.Fatal("expected b.Start to fail on missing app-server binary")
	}

	if !disp.capturedPlan.ForceHost {
		t.Errorf(
			"app-server plan.ForceHost = false; want true.\n"+
				"Root cause: Factory.Ensure sets b.isContainer from IsContainer(project) alone, "+
				"ignoring plan.Sandbox=SandboxOverrideHost. spawn.go then builds the app-server "+
				"plan without propagating the override, so SandboxDispatcher routes the app-server "+
				"to the devcontainer while the frame runs on host — the two cannot share the UDS "+
				"and the codex frame exits 1 within seconds.\n"+
				"captured plan: %+v", disp.capturedPlan)
	}
}

func TestFactory_FindFrameByThread_ScopesLookupToSession(t *testing.T) {
	f := NewFactory(FactoryConfig{})

	a := New(nil, nil, "stream:session:sess-a", "sess-a", "/workspace/a", "codex", nil, "", "", false, false, "/tmp/a.sock", 0)
	a.frames["frame-a"] = &frameBinding{frameID: "frame-a", threadID: "shared-thread"}
	a.threads["shared-thread"] = "frame-a"
	f.backends["stream:session:sess-a"] = a

	b := New(nil, nil, "stream:session:sess-b", "sess-b", "/workspace/b", "codex", nil, "", "", false, false, "/tmp/b.sock", 0)
	b.frames["frame-b"] = &frameBinding{frameID: "frame-b", threadID: "shared-thread"}
	b.threads["shared-thread"] = "frame-b"
	f.backends["stream:session:sess-b"] = b

	if got, ok := f.FindFrameByThread("sess-a", "shared-thread"); !ok || got != "frame-a" {
		t.Fatalf("sess-a/shared-thread = %q ok=%v, want frame-a/true", got, ok)
	}
	if got, ok := f.FindFrameByThread("sess-b", "shared-thread"); !ok || got != "frame-b" {
		t.Fatalf("sess-b/shared-thread = %q ok=%v, want frame-b/true", got, ok)
	}
}
