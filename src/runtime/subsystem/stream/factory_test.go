package stream

import (
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestFactoryMakeIDDistinguishesSandboxMode(t *testing.T) {
	// Auto (= containerized) and Host overrides must produce different IDs so
	// the runtime can keep one app-server per environment. Container-mode IDs
	// are keyed by container; host-mode IDs by project.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	autoID := f.makeID("/repo", state.SandboxOverrideAuto)
	hostID := f.makeID("/repo", state.SandboxOverrideHost)
	if autoID == hostID {
		t.Fatalf("auto and host IDs collided: %q", autoID)
	}
	if want := state.SubsystemID("stream:container:/repo"); autoID != want {
		t.Errorf("autoID = %q, want %q", autoID, want)
	}
	if want := state.SubsystemID("stream:host:/repo"); hostID != want {
		t.Errorf("hostID = %q, want %q", hostID, want)
	}
}

func TestFactoryMakeIDEscapesColons(t *testing.T) {
	// ":" inside project paths would corrupt the "stream:<kind>:<key>" wire
	// format; replace with "_" so the ID stays parseable.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	id := f.makeID("/repo:weird", state.SandboxOverrideAuto)
	if want := state.SubsystemID("stream:container:/repo_weird"); id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

// Regression for "codex frame dies with 'failed to connect to remote app
// server' in shared mode" — the host-side codex app-server / sockbridge pair
// can only support one backend per container, so subsystem IDs from frames
// inside the same shared container must collapse onto a single ID. Currently
// the Factory keys IDs by project path, so two projects in one shared
// container create two backends that race for the same host socket; the loser
// dies on bind and its frame can't reach the bridge.
//
// This test pins the desired key: stream:container:<RunDirKey(project)>.
// RunDirKey returns "__shared__" for shared mode and the project path for
// per-project devcontainers, matching DevcontainerLauncher.RunDirKey.
func TestFactoryMakeID_SharedContainerCollapsesProjects(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(string) string { return "__shared__" },
	}}
	idA := f.makeID("/workspace/agent-roost", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/fintech", state.SandboxOverrideAuto)
	if idA != idB {
		t.Fatalf("shared container: IDs must collapse to one; got %q vs %q", idA, idB)
	}
	if want := state.SubsystemID("stream:container:__shared__"); idA != want {
		t.Errorf("shared container ID = %q, want %q", idA, want)
	}
}

func TestFactoryMakeID_ProjectContainerKeyedByContainer(t *testing.T) {
	// project-isolation devcontainers: each container has its own
	// RunDirKey == projectPath, so IDs stay separate. This is the legacy
	// per-project behavior, just routed through the container key explicitly.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	idA := f.makeID("/workspace/a", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/b", state.SandboxOverrideAuto)
	if idA == idB {
		t.Fatalf("project-isolation containers must stay separate; got identical %q", idA)
	}
	if want := state.SubsystemID("stream:container:/workspace/a"); idA != want {
		t.Errorf("project container ID = %q, want %q", idA, want)
	}
}

func TestFactoryMakeID_HostStaysPerProject(t *testing.T) {
	// Host-mode launches still need a per-project key — each host project
	// runs its own codex app-server in its own cwd, so collapsing them
	// would mix unrelated threads.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return false },
	}}
	idA := f.makeID("/workspace/a", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/b", state.SandboxOverrideAuto)
	if idA == idB {
		t.Errorf("host mode: per-project IDs expected; got identical %q", idA)
	}
	if want := state.SubsystemID("stream:host:/workspace/a"); idA != want {
		t.Errorf("host ID = %q, want %q", idA, want)
	}
}

func TestFactoryMakeID_HostOverrideAlwaysHostKind(t *testing.T) {
	// SandboxOverrideHost is the per-frame "use host" escape hatch even when
	// the project would otherwise run in a container. It must short-circuit
	// to host kind regardless of IsContainer.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(string) string { return "__shared__" },
	}}
	id := f.makeID("/workspace/p", state.SandboxOverrideHost)
	if want := state.SubsystemID("stream:host:/workspace/p"); id != want {
		t.Errorf("host override: id = %q, want %q", id, want)
	}
}
