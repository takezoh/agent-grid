package wslhost

import (
	"context"
	"sync"
	"testing"
	"time"
)

type stopHarness struct {
	snap  Snapshot
	clock *ManualClock
	wsl   *FakeWSLRunner
	host  *FakeHostControl
	unit  *FakeUnit
	token string
	coord *StopCoordinator
}

func newStopHarness(t *testing.T) *stopHarness {
	t.Helper()
	snap := testSnap(t)
	clock := NewManualClock(time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC))
	h := &stopHarness{
		snap: snap, clock: clock, wsl: &FakeWSLRunner{},
		host: &FakeHostControl{
			NextStop:  StopResult{Class: StopClassSignaled, Reason: "sigterm_delivered"},
			NextProbe: ProbeResult{Class: ProbeClassAbsent, Reason: "gone"},
		},
		unit: NewFakeUnit(BuildWSLExecArgv(BuildLaunchSpec(snap))), token: "own-token-1",
	}
	coord, err := NewStopCoordinator(snap, h.host, h.wsl, FuncStopSession(func() StopSessionState {
		return StopSessionState{OwnershipToken: h.token, Unit: h.unit, Generation: snap.Generation}
	}))
	if err != nil {
		t.Fatal(err)
	}
	coord.Wait = &FakeClockWaiter{Clock: clock}
	coord.Sleeper = FakeClockSleeper{Clock: clock}
	h.coord = coord
	return h
}

func hasCP(out StopOutcome, cp StopCheckpoint) bool {
	for _, c := range out.Checkpoints {
		if c == cp {
			return true
		}
	}
	return false
}

// witness-performance-budget-shutdown-derivation
func TestDeriveGracefulStopBudgetPreservesDrainAndGrace(t *testing.T) {
	got := DeriveGracefulStopBudgetMS()
	if got != 15_000 {
		t.Fatalf("budget=%d", got)
	}
	if got != DefaultGracefulStopMS {
		t.Fatalf("default=%d", DefaultGracefulStopMS)
	}
	// rejects counterexample: 8s cut-off or ungrounded 30s
	if ServerDrainBudgetMS != 8_000 || ServerGatewayGraceMS != 5_000 || got == 30_000 {
		t.Fatal("provenance")
	}
}

// UAC-008
func TestUAC008_GracefulSCMStopWithinBudget(t *testing.T) {
	h := newStopHarness(t)
	h.unit.Exit(0)
	out := h.coord.Run(context.Background(), StopPathSCM)
	if !out.Completed || !out.SIGTERMIssued || out.ChildKilled || len(h.wsl.Terminates) != 0 {
		t.Fatalf("%+v", out)
	}
}

// UAC-009/010
func TestUAC009_010_SCMEscalateBoundOnlyNoShutdown(t *testing.T) {
	h := newStopHarness(t)
	h.host.ProbeQueue = []ProbeResult{
		{Class: ProbeClassMatchedRunning, OwnershipToken: h.token}, {Class: ProbeClassAbsent},
	}
	out := h.coord.Run(context.Background(), StopPathSCM)
	if !out.Completed || !out.ChildKilled || out.TerminateTarget != h.snap.Distribution {
		t.Fatalf("%+v", out)
	}
	if out.UsedShutdown || AssertNoShutdownInvariant(BuildTerminateArgv(out.TerminateTarget)) != nil {
		t.Fatal("shutdown")
	}
}

// UAC-013
func TestUAC013_RecoverySkipsChildKill(t *testing.T) {
	h := newStopHarness(t)
	out := h.coord.Run(context.Background(), StopPathRecovery)
	if !out.Completed || out.ChildKilled || !out.DistroTerminated || out.BudgetMS != 15_000 || hasCP(out, CheckpointChildKill) {
		t.Fatalf("%+v", out)
	}
}

func TestStopRejectedOnStaleToken(t *testing.T) {
	h := newStopHarness(t)
	h.host.NextStop = StopResult{Class: StopClassRejected, Reason: "token_mismatch"}
	out := h.coord.Run(context.Background(), StopPathSCM)
	if !out.Rejected || out.SIGTERMIssued || len(h.wsl.Terminates) != 0 {
		t.Fatalf("%+v", out)
	}
}

func TestSCMNoTerminateWhenEvidenceClears(t *testing.T) {
	h := newStopHarness(t)
	h.host.ProbeQueue = []ProbeResult{{Class: ProbeClassAbsent}}
	out := h.coord.Run(context.Background(), StopPathSCM)
	if !out.ChildKilled || out.DistroTerminated || !hasCP(out, CheckpointEvidenceCleared) {
		t.Fatalf("%+v", out)
	}
}

func TestAdoptedInstanceStopObservesAbsenceWithoutFullBudgetSleep(t *testing.T) {
	h := newStopHarness(t)
	h.coord.Source = FuncStopSession(func() StopSessionState {
		return StopSessionState{OwnershipToken: h.token, Unit: nil, Generation: h.snap.Generation}
	})
	h.host.NextProbe = ProbeResult{Class: ProbeClassAbsent}
	start := time.Now()
	out := h.coord.Run(context.Background(), StopPathSCM)
	if !out.Completed || !out.SIGTERMIssued || out.ChildKilled || out.DistroTerminated {
		t.Fatalf("%+v", out)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("adopted stop waited too long: %v", elapsed)
	}
}

// TestContractSCMStopSerialization
func TestContractSCMStopSerialization(t *testing.T) {
	h := newStopHarness(t)
	var started, release sync.WaitGroup
	started.Add(1)
	release.Add(1)
	h.coord.Wait = gracefulWaitFunc(func(SupervisedUnit, time.Duration) bool {
		started.Done()
		release.Wait()
		return true
	})
	h.unit.Exit(0)
	var first StopOutcome
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); first = h.coord.Run(context.Background(), StopPathSCM) }()
	started.Wait()
	dup := h.coord.Run(context.Background(), StopPathSCM)
	if !dup.Duplicate || !hasCP(dup, CheckpointDuplicateIgnored) || len(h.host.Stops) != 1 {
		t.Fatalf("dup=%+v", dup)
	}
	release.Done()
	wg.Wait()
	if !first.Completed || len(h.host.Stops) != 1 {
		t.Fatalf("first=%+v", first)
	}
}

type gracefulWaitFunc func(SupervisedUnit, time.Duration) bool

func (f gracefulWaitFunc) Wait(u SupervisedUnit, d time.Duration) bool { return f(u, d) }

// TestFakeVsRealWindowsSCMUnsignedIn
func TestFakeVsRealWindowsSCMUnsignedIn(t *testing.T) {
	snap := testSnap(t)
	clock := NewManualClock(time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC))
	unit := NewFakeUnit(BuildWSLExecArgv(BuildLaunchSpec(snap)))
	unit.Exit(0)
	scm := NewFakeSCM()
	hp := &FakeHealthProber{}
	hp.SetOK()
	rec := &MemoryRecorder{}
	sh, err := NewServiceHost(snap, Deps{
		WSL: &FakeWSLRunner{}, Host: &FakeHostControl{
			NextStop: StopResult{Class: StopClassSignaled}, NextProbe: ProbeResult{Class: ProbeClassAbsent},
		}, Health: hp, Clock: clock, Timers: clock, Record: rec,
	}, scm, FuncStopSession(func() StopSessionState {
		return StopSessionState{OwnershipToken: "fid", Unit: unit, Generation: snap.Generation}
	}))
	if err != nil {
		t.Fatal(err)
	}
	sh.Stop.Wait = &FakeClockWaiter{Clock: clock}
	sh.RunLifecycle = func(context.Context) {}
	out, err := sh.HandleControl(context.Background(), CmdStop)
	if err != nil || !out.Completed || !out.SIGTERMIssued {
		t.Fatalf("%+v %v", out, err)
	}
	var pending, stopped bool
	for _, s := range scm.SnapshotStatuses() {
		pending = pending || s.State == StateStopPending
		stopped = stopped || s.State == StateStopped
	}
	if !pending || !stopped || uint32(StateStopped) != 1 {
		t.Fatalf("%+v", scm.SnapshotStatuses())
	}
	if rec.CountKind(KindServiceStop) != 1 {
		t.Fatalf("service stop records = %d", rec.CountKind(KindServiceStop))
	}
}

func TestFakeVsRealStopEscalationArgv(t *testing.T) {
	h := newStopHarness(t)
	h.host.ProbeQueue = []ProbeResult{
		{Class: ProbeClassMatchedRunning, OwnershipToken: h.token}, {Class: ProbeClassAbsent},
	}
	out := h.coord.Run(context.Background(), StopPathSCM)
	want := BuildTerminateArgv(h.snap.Distribution)
	if out.TerminateTarget != want[2] || ArgvContainsShutdown(want) {
		t.Fatalf("%+v", out)
	}
}

func TestServiceContextCancellationKeepsStopAuthorityLive(t *testing.T) {
	snap := testSnap(t)
	clock := NewManualClock(time.Now())
	hostControl := &FakeHostControl{
		NextProbe: ProbeResult{Class: ProbeClassAbsent},
		StopFn: func(ctx context.Context, _ StopRequest) (StopResult, error) {
			if err := ctx.Err(); err != nil {
				t.Fatalf("stop received canceled context: %v", err)
			}
			return StopResult{Class: StopClassSignaled}, nil
		},
	}
	unit := NewFakeUnit(BuildWSLExecArgv(BuildLaunchSpec(snap)))
	unit.Exit(0)
	scm := NewFakeSCM()
	service, err := NewServiceHost(snap, Deps{
		WSL: &FakeWSLRunner{}, Host: hostControl, Health: &FakeHealthProber{},
		Clock: clock, Timers: clock, Record: &MemoryRecorder{},
	}, scm, FuncStopSession(func() StopSessionState {
		return StopSessionState{OwnershipToken: "token", Unit: unit, Generation: snap.Generation}
	}))
	if err != nil {
		t.Fatal(err)
	}
	service.RunLifecycle = func(context.Context) {}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Serve(ctx); err != nil {
		t.Fatal(err)
	}
}
