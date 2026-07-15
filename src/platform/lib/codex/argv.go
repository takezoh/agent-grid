package codex

import (
	"fmt"
	"strings"
)

// Driver-level constants shared by all layers.
const (
	DriverName = "codex"
	SockPrefix = "codex-"
	SockSuffix = ".sock"
)

// CommandConfig is the parsed form of a codex launch command string.
type CommandConfig struct {
	ServerBin  string
	ServerArgs []string
	Model      string
	Effort     string
}

// ParseCommand parses a pre-tokenized codex argv into a CommandConfig.
// The caller is responsible for tokenizing the command string (e.g. via
// agentlaunch.SplitArgs) before passing it here.
func ParseCommand(argv []string) (CommandConfig, error) {
	if len(argv) == 0 || argv[0] != DriverName {
		return CommandConfig{}, fmt.Errorf("codex: unsupported command %q", strings.Join(argv, " "))
	}
	cfg := CommandConfig{ServerBin: DriverName}
	for i := 1; i < len(argv); i++ {
		arg := argv[i]
		switch arg {
		case "resume":
			i++ // skip resume target; actual locator comes from the launch plan
		case "-m", "--model":
			if i+1 < len(argv) {
				cfg.Model = argv[i+1]
				i++
			}
		case "--effort":
			if i+1 < len(argv) {
				cfg.Effort = argv[i+1]
				i++
			}
		case "-c", "--config", "--enable", "--disable":
			if i+1 < len(argv) {
				cfg.ServerArgs = append(cfg.ServerArgs, arg, argv[i+1])
				i++
			}
		default:
			switch {
			case strings.HasPrefix(arg, "--model="):
				cfg.Model = strings.TrimSpace(strings.TrimPrefix(arg, "--model="))
			case strings.HasPrefix(arg, "--effort="):
				cfg.Effort = strings.TrimSpace(strings.TrimPrefix(arg, "--effort="))
			}
		}
	}
	return cfg, nil
}

// AppServerListenArgs returns the argv for `codex app-server --listen unix://<sock>`.
// extra is passed through verbatim (e.g. ["-c", "key=val"] config overrides).
// When sandboxExternal is true, -c sandbox_mode="danger-full-access" is appended.
func AppServerListenArgs(serverBin, sock string, extra []string, sandboxExternal bool) []string {
	args := []string{serverBin, "app-server", "--listen", "unix://" + sock}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// AppServerStdioArgs returns the argv for `codex app-server` (stdio transport, no --listen).
func AppServerStdioArgs(extra []string, sandboxExternal bool) []string {
	args := []string{DriverName, "app-server"}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// RemoteAttachArgs returns the argv for the codex CLI frame that attaches to
// the per-session app-server over its unix domain socket. sock is the
// container-absolute UDS path the app-server binds; the codex CLI runs in
// the same sandbox, so it connects to that socket directly (no TCP routing
// bridge).
//
// threadID selects the thread the CLI attaches to:
//
//   - Empty (fresh cold-start): `codex --remote unix://<sock>` — the CLI
//     will issue its own `thread/start` on its connection, and the stream
//     backend adopts the resulting thread into the (single) pending frame
//     via handleThreadStarted (see ADR-0081).
//   - Non-empty (cold-start recovery): `codex resume <id> --remote
//     unix://<sock>` — the CLI reads `~/.codex/sessions/…/rollout-<id>.jsonl`
//     locally and issues `thread/resume`, so app-server events for <id>
//     route back to the frame that was pre-bound with that id.
func RemoteAttachArgs(sock, threadID, startDir, model, effort string) []string {
	args := []string{DriverName}
	if threadID != "" {
		args = append(args, "resume", threadID)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	args = append(args, "--remote", "unix://"+sock, "--dangerously-bypass-approvals-and-sandbox")
	if startDir != "" {
		args = append(args, "-C", startDir)
	}
	return args
}

// ShellJoinArgv single-quote-escapes each element and joins with spaces,
// producing a string safe for embedding inside a shell command (e.g. docker exec bash -lc '...').
func ShellJoinArgv(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(parts, " ")
}
