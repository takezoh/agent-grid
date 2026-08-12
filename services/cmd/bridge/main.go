// bridge is the thin client binary deployed inside devcontainers.
// It handles the container-side roles that need to reach the host client daemon:
//
//	event <type>          – agent hook (forwards CmdEvent / CmdHookEvent to daemon)
//	agent-frames-mcp      – managed frame-messaging MCP server over AG_SOCKET_TOKEN
//	codex-app-server-shim – injects managed dynamicTools in front of codex app-server
//	frame-exec            – preExec + pre-command + main sequencing (AG_FRAME_SPEC)
//	claude-setup-hooks    – register reactor as Claude Code's hook handler in
//	                        the container's ~/.claude/settings.json
//	gemini-setup-hooks    – register reactor as Gemini CLI's hook handler in
//	                        the container's ~/.gemini/settings.json
//
// Both *-setup-hooks subcommands are invoked from the devcontainer postCreate
// so every agent process inside the container starts with hooks already in
// place. The scripts/setup-{claude,gemini}.sh shims were deleted in lockstep
// — this binary owns the registration via host/lib/agenthook.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-grid/services/host/event"
	"github.com/takezoh/agent-grid/services/host/lib/agenthook"
	"github.com/takezoh/agent-grid/services/platform/appid"
	"github.com/takezoh/agent-grid/services/platform/framelaunch"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	sub := os.Args[1]
	rest := os.Args[2:]

	var err error
	switch sub {
	case "event":
		err = event.Run(rest)
	case "agent-frames-mcp":
		err = runAgentFramesMCP(rest)
	case "codex-app-server-shim":
		err = runCodexAppServerShim(rest)
	case "codex-trust-project":
		err = runCodexTrustProject(rest)
	case "frame-exec":
		if err := framelaunch.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "sockbridge":
		err = runSockBridge(rest)
	default:
		if spec, ok := lookupSetupHooksSpec(sub); ok {
			err = runAgentSetupHooks(rest, spec)
			break
		}
		if sub == agenthook.AntigravitySetupSubcommand {
			err = runAntigravitySetupHooks(rest)
			break
		}
		fmt.Fprintf(os.Stderr, "%s: unknown subcommand: %s\n", appid.BridgeBin, sub)
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appid.BridgeBin, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: bridge <subcommand> [args...]

Subcommands:
  event <type>               Send an event to the client daemon
  codex-app-server-shim      Inject managed frame-messaging dynamicTools for Codex
  frame-exec                 Run preExec/pre-commands then exec main (AG_FRAME_SPEC)
  sockbridge                 TCP↔unix socket bridge (fixed-socket)
  claude-setup-hooks         Register reactor as Claude Code's hook handler
                             in this environment's ~/.claude/settings.json
  gemini-setup-hooks         Register reactor as Gemini CLI's hook handler
                             in this environment's ~/.gemini/settings.json
`)
}

// lookupSetupHooksSpec returns the agenthook.Spec whose SubcmdName matches
// sub. Returns ok=false when sub is not a *-setup-hooks subcommand; the
// caller falls through to the unknown-subcommand error path. Reading the
// dispatch table from agenthook.All means adding a Spec there registers
// its container-side subcommand here for free.
func lookupSetupHooksSpec(sub string) (agenthook.Spec, bool) {
	for _, spec := range agenthook.All {
		if spec.SubcmdName == sub {
			return spec, true
		}
	}
	return agenthook.Spec{}, false
}

// runAgentSetupHooks registers the in-container bridge as the given
// agent's hook handler in the container's settings.json. Wired into the
// devcontainer postCreate by cmd/server/coordinator.go so every agent
// process spawned inside the container immediately hits a settings.json
// with every lifecycle event routed back to the daemon. Without it, the
// session card title stays "New Session" forever and the status badge
// never moves — because the daemon never learns the agent's session id /
// transcript path.
//
// Flags:
//
//	-settings PATH   Override the settings.json target. Default is
//	                 $HOME/<spec.SettingsRel>.
//	-data-dir DIR    Append `-data-dir DIR` to the hook command so a daemon
//	                 running with a non-default AG_DATA_DIR is reachable.
//	                 Inside the container AG_DATA_DIR resolves to the
//	                 bind-mounted ContainerRunDir already, so this is
//	                 usually omitted.
//
// The hook command is always built against ContainerBinaryPath
// (/opt/agent-grid/run/bridge) — the canonical bridge
// install path inside every devcontainer.
//
// HOME is read via os.UserHomeDir (passwd-aware) rather than the HOME env
// var so a container whose env strips HOME — but whose /etc/passwd carries
// a uid entry — still writes the settings file under the right user dir.
func runAgentSetupHooks(args []string, spec agenthook.Spec) error {
	fs := flag.NewFlagSet(spec.SubcmdName, flag.ContinueOnError)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("%s: HOME unresolved: %w", spec.SubcmdName, err)
	}
	defaultSettings := filepath.Join(home, spec.SettingsRel)
	settingsPath := fs.String("settings", defaultSettings, "settings.json target")
	dataDir := fs.String("data-dir", "", "append -data-dir DIR to hook command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	hookCmd := agenthook.BuildHookCmd(appid.ContainerBinaryPath, *dataDir, spec)
	registered, err := agenthook.Install(*settingsPath, hookCmd, spec)
	if err != nil {
		return fmt.Errorf("%s: %w", spec.SubcmdName, err)
	}
	if len(registered) == 0 {
		fmt.Fprintf(os.Stderr, "%s: hooks already registered (%s)\n", spec.SubcmdName, *settingsPath)
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s: registered %d events in %s\n", spec.SubcmdName, len(registered), *settingsPath)
	return nil
}

func runAntigravitySetupHooks(args []string) error {
	fs := flag.NewFlagSet(agenthook.AntigravitySetupSubcommand, flag.ContinueOnError)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("%s: HOME unresolved: %w", agenthook.AntigravitySetupSubcommand, err)
	}
	homeDir := fs.String("home", home, "home directory containing Antigravity plugin roots")
	dataDir := fs.String("data-dir", "", "append -data-dir DIR to hook command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	changed, err := agenthook.InstallAntigravityPlugins(*homeDir, appid.ContainerBinaryPath, *dataDir)
	if err != nil {
		return fmt.Errorf("%s: %w", agenthook.AntigravitySetupSubcommand, err)
	}
	fmt.Fprintf(os.Stderr, "%s: registered %d plugins\n",
		agenthook.AntigravitySetupSubcommand, len(changed))
	return nil
}
