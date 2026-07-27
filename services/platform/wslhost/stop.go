package wslhost

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 8s drain + 5s gateway grace + 2s unit exit = 15s (OPT-DERIVE-FROM-SERVER-BUDGET).
const (
	ServerDrainBudgetMS      = 8_000
	ServerGatewayGraceMS     = 5_000
	SupervisedUnitExitOverMS = 2_000
	ChildKillWaitMS          = 2_000
)

type StopPath string

const (
	StopPathSCM      StopPath = "scm_stop"
	StopPathRecovery StopPath = "recovery"
)

type StopCheckpoint string

const (
	CheckpointAccepted         StopCheckpoint = "stop_accepted"
	CheckpointDuplicateIgnored StopCheckpoint = "duplicate_ignored"
	CheckpointTokenMissing     StopCheckpoint = "token_missing"
	CheckpointStopRejected     StopCheckpoint = "stop_rejected"
	CheckpointSIGTERMDelivered StopCheckpoint = "sigterm_delivered"
	CheckpointGracefulWaiting  StopCheckpoint = "graceful_waiting"
	CheckpointGracefulExited   StopCheckpoint = "graceful_exited"
	CheckpointBudgetExpired    StopCheckpoint = "budget_expired"
	CheckpointChildKill        StopCheckpoint = "child_kill"
	CheckpointChildWait        StopCheckpoint = "child_wait"
	CheckpointEvidenceProbe    StopCheckpoint = "evidence_probe"
	CheckpointTerminateDistro  StopCheckpoint = "terminate_distro"
	CheckpointAbsenceConfirmed StopCheckpoint = "absence_confirmed"
	CheckpointCompleted        StopCheckpoint = "completed"
	CheckpointTerminateFailed  StopCheckpoint = "terminate_failed"
	CheckpointEvidenceCleared  StopCheckpoint = "evidence_cleared"
)

const (
	EscalationChildKill       = "child_kill"
	EscalationTerminateDistro = "terminate_distro"
)

// GracefulWaiter waits for unit exit or budget expiry.
type GracefulWaiter interface {
	Wait(unit SupervisedUnit, budget time.Duration) bool
}

// RealGracefulWaiter uses wall clock + unit.Wait.
type RealGracefulWaiter struct{}

func (RealGracefulWaiter) Wait(unit SupervisedUnit, budget time.Duration) bool {
	if unit == nil {
		time.Sleep(budget)
		return false
	}
	if _, ok := unit.ExitCode(); ok {
		return true
	}
	done := make(chan struct{})
	go func() { _, _ = unit.Wait(); close(done) }()
	t := time.NewTimer(budget)
	defer t.Stop()
	select {
	case <-done:
		return true
	case <-t.C:
		return false
	}
}

// FakeClockWaiter advances ManualClock by budget when the unit is still live.
type FakeClockWaiter struct{ Clock *ManualClock }

func (w *FakeClockWaiter) Wait(unit SupervisedUnit, budget time.Duration) bool {
	if unit != nil {
		if _, ok := unit.ExitCode(); ok {
			return true
		}
	}
	if w.Clock != nil && budget > 0 {
		w.Clock.Advance(budget)
	}
	if unit != nil {
		if _, ok := unit.ExitCode(); ok {
			return true
		}
	}
	return false
}

// SleepWaiter is a fixed delay (SCM child-kill wait).
type SleepWaiter interface{ Sleep(d time.Duration) }

type FakeClockSleeper struct{ Clock *ManualClock }

func (s FakeClockSleeper) Sleep(d time.Duration) {
	if s.Clock != nil && d > 0 {
		s.Clock.Advance(d)
	}
}

type RealSleeper struct{}

func (RealSleeper) Sleep(d time.Duration) { time.Sleep(d) }

// StopSessionState is ownership + unit authority for one transaction.
type StopSessionState struct {
	OwnershipToken string
	Unit           SupervisedUnit
	Generation     string
}

type StopSessionSource interface{ StopSession() StopSessionState }

type FuncStopSession func() StopSessionState

func (f FuncStopSession) StopSession() StopSessionState { return f() }

// StopOutcome is the terminal result of one stop transaction.
type StopOutcome struct {
	Path             StopPath
	Completed        bool
	Rejected         bool
	Duplicate        bool
	Class            string
	Reason           string
	Escalation       string
	TerminateTarget  string
	Checkpoints      []StopCheckpoint
	BudgetMS         int
	UsedShutdown     bool
	SIGTERMIssued    bool
	ChildKilled      bool
	DistroTerminated bool
}

type StopStatusSink interface {
	OnCheckpoint(cp StopCheckpoint, waitHintMS uint32, checkpoint uint32)
}

// StopCoordinator serializes SCM/recovery stop escalation (alg-stop-escalation).
type StopCoordinator struct {
	mu       sync.Mutex
	Snap     Snapshot
	Host     HostControl
	WSL      WSLRunner
	Source   StopSessionSource
	Wait     GracefulWaiter
	Sleeper  SleepWaiter
	Status   StopStatusSink
	inFlight bool
	last     StopOutcome
	cpSeq    uint32
}

func NewStopCoordinator(snap Snapshot, host HostControl, wsl WSLRunner, source StopSessionSource) (*StopCoordinator, error) {
	if err := ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	if host == nil || wsl == nil || source == nil {
		return nil, errors.New("wslhost: Host, WSL, and Source are required for stop")
	}
	return &StopCoordinator{
		Snap: snap, Host: host, WSL: wsl, Source: source,
		Wait: RealGracefulWaiter{}, Sleeper: RealSleeper{},
	}, nil
}

// DeriveGracefulStopBudgetMS is 8+5+2 from the server drain/grace contract.
func DeriveGracefulStopBudgetMS() int {
	return ServerDrainBudgetMS + ServerGatewayGraceMS + SupervisedUnitExitOverMS
}

func (c *StopCoordinator) GracefulBudget() time.Duration {
	ms := c.Snap.Thresholds.GracefulStopBudgetMS
	if ms <= 0 {
		ms = DeriveGracefulStopBudgetMS()
	}
	return time.Duration(ms) * time.Millisecond
}

func (c *StopCoordinator) LastOutcome() StopOutcome {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneOutcome(c.last)
}

func (c *StopCoordinator) InFlight() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inFlight
}

// Run executes one stop path; concurrent calls collapse onto the in-flight transaction.
func (c *StopCoordinator) Run(ctx context.Context, path StopPath) StopOutcome {
	c.mu.Lock()
	if c.inFlight {
		out := cloneOutcome(c.last)
		out.Duplicate = true
		out.Checkpoints = append(append([]StopCheckpoint(nil), out.Checkpoints...), CheckpointDuplicateIgnored)
		c.last = out
		c.mu.Unlock()
		return cloneOutcome(out)
	}
	c.inFlight, c.cpSeq = true, 0
	c.last = StopOutcome{Path: path, BudgetMS: int(c.GracefulBudget() / time.Millisecond)}
	c.mu.Unlock()
	out := c.runTransaction(ctx, path)
	c.mu.Lock()
	c.inFlight, c.last = false, out
	c.mu.Unlock()
	return cloneOutcome(out)
}

func (c *StopCoordinator) runTransaction(ctx context.Context, path StopPath) StopOutcome {
	out := StopOutcome{Path: path, BudgetMS: int(c.GracefulBudget() / time.Millisecond)}
	c.mark(&out, CheckpointAccepted)
	session := c.Source.StopSession()
	if session.OwnershipToken == "" {
		out.Rejected, out.Reason, out.Class = true, "missing_ownership_token", StopClassRejected
		c.mark(&out, CheckpointTokenMissing)
		c.mark(&out, CheckpointCompleted)
		return out
	}
	stopRes, err := c.Host.Stop(ctx, StopRequest{DataDir: c.Snap.DataDir, OwnershipToken: session.OwnershipToken})
	if err != nil {
		out.Rejected, out.Reason, out.Class = true, "stop_error: "+err.Error(), StopClassUnknown
		c.mark(&out, CheckpointStopRejected)
		c.mark(&out, CheckpointCompleted)
		return out
	}
	out.Class, out.Reason = stopRes.Class, stopRes.Reason
	switch stopRes.Class {
	case StopClassRejected, StopClassUnknown:
		out.Rejected = true
		c.mark(&out, CheckpointStopRejected)
		c.mark(&out, CheckpointCompleted)
		return out
	case StopClassAbsent:
		c.mark(&out, CheckpointSIGTERMDelivered)
		c.mark(&out, CheckpointGracefulExited)
		c.confirmAbsence(ctx, &out)
		out.Completed = true
		c.mark(&out, CheckpointCompleted)
		return out
	default:
		out.SIGTERMIssued = true
		c.mark(&out, CheckpointSIGTERMDelivered)
	}
	c.mark(&out, CheckpointGracefulWaiting)
	waiter := c.Wait
	if waiter == nil {
		waiter = RealGracefulWaiter{}
	}
	if c.waitForGracefulExit(ctx, waiter, session.Unit) {
		c.mark(&out, CheckpointGracefulExited)
		c.confirmAbsence(ctx, &out)
		out.Completed = true
		c.mark(&out, CheckpointCompleted)
		return out
	}
	c.mark(&out, CheckpointBudgetExpired)
	switch path {
	case StopPathSCM:
		c.scmEscalate(ctx, &out, session.Unit)
	case StopPathRecovery:
		c.terminateBound(ctx, &out) // skip child kill (UAC-013)
	default:
		out.Rejected, out.Reason = true, fmt.Sprintf("unknown_stop_path:%s", path)
		c.mark(&out, CheckpointCompleted)
		return out
	}
	c.confirmAbsence(ctx, &out)
	out.Completed = !out.Rejected
	c.mark(&out, CheckpointCompleted)
	return out
}

func (c *StopCoordinator) waitForGracefulExit(ctx context.Context, waiter GracefulWaiter, unit SupervisedUnit) bool {
	if unit != nil {
		return waiter.Wait(unit, c.GracefulBudget())
	}
	timer := time.NewTimer(c.GracefulBudget())
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		probe, err := c.Host.Probe(ctx, c.probeReq())
		if err == nil && probe.Class == ProbeClassAbsent {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-ticker.C:
		}
	}
}

func (c *StopCoordinator) scmEscalate(ctx context.Context, out *StopOutcome, unit SupervisedUnit) {
	if unit != nil {
		c.mark(out, CheckpointChildKill)
		if err := unit.Kill(); err != nil {
			out.Reason = "child_kill: " + err.Error()
		}
		out.ChildKilled, out.Escalation = true, EscalationChildKill
		c.mark(out, CheckpointChildWait)
		sleeper := c.Sleeper
		if sleeper == nil {
			sleeper = RealSleeper{}
		}
		sleeper.Sleep(time.Duration(ChildKillWaitMS) * time.Millisecond)
	}
	c.mark(out, CheckpointEvidenceProbe)
	if !c.evidencePersists(ctx) {
		c.mark(out, CheckpointEvidenceCleared)
		return
	}
	c.terminateBound(ctx, out)
}

func (c *StopCoordinator) terminateBound(ctx context.Context, out *StopOutcome) {
	distro := c.Snap.Distribution
	if ArgvContainsShutdown(BuildTerminateArgv(distro)) {
		out.Rejected, out.Reason = true, "shutdown_forbidden"
		c.mark(out, CheckpointTerminateFailed)
		return
	}
	c.mark(out, CheckpointTerminateDistro)
	if err := c.WSL.TerminateDistro(ctx, distro); err != nil {
		out.Rejected, out.Reason = true, "terminate: "+err.Error()
		c.mark(out, CheckpointTerminateFailed)
		return
	}
	out.DistroTerminated, out.TerminateTarget = true, distro
	out.Escalation, out.UsedShutdown = EscalationTerminateDistro, false
}

func (c *StopCoordinator) evidencePersists(ctx context.Context) bool {
	pr, err := c.Host.Probe(ctx, c.probeReq())
	return err == nil && pr.Class == ProbeClassMatchedRunning
}

func (c *StopCoordinator) confirmAbsence(ctx context.Context, out *StopOutcome) {
	pr, err := c.Host.Probe(ctx, c.probeReq())
	if err == nil && pr.Class == ProbeClassAbsent {
		c.mark(out, CheckpointAbsenceConfirmed)
	}
}

func (c *StopCoordinator) probeReq() ProbeRequest {
	return ProbeRequest{
		DataDir: c.Snap.DataDir, ExpectedExe: c.Snap.ServerExecutable,
		ExpectedArgv: expectedServerArgv(c.Snap), HealthURL: c.Snap.HealthURL,
		Timeout: time.Duration(c.Snap.Thresholds.HealthProbeTimeoutMS) * time.Millisecond,
	}
}

func (c *StopCoordinator) mark(out *StopOutcome, cp StopCheckpoint) {
	out.Checkpoints = append(out.Checkpoints, cp)
	c.mu.Lock()
	c.cpSeq++
	seq := c.cpSeq
	c.last = cloneOutcome(*out)
	c.mu.Unlock()
	if c.Status != nil {
		c.Status.OnCheckpoint(cp, c.checkpointWaitHint(cp), seq)
	}
}

func (c *StopCoordinator) checkpointWaitHint(cp StopCheckpoint) uint32 {
	switch cp {
	case CheckpointChildWait:
		return ChildKillWaitMS
	case CheckpointCompleted, CheckpointGracefulExited, CheckpointAbsenceConfirmed:
		return 0
	case CheckpointAccepted, CheckpointDuplicateIgnored, CheckpointTokenMissing,
		CheckpointStopRejected, CheckpointSIGTERMDelivered, CheckpointGracefulWaiting,
		CheckpointBudgetExpired, CheckpointChildKill, CheckpointEvidenceProbe,
		CheckpointTerminateDistro, CheckpointTerminateFailed, CheckpointEvidenceCleared:
		return uint32(c.GracefulBudget() / time.Millisecond)
	}
	return uint32(c.GracefulBudget() / time.Millisecond)
}

func cloneOutcome(o StopOutcome) StopOutcome {
	o.Checkpoints = append([]StopCheckpoint(nil), o.Checkpoints...)
	return o
}

func AssertNoShutdownInvariant(argv []string) error {
	if ArgvContainsShutdown(argv) {
		return errors.New("wslhost: --shutdown is forbidden on stop escalation")
	}
	return nil
}
