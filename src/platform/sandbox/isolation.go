package sandbox

// IsolationKind discriminates how a launch's sandbox instance is shared across
// projects.
type IsolationKind int

const (
	// IsolationProject gives each project its own sandbox instance (default).
	IsolationProject IsolationKind = iota
	// IsolationShared routes every project through a single shared instance.
	IsolationShared
)

// SharedInstanceKey is the reserved containers-map / run-dir key for the shared
// instance. It is intentionally NOT an absolute path: config.SandboxResolver
// maps non-absolute keys to the user scope, which is exactly what a shared
// instance needs (it has no single project scope to merge). The literal value
// is part of the on-disk contract — existing containers and run directories are
// keyed by it — so it must not change.
const SharedInstanceKey = "__shared__"

// IsolationPlan is the single source of truth for how one launch is isolated.
// It is computed once (purely, see agentlaunch.DecideIsolation) at the decision
// point and threaded through the manager, the overlay builder, and teardown, so
// every key/name/path derivation reads the same value instead of independently
// re-deriving "am I shared?". Drift between such independent derivations is what
// repeatedly broke shared containers in the past (proxy sockets landing under a
// different hash than the run-dir bind); routing them all through one value
// makes that class of bug structurally impossible.
//
// Key derivation takes projectPath as an argument rather than storing it: the
// manager already carries one canonical projectPath (the EnsureInstance
// argument), and keeping the plan free of it avoids a second, divergeable copy.
type IsolationPlan struct {
	// Kind selects shared vs per-project isolation.
	Kind IsolationKind
	// DevcontainerDir overrides devcontainer.json discovery; "" = auto-discover.
	// Already ~-expanded by the decision shell.
	DevcontainerDir string
}

// IsShared reports whether this plan routes through the shared instance.
func (p IsolationPlan) IsShared() bool { return p.Kind == IsolationShared }

// IsSharedKey reports whether key is the shared instance's reserved key. Callers
// that only hold a resolved key (e.g. credproxy provider hooks) use this instead
// of comparing against SharedInstanceKey by hand, so "what a shared key is" lives
// in one place next to the constant.
func IsSharedKey(key string) bool { return key == SharedInstanceKey }

// ContainerKey is the instance identity: it is BOTH the containers-map key and
// the run-dir key. Being one method means the two cannot drift. Shared collapses
// to SharedInstanceKey; per-project keys by the project path.
func (p IsolationPlan) ContainerKey(projectPath string) string {
	if p.Kind == IsolationShared {
		return SharedInstanceKey
	}
	return projectPath
}

// OverlayProject is the project key used to resolve the proxy ContainerSpec and
// its per-project socket directory. INVARIANT: it MUST equal ContainerKey, or
// the proxy sockets land under a different hash than the run-dir bind and never
// appear inside the container. Defining it as ContainerKey enforces the
// invariant by construction.
func (p IsolationPlan) OverlayProject(projectPath string) string {
	return p.ContainerKey(projectPath)
}

// WorkspaceFallbackProject is the project whose path seeds the container's
// default workspace folder. A shared instance has no single project (it binds
// every project via extra workspace mounts and sets the cwd per frame), so the
// fallback is erased.
func (p IsolationPlan) WorkspaceFallbackProject(projectPath string) string {
	if p.Kind == IsolationShared {
		return ""
	}
	return projectPath
}

// FrameWorkspaceMount returns the (host, container) workspace mount a frame
// registers with pathmap. Shared erases it because a shared container has no
// single project root: the per-frame cwd is resolved against the instance's
// extra workspace mounts instead.
func (p IsolationPlan) FrameWorkspaceMount(projectPath, containerWS string) (host, container string) {
	if p.Kind == IsolationShared {
		return "", ""
	}
	return projectPath, containerWS
}
