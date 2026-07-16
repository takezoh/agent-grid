// Package framelaunch owns the "run pre-conditions, then exec main" sequencing
// used by every frame launch (container-sandboxed via `bridge frame-exec` and
// host-direct via `<self-bin> frame-exec`). Sequencing lives here so both
// callers share exactly one Go implementation; only the sandbox harness (docker
// exec vs direct spawn) differs upstream. See adr-20260711-0082.
//
// Fallback note (R-4): if `env -0` is unavailable on a container's shell,
// a future path may emit env via `/proc/self/exe --emit-env` instead of
// shell `env -0`. The wire format (FrameSpec + AG_FRAME_SPEC) stays unchanged.
package framelaunch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/takezoh/agent-grid/platform/appid"
)

// FrameSpec is transported via AG_FRAME_SPEC env var (JSON).
// See adr-20260711-0084 for the transport rationale.
type FrameSpec struct {
	PreExec           string     `json:"pre_exec,omitempty"`            // devcontainer.json preExecCommand
	LoginShell        string     `json:"login_shell,omitempty"`         // optional override; empty → resolve from /etc/passwd
	PreCommands       [][]string `json:"pre_commands,omitempty"`        // argv列 (all must exit 0 before main)
	MainCommand       []string   `json:"main_command"`                  // argv, required
	PreCommandTimeout string     `json:"pre_command_timeout,omitempty"` // Go time.Duration string; default 10s
}

// DefaultTimeout is the per-preCommand / preExec deadline when the FrameSpec
// does not override it. OQ-2 resolution: 10s balances typical sub-second
// pre-commands with container cold-start filesystem I/O outliers.
const DefaultTimeout = 10 * time.Second

// EnvVar is the process environment key carrying the FrameSpec JSON payload.
const EnvVar = "AG_FRAME_SPEC"

// Package-var seams for T0 tests. Production paths use the syscall / os
// implementations; unit tests replace these with capturing fakes.
var (
	execReplacer = syscall.Exec
	readPasswd   = func() ([]byte, error) { return os.ReadFile("/etc/passwd") }
	currentUser  = user.Current
	// runtimePathListForTest is the T1 seam that lets hermetic tests override
	// the SSOT list without importing appid's real container paths. Production
	// path uses appid.RuntimeAuthoritativePathList(). See
	// adr-20260716-framelaunch-runtime-path-owner.
	runtimePathListForTest func() []string
)

// TogglePathReassertDisableEnv is the env var that, when set to a truthy value
// ("1" / "true" / "yes" — case-insensitive), causes Run() to skip the PATH
// re-assertion merge. Provided as a rollback escape hatch for the case-D
// shim-priority-hardening rollout (adr-20260716-shim-priority-hardening-and-migration).
const TogglePathReassertDisableEnv = "AG_FRAMELAUNCH_DISABLE_PATH_REASSERT"

func runtimePathList() []string {
	if runtimePathListForTest != nil {
		return runtimePathListForTest()
	}
	return appid.RuntimeAuthoritativePathList()
}

func pathReassertDisabled() bool {
	switch strings.ToLower(os.Getenv(TogglePathReassertDisableEnv)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// Encode marshals spec into the AG_FRAME_SPEC wire format (JSON string).
// This is the single source of truth for the on-wire encoding used by both
// the devcontainer path and DirectLauncher.
func Encode(spec FrameSpec) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("frame-exec: encode FrameSpec: %w", err)
	}
	return string(b), nil
}

// Run is the shared entry point for `<bin> frame-exec`. It reads
// AG_FRAME_SPEC, evaluates PreExec (env capture), runs PreCommands
// sequentially, then syscall.Execs MainCommand. Returns a non-nil error
// only on setup / gate failure; on success it does not return (the process
// is replaced by MainCommand).
func Run() error {
	spec, err := loadFrameSpec(os.Getenv(EnvVar))
	if err != nil {
		return err
	}
	timeout := DefaultTimeout
	if spec.PreCommandTimeout != "" {
		if d, perr := time.ParseDuration(spec.PreCommandTimeout); perr == nil {
			timeout = d
		}
	}
	loginShell := spec.LoginShell
	if loginShell == "" {
		loginShell = resolveLoginShell()
	}

	if spec.PreExec != "" {
		origPath := os.Getenv("PATH")
		env, err := capturePreExecEnv(loginShell, spec.PreExec, timeout)
		if err != nil {
			return fmt.Errorf("frame-exec: preExec eval: %w", err)
		}
		for k, v := range env {
			_ = os.Setenv(k, v)
		}
		// Re-assert runtime-authoritative shim dirs at PATH front. Case-D
		// invariant: preExec's shell (mise activate / dotfiles) commonly
		// reorders PATH so provider shim dirs are no longer first. framelaunch
		// is the sole owner of the "shim first" ordering and unconditionally
		// prepends appid.RuntimeAuthoritativePathList() here.
		// See adr-20260716-framelaunch-runtime-path-owner.
		reassertPath(origPath, env["PATH"])
	}
	for i, pre := range spec.PreCommands {
		if err := runPreCommand(pre, timeout); err != nil {
			return fmt.Errorf("frame-exec: preCommand[%d] %v: %w", i, pre, err)
		}
	}
	if len(spec.MainCommand) == 0 {
		return errors.New("frame-exec: MainCommand is empty")
	}
	bin, err := exec.LookPath(spec.MainCommand[0])
	if err != nil {
		return fmt.Errorf("frame-exec: lookup main: %w", err)
	}
	return execReplacer(bin, spec.MainCommand, os.Environ())
}

// resolveLoginShell reads the current process user's login shell from
// /etc/passwd, mirroring the existing envelope's `getent passwd | cut -d: -f7`
// behavior (so user zsh / bash / etc. dotfiles get sourced by preExec's shell).
// Falls back to /bin/sh if resolution fails.
func resolveLoginShell() string {
	u, err := currentUser()
	if err != nil {
		return "/bin/sh"
	}
	data, err := readPasswd()
	if err != nil {
		return "/bin/sh"
	}
	for _, line := range strings.Split(string(data), "\n") {
		// <user>:<passwd>:<uid>:<gid>:<gecos>:<home>:<shell>
		fields := strings.Split(line, ":")
		if len(fields) >= 7 && fields[0] == u.Username {
			if shell := strings.TrimSpace(fields[6]); shell != "" {
				return shell
			}
		}
	}
	return "/bin/sh"
}

func loadFrameSpec(raw string) (FrameSpec, error) {
	if raw == "" {
		return FrameSpec{}, errors.New("frame-exec: AG_FRAME_SPEC env is empty")
	}
	var s FrameSpec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return FrameSpec{}, fmt.Errorf("frame-exec: parse AG_FRAME_SPEC: %w", err)
	}
	return s, nil
}

// capturePreExecEnv runs `<loginShell> -lc '<preExec> && env -0'` and parses
// the NUL-delimited env dump. See FR-008.
func capturePreExecEnv(loginShell, preExec string, timeout time.Duration) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, loginShell, "-lc", preExec+" && env -0")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return parseEnv0(buf.Bytes()), nil
}

func parseEnv0(b []byte) map[string]string {
	out := map[string]string{}
	for _, kv := range bytes.Split(b, []byte{0}) {
		if len(kv) == 0 {
			continue
		}
		if i := bytes.IndexByte(kv, '='); i > 0 {
			out[string(kv[:i])] = string(kv[i+1:])
		}
	}
	return out
}

// reassertPath composes the final PATH from runtimePathList (SSOT) prepended
// to the base PATH, dedup'd, and writes it via os.Setenv. Emits a single
// framelaunch.path_reassert slog record per invocation. When the rollback
// toggle env is set truthy, skips the os.Setenv but still emits the slog
// with skip_branch="toggle_disabled" so operators can see the fix was gated
// off. See adr-20260716-shim-priority-hardening-and-migration.
func reassertPath(origPath, capturedPath string) {
	list := runtimePathList()
	finalPath, decision := computeFinalPath(list, capturedPath, origPath)
	if pathReassertDisabled() {
		decision.Branch = "toggle_disabled"
		slog.Info("framelaunch.path_reassert",
			"orig_path_len", len(origPath),
			"runtime_prefix_count", decision.PrefixCount,
			"dedup_dropped_count", decision.DroppedCount,
			"post_reassert_changed_head", false,
			"skip_branch", "toggle_disabled",
		)
		return
	}
	_ = os.Setenv("PATH", finalPath)
	slog.Info("framelaunch.path_reassert",
		"orig_path_len", len(origPath),
		"runtime_prefix_count", decision.PrefixCount,
		"dedup_dropped_count", decision.DroppedCount,
		"post_reassert_changed_head", decision.HeadChanged,
		"skip_branch", "",
	)
}

// runPreCommand executes pre with per-command timeout, forwarding stdio.
// SIGTERM on ctx cancel, SIGKILL 5s later. Non-zero exit or timeout → error.
func runPreCommand(pre []string, timeout time.Duration) error {
	if len(pre) == 0 {
		return errors.New("empty preCommand argv")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, pre[0], pre[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Run(); err != nil {
		slog.Warn("frame-exec: preCommand failed", "argv", pre, "err", err)
		return err
	}
	return nil
}
