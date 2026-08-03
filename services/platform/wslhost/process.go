package wslhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Probe / stop class strings mirror the Linux supervisor-control contract.
const (
	ProbeClassAbsent         = "absent"
	ProbeClassMatchedRunning = "matched_running"
	ProbeClassMismatch       = "mismatch"
	ProbeClassUnknown        = "unknown"

	StopClassSignaled = "signaled"
	StopClassRejected = "rejected"
	StopClassAbsent   = "absent"
	StopClassUnknown  = "unknown"
)

// SupervisedUnit is one foreground wsl.exe --exec child.
type SupervisedUnit interface {
	// Wait blocks until the unit exits and returns its exit code.
	Wait() (exitCode int, err error)
	// Kill terminates the Windows-side child (not a distro-wide shutdown).
	Kill() error
	// ExitCode returns the code once exited.
	ExitCode() (code int, exited bool)
	// Argv is the exact process argv used for launch (test/observability).
	Argv() []string
}

// LaunchSpec is the registration-bound direct-exec launch request.
type LaunchSpec struct {
	Distribution     string
	LinuxUser        string
	ServerExecutable string
	ServerArgs       []string
}

// ProbeRequest is the typed HostControl probe input.
type ProbeRequest struct {
	DataDir         string
	ExpectedExe     string
	ExpectedArgv    []string
	ArgvFingerprint string
	HealthURL       string
	Timeout         time.Duration
}

// ProbeResult is the typed HostControl probe output.
type ProbeResult struct {
	Class          string
	OwnershipToken string
	PID            int
	StartToken     string
	Reason         string
}

// StopRequest is the ownership-checked stop input.
type StopRequest struct {
	DataDir        string
	OwnershipToken string
}

// StopResult is the typed stop output.
type StopResult struct {
	Class  string
	PID    int
	Reason string
}

// HostControl is the Linux process/lock seam (supervisor-probe / supervisor-stop).
type HostControl interface {
	Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error)
	Stop(ctx context.Context, req StopRequest) (StopResult, error)
}

// WSLRunner launches and waits on registration-bound WSL units.
type WSLRunner interface {
	// StartForeground runs wsl.exe --distribution D --user U --exec server ...
	// as a long-lived child. It must not insert a shell or launcher.
	StartForeground(ctx context.Context, spec LaunchSpec) (SupervisedUnit, error)
	// TerminateDistro runs wsl.exe --terminate <bound distro> only.
	// It must never invoke --shutdown.
	TerminateDistro(ctx context.Context, distribution string) error
}

// BuildLaunchSpec projects a Snapshot into a direct-exec LaunchSpec.
func BuildLaunchSpec(s Snapshot) LaunchSpec {
	args := []string{
		"-data-dir", s.DataDir,
		"-addr", s.Addr,
		"-no-auth",
	}
	if s.TLSMode == TLSModeInsecure {
		args = append(args, "-insecure")
	}
	return LaunchSpec{
		Distribution:     s.Distribution,
		LinuxUser:        s.LinuxUser,
		ServerExecutable: s.ServerExecutable,
		ServerArgs:       args,
	}
}

// BuildWSLExecArgv returns the full wsl.exe argv for a direct foreground exec.
// Form: wsl.exe --distribution D --user U --exec <server> <server-args...>
func BuildWSLExecArgv(spec LaunchSpec) []string {
	argv := []string{
		"wsl.exe",
		"--distribution", spec.Distribution,
		"--user", spec.LinuxUser,
		"--exec",
		spec.ServerExecutable,
	}
	return append(argv, spec.ServerArgs...)
}

// BuildTerminateArgv returns the bound-distro terminate argv.
func BuildTerminateArgv(distribution string) []string {
	return []string{"wsl.exe", "--terminate", distribution}
}

// ArgvContainsShutdown reports whether argv requests an all-distribution shutdown.
func ArgvContainsShutdown(argv []string) bool {
	for _, a := range argv {
		if a == "--shutdown" {
			return true
		}
	}
	return false
}

// ArgvHasShell reports residual shell/launcher tokens that must not appear.
func ArgvHasShell(argv []string) bool {
	shells := map[string]struct{}{
		"cmd.exe": {}, "powershell.exe": {}, "pwsh.exe": {},
		"sh": {}, "bash": {}, "-c": {}, "nohup": {}, "setsid": {},
	}
	for _, a := range argv {
		base := strings.ToLower(a)
		if _, ok := shells[base]; ok {
			return true
		}
	}
	return false
}

// BuildProbeLinuxArgv is the Linux argv after --exec for supervisor-probe.
func BuildProbeLinuxArgv(s Snapshot, timeout time.Duration) []string {
	if timeout <= 0 {
		timeout = time.Duration(s.Thresholds.HealthProbeTimeoutMS) * time.Millisecond
	}
	argv := []string{
		s.ServerExecutable,
		"supervisor-probe",
		"-data-dir", s.DataDir,
		"-exe", s.ServerExecutable,
		"-health-url", s.HealthURL,
		"-health-timeout", timeout.String(),
	}
	// Expected argv is the complete Linux process argv, including argv0.
	spec := BuildLaunchSpec(s)
	for _, a := range append([]string{spec.ServerExecutable}, spec.ServerArgs...) {
		argv = append(argv, "-arg", a)
	}
	return argv
}

// BuildStopLinuxArgv is the Linux argv after --exec for supervisor-stop.
func BuildStopLinuxArgv(s Snapshot, ownershipToken string) []string {
	return []string{
		s.ServerExecutable,
		"supervisor-stop",
		"-data-dir", s.DataDir,
		"-ownership-token", ownershipToken,
	}
}

// FakeUnit is an in-process SupervisedUnit for tests.
type FakeUnit struct {
	mu      sync.Mutex
	argv    []string
	code    int
	exited  bool
	exitCh  chan struct{}
	killErr error
	waitErr error
	onKill  func()
}

// NewFakeUnit constructs a unit that has not exited.
func NewFakeUnit(argv []string) *FakeUnit {
	return &FakeUnit{argv: append([]string(nil), argv...), exitCh: make(chan struct{})}
}

// Argv implements SupervisedUnit.
func (u *FakeUnit) Argv() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.argv...)
}

// Exit completes the unit with code (idempotent).
func (u *FakeUnit) Exit(code int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.exited {
		return
	}
	u.code = code
	u.exited = true
	close(u.exitCh)
}

// Wait implements SupervisedUnit.
func (u *FakeUnit) Wait() (int, error) {
	<-u.exitCh
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.code, u.waitErr
}

// Kill implements SupervisedUnit.
func (u *FakeUnit) Kill() error {
	u.mu.Lock()
	fn := u.onKill
	err := u.killErr
	u.mu.Unlock()
	if fn != nil {
		fn()
	}
	if err != nil {
		return err
	}
	u.Exit(1)
	return nil
}

// ExitCode implements SupervisedUnit.
func (u *FakeUnit) ExitCode() (int, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.code, u.exited
}

// FakeWSLRunner is the scripted WSL runtime fake.
type FakeWSLRunner struct {
	mu sync.Mutex

	// StartFn optionally overrides StartForeground.
	StartFn func(ctx context.Context, spec LaunchSpec) (SupervisedUnit, error)
	// TerminateFn optionally overrides TerminateDistro.
	TerminateFn func(ctx context.Context, distribution string) error

	Starts     []LaunchSpec
	Terminates []string
	LastArgv   []string
	Units      []*FakeUnit
	StartErr   error
	TermErr    error
	// AutoExitCode, when non-nil, exits each new unit immediately with *AutoExitCode.
	AutoExitCode *int
}

// StartForeground implements WSLRunner.
func (f *FakeWSLRunner) StartForeground(ctx context.Context, spec LaunchSpec) (SupervisedUnit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StartFn != nil {
		return f.StartFn(ctx, spec)
	}
	f.Starts = append(f.Starts, spec)
	argv := BuildWSLExecArgv(spec)
	f.LastArgv = append([]string(nil), argv...)
	if f.StartErr != nil {
		return nil, f.StartErr
	}
	if ArgvContainsShutdown(argv) || ArgvHasShell(argv) {
		return nil, errors.New("wslhost: invalid launch argv")
	}
	u := NewFakeUnit(argv)
	f.Units = append(f.Units, u)
	if f.AutoExitCode != nil {
		code := *f.AutoExitCode
		go u.Exit(code)
	}
	return u, nil
}

// TerminateDistro implements WSLRunner.
func (f *FakeWSLRunner) TerminateDistro(ctx context.Context, distribution string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.TerminateFn != nil {
		return f.TerminateFn(ctx, distribution)
	}
	f.Terminates = append(f.Terminates, distribution)
	argv := BuildTerminateArgv(distribution)
	if ArgvContainsShutdown(argv) {
		return errors.New("wslhost: shutdown forbidden")
	}
	return f.TermErr
}

// SpawnCount returns how many StartForeground calls succeeded or were attempted.
func (f *FakeWSLRunner) SpawnCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Starts)
}

// FakeHostControl is the scripted probe/stop client.
type FakeHostControl struct {
	mu sync.Mutex

	ProbeFn func(ctx context.Context, req ProbeRequest) (ProbeResult, error)
	StopFn  func(ctx context.Context, req StopRequest) (StopResult, error)

	Probes []ProbeRequest
	Stops  []StopRequest

	NextProbe ProbeResult
	ProbeErr  error
	NextStop  StopResult
	StopErr   error

	// ProbeQueue, when non-empty, pops results FIFO before NextProbe.
	ProbeQueue []ProbeResult
}

// Probe implements HostControl.
func (f *FakeHostControl) Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Probes = append(f.Probes, req)
	if f.ProbeFn != nil {
		return f.ProbeFn(ctx, req)
	}
	if f.ProbeErr != nil {
		return ProbeResult{}, f.ProbeErr
	}
	if len(f.ProbeQueue) > 0 {
		r := f.ProbeQueue[0]
		f.ProbeQueue = f.ProbeQueue[1:]
		return r, nil
	}
	return f.NextProbe, nil
}

// Stop implements HostControl.
func (f *FakeHostControl) Stop(ctx context.Context, req StopRequest) (StopResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Stops = append(f.Stops, req)
	if f.StopFn != nil {
		return f.StopFn(ctx, req)
	}
	if f.StopErr != nil {
		return StopResult{}, f.StopErr
	}
	if f.NextStop.Class == "" {
		return StopResult{Class: StopClassSignaled, Reason: "fake_signaled"}, nil
	}
	return f.NextStop, nil
}

// SetProbe sets the next probe result.
func (f *FakeHostControl) SetProbe(r ProbeResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.NextProbe = r
}

// ProbeCount returns how many probes were issued.
func (f *FakeHostControl) ProbeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Probes)
}

// ScriptedHostControl maps data-dir keys to fixed classes for fidelity tests.
type ScriptedHostControl struct {
	ByClass map[string]ProbeResult // unused key; use Class field scripts
	Results []ProbeResult
	idx     int
	mu      sync.Mutex
	Tokens  map[string]string // token -> dataDir for stop revalidation
}

// Probe implements HostControl.
func (s *ScriptedHostControl) Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idx >= len(s.Results) {
		return ProbeResult{Class: ProbeClassUnknown, Reason: "script_exhausted"}, nil
	}
	r := s.Results[s.idx]
	s.idx++
	if r.Class == ProbeClassMatchedRunning && r.OwnershipToken != "" {
		if s.Tokens == nil {
			s.Tokens = make(map[string]string)
		}
		s.Tokens[r.OwnershipToken] = req.DataDir
	}
	return r, nil
}

// Stop implements HostControl with token revalidation.
func (s *ScriptedHostControl) Stop(ctx context.Context, req StopRequest) (StopResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.OwnershipToken == "" {
		return StopResult{Class: StopClassRejected, Reason: "missing_token"}, nil
	}
	if dd, ok := s.Tokens[req.OwnershipToken]; !ok || dd != req.DataDir {
		return StopResult{Class: StopClassRejected, Reason: "token_mismatch"}, nil
	}
	return StopResult{Class: StopClassSignaled, Reason: "sigterm_delivered"}, nil
}

// parseProbeJSON decodes supervisor-probe stdout into ProbeResult.
func parseProbeJSON(raw []byte) (ProbeResult, error) {
	var wire struct {
		Class          string `json:"class"`
		OwnershipToken string `json:"ownership_token"`
		PID            int    `json:"pid"`
		StartToken     string `json:"start_token"`
		Reason         string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ProbeResult{Class: ProbeClassUnknown, Reason: "probe_json: " + err.Error()}, nil
	}
	switch wire.Class {
	case ProbeClassAbsent, ProbeClassMatchedRunning, ProbeClassMismatch, ProbeClassUnknown:
	default:
		return ProbeResult{
			Class:  ProbeClassUnknown,
			Reason: "probe_json: unrecognized class " + wire.Class,
		}, fmt.Errorf("wslhost: unrecognized probe class %q", wire.Class)
	}
	return ProbeResult{
		Class:          wire.Class,
		OwnershipToken: wire.OwnershipToken,
		PID:            wire.PID,
		StartToken:     wire.StartToken,
		Reason:         wire.Reason,
	}, nil
}

// parseStopJSON decodes supervisor-stop stdout into StopResult.
func parseStopJSON(raw []byte) (StopResult, error) {
	var wire struct {
		Class  string `json:"class"`
		PID    int    `json:"pid"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return StopResult{Class: StopClassUnknown, Reason: "stop_json: " + err.Error()}, nil
	}
	switch wire.Class {
	case StopClassSignaled, StopClassRejected, StopClassAbsent, StopClassUnknown:
	default:
		return StopResult{
			Class:  StopClassUnknown,
			Reason: "stop_json: unrecognized class " + wire.Class,
		}, fmt.Errorf("wslhost: unrecognized stop class %q", wire.Class)
	}
	return StopResult{Class: wire.Class, PID: wire.PID, Reason: wire.Reason}, nil
}

// ValidateLaunchArgv enforces direct-exec invariants for a captured argv.
func ValidateLaunchArgv(argv []string, boundDistro, boundUser string) error {
	if len(argv) < 7 {
		return fmt.Errorf("wslhost: launch argv too short: %v", argv)
	}
	if argv[0] != "wsl.exe" {
		return fmt.Errorf("wslhost: expected wsl.exe, got %q", argv[0])
	}
	if ArgvContainsShutdown(argv) {
		return errors.New("wslhost: --shutdown is forbidden")
	}
	if ArgvHasShell(argv) {
		return errors.New("wslhost: shell/launcher tokens forbidden")
	}
	var distro, user string
	hasExec := false
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--distribution":
			if i+1 < len(argv) {
				distro = argv[i+1]
			}
		case "--user":
			if i+1 < len(argv) {
				user = argv[i+1]
			}
		case "--exec":
			hasExec = true
		}
	}
	if !hasExec {
		return errors.New("wslhost: missing --exec")
	}
	if distro != boundDistro {
		return fmt.Errorf("wslhost: distro %q != bound %q", distro, boundDistro)
	}
	if user != boundUser {
		return fmt.Errorf("wslhost: user %q != bound %q", user, boundUser)
	}
	return nil
}
