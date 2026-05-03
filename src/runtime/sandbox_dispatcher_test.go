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
		Options: state.LaunchOptions{Sandbox: state.SandboxOverrideHost},
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
