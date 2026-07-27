// Package appid is the single source of truth for the application's on-disk
// and process identity: binary/command names, directory names, container
// paths, and runtime file names. Callers must reference these constants rather
// than hard-coding the literals, so renaming the app touches exactly one file.
//
// Two tokens span the identity:
//   - Name ("agent-grid") — the project name, used for directories
//     (~/.agent-grid, .agent-grid/, /opt/agent-grid, ~/.local/lib/agent-grid).
//   - ClientBin ("server") — the backend binary name, used for the runtime
//     files it owns (server.sock/.pid/.log).
//
// Note: the IPC/runtime env-var contract (AG_* names) and the persisted
// JSON key roost_session_id are intentionally NOT defined here — they are a
// separate wire/contract surface kept stable for backward compatibility.
package appid

const (
	// Name is the project/app name, used for on-disk directories.
	Name = "agent-grid"

	// ClientBin is the backend daemon binary and command name.
	ClientBin = "server"

	// BridgeBin is the in-container helper binary name (agent-grid bridge).
	BridgeBin = "bridge"

	// DotDir is the per-user and per-project dot directory name,
	// e.g. ~/.agent-grid and <project>/.agent-grid.
	DotDir = "." + Name

	// LibDirName is the libexec directory name under ~/.local/lib/.
	LibDirName = Name

	// Runtime files the daemon writes into its data directory.
	SocketFileName = ClientBin + ".sock" // server.sock
	PidFileName    = ClientBin + ".pid"  // server.pid
	LogFileName    = ClientBin + ".log"  // server.log

	// Container-side paths for files bind-mounted from the per-project run dir.
	// These are the canonical sources; callers must not hard-code these literals.
	ContainerRunDir           = "/opt/" + Name + "/run"
	ContainerBinaryPath       = ContainerRunDir + "/" + BridgeBin
	ContainerSockFileName     = SocketFileName
	ContainerSockFilePath     = ContainerRunDir + "/" + ContainerSockFileName
	ContainerHostExecSockPath = ContainerRunDir + "/hostexec.sock"
	ContainerMCPSockPath      = ContainerRunDir + "/mcp.sock"

	// Shim subdirectory names under ContainerRunDir. Runtime-authoritative:
	// framelaunch prepends the full paths to every PreExec-branched frame's
	// PATH so shim resolution is deterministic regardless of what preExec
	// (mise activate / dotfiles) does to PATH ordering.
	// See adr-20260716-framelaunch-runtime-path-owner /
	// adr-20260716-provider-shim-root-appid-ssot.
	HostExecShimsDir  = "hostexec-shims"
	SecretEnvShimsDir = "secretenv-shims"

	HostExecShimsPath  = ContainerRunDir + "/" + HostExecShimsDir
	SecretEnvShimsPath = ContainerRunDir + "/" + SecretEnvShimsDir
)

// RuntimeAuthoritativePathList returns the ordered list of container-side
// directories that framelaunch.Run() unconditionally prepends to PATH after
// preExec evaluation. The order is stable and part of the SSOT: hostexec-shims
// first, secretenv-shims second. A fresh slice is returned on every call so
// callers cannot mutate the underlying list.
//
// This function is the sole SSOT for what counts as "runtime authoritative"
// in the PATH ordering contract. Providers (hostexec, secretenv) MUST NOT
// contribute Env["PATH"]; framelaunch reads only this list.
func RuntimeAuthoritativePathList() []string {
	return []string{HostExecShimsPath, SecretEnvShimsPath}
}
