package runtime

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
)

// fakeAgentLauncher records calls for assertion in tests.
type fakeAgentLauncher struct {
	wrapLaunchCalled bool
	adoptFrameCalled bool
	wrapErr          error
	adoptErr         error
	wrapResult       WrappedLaunch
}

func (f *fakeAgentLauncher) WrapLaunch(_ state.FrameID, _ state.LaunchPlan, _ map[string]string) (WrappedLaunch, error) {
	f.wrapLaunchCalled = true
	return f.wrapResult, f.wrapErr
}

func (f *fakeAgentLauncher) AdoptFrame(_ context.Context, _ state.FrameID, _ string) (func() error, pathmap.Mounts, error) {
	f.adoptFrameCalled = true
	return nil, nil, f.adoptErr
}

func (f *fakeAgentLauncher) EnsureProject(_ context.Context, _ string) error { return nil }

func (f *fakeAgentLauncher) IsContainer(_ string) bool { return false }

func TestSandboxDispatcher_DirectMode_RoutesToDirect(t *testing.T) {
	direct := &fakeAgentLauncher{wrapResult: WrappedLaunch{Command: "bash"}}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: direct}

	plan := state.LaunchPlan{Project: "/workspace/foo", Command: "bash"}
	got, err := d.WrapLaunch("f1", plan, nil)
	if err != nil {
		t.Fatalf("WrapLaunch error: %v", err)
	}
	if !direct.wrapLaunchCalled {
		t.Error("expected direct WrapLaunch to be called")
	}
	if got.Command != "bash" {
		t.Errorf("Command = %q, want bash", got.Command)
	}
}

func TestSandboxDispatcher_EmptyMode_RoutesToDirect(t *testing.T) {
	direct := &fakeAgentLauncher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{}) // mode = ""
	d := &SandboxDispatcher{Resolver: resolver, Direct: direct}

	_, err := d.WrapLaunch("f1", state.LaunchPlan{Project: "/workspace/foo"}, nil)
	if err != nil {
		t.Fatalf("WrapLaunch error: %v", err)
	}
	if !direct.wrapLaunchCalled {
		t.Error("expected direct.WrapLaunch called for empty mode")
	}
}

func TestSandboxDispatcher_DevcontainerMode_NilDevcontainer_ReturnsError(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: DirectLauncher{}, Devcontainer: nil}

	_, err := d.WrapLaunch("f1", state.LaunchPlan{Project: "/workspace/foo"}, nil)
	if err == nil {
		t.Error("expected error when devcontainer backend is nil but mode=devcontainer")
	}
}

func TestSandboxDispatcher_UnknownMode_ReturnsError(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "firecracker"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: DirectLauncher{}}

	_, err := d.WrapLaunch("f1", state.LaunchPlan{Project: "/workspace/foo"}, nil)
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// TestSandboxDispatcher_HostOverride_RoutesToDirect verifies that SandboxOverrideHost
// bypasses the project-level sandbox config and goes directly to Direct.
// Devcontainer is left nil; without the override that would return an error —
// the fact that Direct succeeds confirms the early-return path was taken.
func TestSandboxDispatcher_HostOverride_RoutesToDirect(t *testing.T) {
	direct := &fakeAgentLauncher{wrapResult: WrappedLaunch{Command: "claude"}}
	// Mode=devcontainer with nil Devcontainer normally errors, but HostOverride must bypass.
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &SandboxDispatcher{
		Resolver:     resolver,
		Direct:       direct,
		Devcontainer: nil,
	}

	plan := state.LaunchPlan{
		Project: "/workspace/foo",
		Command: "claude",
		Sandbox: state.SandboxOverrideHost,
	}
	got, err := d.WrapLaunch("f1", plan, nil)
	if err != nil {
		t.Fatalf("WrapLaunch error: %v", err)
	}
	if !direct.wrapLaunchCalled {
		t.Error("expected Direct.WrapLaunch to be called")
	}
	if got.Command != "claude" {
		t.Errorf("Command = %q, want claude", got.Command)
	}
}

func TestSandboxDispatcher_AdoptFrame_DirectMode(t *testing.T) {
	direct := &fakeAgentLauncher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: direct}

	_, _, err := d.AdoptFrame(context.Background(), "f1", "/workspace/foo")
	if err != nil {
		t.Fatalf("AdoptFrame error: %v", err)
	}
	if !direct.adoptFrameCalled {
		t.Error("expected direct.AdoptFrame called")
	}
}

// fakeEnsureLauncher tracks EnsureProject calls.
type fakeEnsureLauncher struct {
	fakeAgentLauncher
	ensureCalled bool
	ensureErr    error
}

func (f *fakeEnsureLauncher) EnsureProject(_ context.Context, _ string) error {
	f.ensureCalled = true
	return f.ensureErr
}

func TestSandboxDispatcher_EnsureProject_DirectMode(t *testing.T) {
	direct := &fakeEnsureLauncher{}
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "direct"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: direct}

	if err := d.EnsureProject(context.Background(), "/p"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if !direct.ensureCalled {
		t.Errorf("direct.EnsureProject not called")
	}
}

func TestSandboxDispatcher_EnsureProject_DevcontainerWithoutBackend_NoOp(t *testing.T) {
	// devcontainer mode but no Devcontainer backend wired → return nil (no error)
	// instead of crashing. This is the path coordinator.go takes when Docker is
	// unavailable but the user still asked for devcontainer mode.
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: &fakeEnsureLauncher{}}

	if err := d.EnsureProject(context.Background(), "/p"); err != nil {
		t.Errorf("EnsureProject without backend: %v, want nil", err)
	}
}

func TestSandboxDispatcher_EnsureProject_UnknownMode(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "vagrant"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: &fakeEnsureLauncher{}}
	err := d.EnsureProject(context.Background(), "/p")
	if err == nil {
		t.Errorf("unknown mode must error")
	}
}

func TestSandboxDispatcher_IsContainer_NoBackend(t *testing.T) {
	d := &SandboxDispatcher{
		Resolver: config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"}),
		// Devcontainer left nil
	}
	if d.IsContainer("/p") {
		t.Errorf("IsContainer must return false when no devcontainer backend")
	}
}

func TestSandboxDispatcher_IsContainer_DevcontainerMode(t *testing.T) {
	d := &SandboxDispatcher{
		Resolver:     config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"}),
		Devcontainer: &DevcontainerLauncher{},
	}
	if !d.IsContainer("/p") {
		t.Errorf("IsContainer must be true for devcontainer mode + backend")
	}
}

func TestSandboxDispatcher_AdoptFrame_DevcontainerWithoutBackend(t *testing.T) {
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	d := &SandboxDispatcher{Resolver: resolver, Direct: &fakeAgentLauncher{}}
	cleanup, mounts, err := d.AdoptFrame(context.Background(), "f1", "/p")
	if err != nil || cleanup != nil || mounts != nil {
		t.Errorf("nil-backend devcontainer adopt: got cleanup-set=%v mounts=%v err=%v",
			cleanup != nil, mounts, err)
	}
}

func TestSandboxDispatcher_AdoptFrame_UnknownMode(t *testing.T) {
	d := &SandboxDispatcher{
		Resolver: config.NewSandboxResolver(config.SandboxConfig{Mode: "vagrant"}),
		Direct:   &fakeAgentLauncher{},
	}
	_, _, err := d.AdoptFrame(context.Background(), "f1", "/p")
	if err == nil {
		t.Errorf("unknown mode must error")
	}
}

func TestDevcontainerLauncherFor(t *testing.T) {
	dl := &DevcontainerLauncher{}
	if got := devcontainerLauncherFor(dl); got != dl {
		t.Errorf("bare DevcontainerLauncher: got %v, want %v", got, dl)
	}
	disp := &SandboxDispatcher{Devcontainer: dl}
	if got := devcontainerLauncherFor(disp); got != dl {
		t.Errorf("wrapped: got %v, want %v", got, dl)
	}
	if got := devcontainerLauncherFor(&fakeAgentLauncher{}); got != nil {
		t.Errorf("unknown launcher type: got %v, want nil", got)
	}
}
