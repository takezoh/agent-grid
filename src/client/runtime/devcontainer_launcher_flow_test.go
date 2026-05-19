package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/client/config"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-roost/platform/sandbox/devcontainer"
)

// mockMgr is a sandbox.Manager[*sandboxdc.ContainerState] that lets tests
// drive WrapLaunch / AdoptFrame / EnsureProject without a real docker daemon.
type mockMgr struct {
	inst              *sandbox.Instance[*sandboxdc.ContainerState]
	ensureErr         error
	ensureCalls       int
	buildErr          error
	buildPlan         state.LaunchPlan
	buildFrameCtx     sandbox.FrameContext
	acquireCalls      int
	releaseCalls      int
	releaseReturnZero bool
	destroyCalls      int
}

func (m *mockMgr) EnsureInstance(_ context.Context, projectPath, _ string, opts sandbox.StartOptions) (*sandbox.Instance[*sandboxdc.ContainerState], error) {
	m.ensureCalls++
	if m.ensureErr != nil {
		return nil, m.ensureErr
	}
	if m.inst == nil {
		m.inst = &sandbox.Instance[*sandboxdc.ContainerState]{
			ProjectPath: projectPath,
			Internal:    nil, // not used by mock paths we exercise
		}
	}
	return m.inst, nil
}

func (m *mockMgr) BuildLaunchCommand(_ *sandbox.Instance[*sandboxdc.ContainerState], plan state.LaunchPlan, frameCtx sandbox.FrameContext, _ map[string]string) (string, map[string]string, error) {
	m.buildPlan = plan
	m.buildFrameCtx = frameCtx
	if m.buildErr != nil {
		return "", nil, m.buildErr
	}
	return "docker exec ... " + frameCtx.WorkDir, map[string]string{"FOO": "bar"}, nil
}

func (m *mockMgr) AcquireFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) { m.acquireCalls++ }

func (m *mockMgr) ReleaseFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) bool {
	m.releaseCalls++
	return m.releaseReturnZero
}

func (m *mockMgr) DestroyInstance(_ context.Context, _ *sandbox.Instance[*sandboxdc.ContainerState]) error {
	m.destroyCalls++
	return nil
}

func newLauncherForTest(t *testing.T, mgr *mockMgr, isolation string) *DevcontainerLauncher {
	t.Helper()
	return &DevcontainerLauncher{
		mgr: mgr,
		resolveSandbox: func(string) config.SandboxConfig {
			return config.SandboxConfig{Isolation: isolation}
		},
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
		dataDir:             t.TempDir(),
	}
}

func TestWrapLaunch_RejectsEmptyProject(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "")
	_, err := l.WrapLaunch("frame-1", state.LaunchPlan{Project: ""}, nil)
	if err == nil {
		t.Errorf("expected error for empty project")
	}
	if mgr.ensureCalls != 0 {
		t.Errorf("EnsureInstance must not be called when project is empty")
	}
}

func TestWrapLaunch_PropagatesEnsureError(t *testing.T) {
	mgr := &mockMgr{ensureErr: errors.New("docker down")}
	l := newLauncherForTest(t, mgr, "")
	_, err := l.WrapLaunch("frame-1", state.LaunchPlan{Project: "/workspace/myapp"}, nil)
	if err == nil || !strings.Contains(err.Error(), "ensure instance") {
		t.Errorf("expected ensure-instance error wrap, got: %v", err)
	}
	if mgr.acquireCalls != 0 {
		t.Errorf("AcquireFrame must not run after EnsureInstance fails; got %d calls", mgr.acquireCalls)
	}
}

func TestWrapLaunch_HappyPath_AcquireFrameAndCleanup(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "")
	plan := state.LaunchPlan{Project: "/workspace/myapp", StartDir: "/workspace/myapp"}
	wl, err := l.WrapLaunch("frame-1", plan, map[string]string{"E": "1"})
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if mgr.ensureCalls != 1 {
		t.Errorf("ensureCalls = %d, want 1", mgr.ensureCalls)
	}
	if mgr.acquireCalls != 1 {
		t.Errorf("acquireCalls = %d, want 1", mgr.acquireCalls)
	}
	if wl.Cleanup == nil {
		t.Fatalf("Cleanup callback was not registered")
	}

	// Cleanup invokes ReleaseFrame; when refCount=0 it must also Destroy.
	mgr.releaseReturnZero = true
	if err := wl.Cleanup(); err != nil {
		t.Errorf("Cleanup: %v", err)
	}
	if mgr.releaseCalls != 1 {
		t.Errorf("releaseCalls = %d, want 1", mgr.releaseCalls)
	}
	if mgr.destroyCalls != 1 {
		t.Errorf("destroyCalls = %d, want 1 (refCount==0 destroys)", mgr.destroyCalls)
	}
}

func TestWrapLaunch_CleanupSkipsDestroyWhenRefCountPositive(t *testing.T) {
	mgr := &mockMgr{releaseReturnZero: false}
	l := newLauncherForTest(t, mgr, "")
	wl, err := l.WrapLaunch("frame-1", state.LaunchPlan{Project: "/p"}, nil)
	if err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if err := wl.Cleanup(); err != nil {
		t.Errorf("Cleanup: %v", err)
	}
	if mgr.destroyCalls != 0 {
		t.Errorf("DestroyInstance must NOT be called when refCount>0; got %d", mgr.destroyCalls)
	}
}

func TestWrapLaunch_PassesFrameIDAndWorkDirThroughCtx(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "")
	plan := state.LaunchPlan{Project: "/p", StartDir: "/p/sub"}
	if _, err := l.WrapLaunch("frame-abc", plan, nil); err != nil {
		t.Fatalf("WrapLaunch: %v", err)
	}
	if mgr.buildFrameCtx.FrameID != "frame-abc" {
		t.Errorf("frameCtx.FrameID = %q, want frame-abc", mgr.buildFrameCtx.FrameID)
	}
	// WorkDir is populated from plan.StartDir when pathmap has no entry (mock instance has no mounts).
	if mgr.buildFrameCtx.WorkDir != "/p/sub" {
		t.Errorf("frameCtx.WorkDir = %q, want /p/sub", mgr.buildFrameCtx.WorkDir)
	}
}

func TestEnsureProject_PassesStartOptions(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "shared")
	if err := l.EnsureProject(context.Background(), "/workspace/myapp"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if mgr.ensureCalls != 1 {
		t.Errorf("ensureCalls = %d, want 1", mgr.ensureCalls)
	}
}

func TestEnsureProject_PropagatesError(t *testing.T) {
	mgr := &mockMgr{ensureErr: errors.New("docker stopped")}
	l := newLauncherForTest(t, mgr, "")
	if err := l.EnsureProject(context.Background(), "/p"); err == nil || !strings.Contains(err.Error(), "ensure project") {
		t.Errorf("expected ensure-project error, got: %v", err)
	}
}

func TestIsContainer_AlwaysTrue(t *testing.T) {
	l := newLauncherForTest(t, &mockMgr{}, "")
	if !l.IsContainer("/p") {
		t.Errorf("DevcontainerLauncher.IsContainer must always be true")
	}
}

func TestAdoptFrame_NoOpForEmptyProject(t *testing.T) {
	mgr := &mockMgr{}
	l := newLauncherForTest(t, mgr, "")
	cleanup, mounts, err := l.AdoptFrame(context.Background(), "frame-1", "")
	if err != nil || cleanup != nil || mounts != nil {
		t.Errorf("empty project: got cleanup-set=%v mounts=%v err=%v; all should be nil",
			cleanup != nil, mounts, err)
	}
	if mgr.ensureCalls != 0 {
		t.Errorf("EnsureInstance must not be called for empty project")
	}
}

func TestAdoptFrame_PropagatesEnsureError(t *testing.T) {
	mgr := &mockMgr{ensureErr: errors.New("docker down")}
	l := newLauncherForTest(t, mgr, "")
	_, _, err := l.AdoptFrame(context.Background(), "frame-1", "/p")
	if err == nil || !strings.Contains(err.Error(), "adopt frame") {
		t.Errorf("expected adopt-frame error, got: %v", err)
	}
}

// Ensure that the sandbox.Manager interface contract is honored — if the
// signature ever shifts (e.g. a new method added) this won't compile.
var _ sandbox.Manager[*sandboxdc.ContainerState] = (*mockMgr)(nil)

// Suppress unused-import flags for sandboxdc.SharedContainerKey if needed.
var _ = sandboxdc.SharedContainerKey
