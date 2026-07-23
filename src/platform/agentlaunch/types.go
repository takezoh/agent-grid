package agentlaunch

import (
	"context"
	"time"
)

// LaunchPlan carries the pure launch parameters. ForceHost replaces the
// host/state SandboxOverride == SandboxOverrideHost sentinel.
//
// Argv, when non-nil, holds the structured argv for Spawn (no host-side shell).
// Command is the shell-joined string form used by backend frame launchers.
// Both are populated by per-agent lib builders; callers choose which to use.
// PreCommands / PreExec are consumed by the in-process frame-exec launcher
// (see platform/framelaunch and adr-20260711-0082).
type LaunchPlan struct {
	Command               string
	Argv                  []string
	PreCommands           [][]string
	PreExec               string // devcontainer.json preExecCommand; forwarded to bridge frame-exec
	LoginShell            string // absolute path; optional override (empty → framelaunch resolves)
	PreCommandTimeout     time.Duration
	Env                   map[string]string
	StartDir              string
	Project               string
	ForceHost             bool
	ManagedFrameMessaging bool
}

// Mount is a host↔container path pair used to translate paths at the IPC boundary.
type Mount struct {
	Host, Container string
}

// WrappedLaunch is the resolved launch specification after sandboxing has been
// applied. Command/StartDir/Env are handed to the caller's spawn layer (a frame
// backend for the client, a direct stdio exec for the orchestrator);
// Cleanup is called when the launch is torn down.
//
// Argv, when non-nil, is the argv for Spawn (no host-side shell). Command is the
// shell-joined equivalent for backend frame launchers. Dispatchers populate both.
type WrappedLaunch struct {
	Command          string
	Argv             []string
	StartDir         string
	Env              map[string]string
	Cleanup          func(context.Context) error
	ContainerSockDir string
	Mounts           []Mount
}
