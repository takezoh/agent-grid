// roost-bridge is the thin client binary deployed inside devcontainers.
// It handles exactly the four container-side roles that need to reach the
// host roost daemon via the bind-mounted Unix socket:
//
//	event <type>      – agent hook (forwards CmdEvent / CmdHookEvent to daemon)
//	host-exec <bin>   – PATH shim target (proxies stdio to host via SCM_RIGHTS)
//	peers-mcp         – stdio MCP server for roost-peers
//	setup <agent>     – postCreate hook registration (claude / codex / gemini)
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/takezoh/agent-roost/event"
	"github.com/takezoh/agent-roost/hostexec"
	"github.com/takezoh/agent-roost/lib/claude"
	"github.com/takezoh/agent-roost/lib/codex"
	"github.com/takezoh/agent-roost/lib/gemini"
	"github.com/takezoh/agent-roost/lib/peers"
)

// hostExecSockPath is the Unix socket for the host-exec broker inside the container.
// This matches runtime.ContainerHostExecSockPath; duplicated here to avoid importing
// the full runtime package (which would pull in tmux, TUI, and other host-only deps).
const hostExecSockPath = "/opt/roost/run/hostexec.sock"

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
	case "host-exec":
		err = runHostExec(rest)
	case "peers-mcp":
		err = peers.Run(rest)
	case "setup":
		err = runSetup(rest)
	default:
		fmt.Fprintf(os.Stderr, "roost-bridge: unknown subcommand: %s\n", sub)
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "roost-bridge: %v\n", err)
		os.Exit(1)
	}
}

func runHostExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: host-exec <binary> [args...]")
	}

	conn, err := net.Dial("unix", hostExecSockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "host-exec: broker unavailable (%v)\n", err)
		os.Exit(127)
	}
	uc := conn.(*net.UnixConn)

	cwd, _ := os.Getwd()
	req := hostexec.Request{
		Binary: args[0],
		Args:   args[1:],
		Cwd:    cwd,
	}
	fds := [3]int{int(os.Stdin.Fd()), int(os.Stdout.Fd()), int(os.Stderr.Fd())}
	if err := hostexec.SendRequest(uc, req, fds); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "host-exec: %v\n", err)
		os.Exit(127)
	}

	var resp hostexec.Response
	if err := json.NewDecoder(uc).Decode(&resp); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "host-exec: read response: %v\n", err)
		os.Exit(127)
	}

	conn.Close()
	os.Exit(resp.ExitCode)
	return nil //nolint:govet // unreachable after os.Exit
}

func runSetup(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: setup <agent> (claude|codex|gemini)")
	}
	switch args[0] {
	case "claude":
		return claude.RunSetup()
	case "codex":
		return codex.RunSetup()
	case "gemini":
		return gemini.RunSetup()
	default:
		return fmt.Errorf("setup: unknown agent %q (want claude|codex|gemini)", args[0])
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: roost-bridge <subcommand> [args...]

Subcommands:
  event <type>      Send an event to the roost daemon
  host-exec <bin>   Execute a host binary via the hostexec broker
  peers-mcp         Start the roost-peers MCP server (stdio)
  setup <agent>     Register roost hooks for an agent (claude|codex|gemini)
`)
}
