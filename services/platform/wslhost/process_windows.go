//go:build windows

package wslhost

import (
	"context"
	"errors"
	"os/exec"
	"sync"
)

// ExecWSLRunner is the production WSLRunner using wsl.exe on Windows.
type ExecWSLRunner struct {
	// WSLExe overrides the wsl.exe path (tests).
	WSLExe string
}

func (r *ExecWSLRunner) wsl() string {
	if r.WSLExe != "" {
		return r.WSLExe
	}
	return "wsl.exe"
}

// StartForeground implements WSLRunner via direct wsl.exe --exec (no shell).
func (r *ExecWSLRunner) StartForeground(ctx context.Context, spec LaunchSpec) (SupervisedUnit, error) {
	argv := BuildWSLExecArgv(spec)
	if err := ValidateLaunchArgv(argv, spec.Distribution, spec.LinuxUser); err != nil {
		return nil, err
	}
	// Replace program name with configured binary; argv[0] is logical "wsl.exe".
	cmd := exec.CommandContext(ctx, r.wsl(), argv[1:]...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execUnit{cmd: cmd, argv: argv}, nil
}

// TerminateDistro implements WSLRunner for the registration-bound distribution only.
func (r *ExecWSLRunner) TerminateDistro(ctx context.Context, distribution string) error {
	if distribution == "" {
		return errors.New("wslhost: empty distribution")
	}
	argv := BuildTerminateArgv(distribution)
	if ArgvContainsShutdown(argv) {
		return errors.New("wslhost: --shutdown is forbidden")
	}
	cmd := exec.CommandContext(ctx, r.wsl(), argv[1:]...)
	return cmd.Run()
}

type execUnit struct {
	cmd    *exec.Cmd
	argv   []string
	mu     sync.Mutex
	code   int
	exited bool
	err    error
	once   sync.Once
}

func (u *execUnit) Argv() []string { return append([]string(nil), u.argv...) }

func (u *execUnit) Wait() (int, error) {
	u.once.Do(func() {
		err := u.cmd.Wait()
		u.mu.Lock()
		defer u.mu.Unlock()
		u.exited = true
		u.err = err
		if err == nil {
			u.code = 0
			return
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			u.code = ee.ExitCode()
			u.err = nil
			return
		}
		u.code = -1
	})
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.code, u.err
}

func (u *execUnit) Kill() error {
	if u.cmd.Process == nil {
		return errors.New("wslhost: no process")
	}
	return u.cmd.Process.Kill()
}

func (u *execUnit) ExitCode() (int, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.code, u.exited
}

// ExecHostControl runs supervisor-probe/stop inside the bound distro via WSLRunner.
type ExecHostControl struct {
	WSL   WSLRunner
	Snap  Snapshot
	// Run is an injectable short-lived exec for probe/stop JSON capture.
	// When nil, uses wsl.exe --distribution --user --exec directly.
	Run func(ctx context.Context, linuxArgv []string) (stdout []byte, err error)
}

// Probe implements HostControl.
func (h *ExecHostControl) Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error) {
	_ = req
	argv := BuildProbeLinuxArgv(h.Snap, req.Timeout)
	out, err := h.run(ctx, argv)
	if err != nil {
		return ProbeResult{Class: ProbeClassUnknown, Reason: "probe_exec: " + err.Error()}, nil
	}
	return parseProbeJSON(out)
}

// Stop implements HostControl.
func (h *ExecHostControl) Stop(ctx context.Context, req StopRequest) (StopResult, error) {
	argv := BuildStopLinuxArgv(h.Snap, req.OwnershipToken)
	out, err := h.run(ctx, argv)
	if err != nil {
		return StopResult{Class: StopClassUnknown, Reason: "stop_exec: " + err.Error()}, nil
	}
	return parseStopJSON(out)
}

func (h *ExecHostControl) run(ctx context.Context, linuxArgv []string) ([]byte, error) {
	if h.Run != nil {
		return h.Run(ctx, linuxArgv)
	}
	if len(linuxArgv) == 0 {
		return nil, errors.New("wslhost: empty linux argv")
	}
	args := []string{
		"--distribution", h.Snap.Distribution,
		"--user", h.Snap.LinuxUser,
		"--exec",
	}
	args = append(args, linuxArgv...)
	cmd := exec.CommandContext(ctx, "wsl.exe", args...)
	return cmd.Output()
}
