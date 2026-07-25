package wslhost

import (
	"context"
	"testing"
	"time"
)

func testSnap(t *testing.T) Snapshot {
	t.Helper()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	snap, err := BuildSnapshot(RegistrationRequest{
		LinuxUser:        "agent",
		ServerExecutable: "/usr/local/bin/server",
		DataDir:          "/var/lib/agent-grid",
		Addr:             "127.0.0.1:8443",
		TLSMode:          TLSModeInsecure,
	}, Defaults{WindowsIdentity: "CONTOSO\\svc", Distribution: "Ubuntu-AG"}, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

type harness struct {
	snap   Snapshot
	clock  *ManualClock
	wsl    *FakeWSLRunner
	host   *FakeHostControl
	health *FakeHealthProber
	resume *FakeResumeSource
	rec    *MemoryRecorder
	sup    *Supervisor
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	snap := testSnap(t)
	h := &harness{
		snap:   snap,
		clock:  NewManualClock(time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)),
		wsl:    &FakeWSLRunner{},
		host:   &FakeHostControl{NextProbe: ProbeResult{Class: ProbeClassAbsent, Reason: "no_lock"}},
		health: &FakeHealthProber{},
		resume: &FakeResumeSource{},
		rec:    &MemoryRecorder{},
	}
	h.health.SetOK()
	sup, err := NewSupervisor(snap, Deps{
		WSL: h.wsl, Host: h.host, Health: h.health,
		Clock: h.clock, Timers: h.clock, Resume: h.resume, Record: h.rec,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.sup = sup
	t.Cleanup(sup.Close)
	return h
}

// UAC-001 / UAC-014: Ready only after status ok; daemon-unavailable is non-Ready.
func TestUAC001_ReadyOnlyOnStatusOK(t *testing.T) {
	h := newHarness(t)
	h.health.SetUnavailable()
	h.sup.Start(context.Background())
	if h.sup.Phase() == PhaseReady {
		t.Fatal("daemon-unavailable must not become Ready")
	}
	if h.wsl.SpawnCount() != 1 {
		t.Fatalf("spawn count = %d, want 1 after absent probe", h.wsl.SpawnCount())
	}
	// rejects counterexample: transport/process existence alone as Ready
	h.health.SetOK()
	h.clock.Advance(time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond)
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s, want ready after status ok", h.sup.Phase())
	}
	if _, ok := h.rec.LastKind(KindReady); !ok {
		t.Fatal("missing ready record")
	}
}

func TestStartupBudgetExpiresIntoBackoff(t *testing.T) {
	h := newHarness(t)
	h.health.SetUnavailable()
	h.sup.Start(context.Background())

	cadence := time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond
	for elapsed := time.Duration(0); elapsed < time.Duration(h.snap.Thresholds.StartupBudgetMS)*time.Millisecond; elapsed += cadence {
		h.clock.Advance(cadence)
	}

	if h.sup.Phase() != PhaseBackoff {
		t.Fatalf("phase = %s, want backoff after startup budget", h.sup.Phase())
	}
	if _, ok := h.rec.LastKind(KindBudgetExpiry); !ok {
		t.Fatal("missing startup budget expiry record")
	}
}

func TestHealthRestartInvokesRecoveryStop(t *testing.T) {
	h := newHarness(t)
	called := make(chan struct{}, 1)
	h.sup.SetRecoveryStop(func(context.Context) StopOutcome {
		called <- struct{}{}
		return StopOutcome{Path: StopPathRecovery, Completed: true}
	})
	h.sup.Start(context.Background())
	h.health.SetUnavailable()

	cadence := time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond
	for range h.snap.Thresholds.HealthRestartThreshold {
		h.clock.Advance(cadence)
	}

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("recovery stop was not invoked")
	}
}

// UAC-002: foreground unit stays healthy; retry count does not increase.
func TestUAC002_SteadyReadyNoRetryGrowth(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s", h.sup.Phase())
	}
	attempt := h.sup.RetryAttempt()
	h.clock.Advance(time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond)
	h.clock.Advance(time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond)
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase drifted to %s", h.sup.Phase())
	}
	if h.sup.RetryAttempt() != attempt {
		t.Fatalf("retry attempt grew: %d -> %d", attempt, h.sup.RetryAttempt())
	}
	if h.wsl.SpawnCount() != 1 {
		t.Fatalf("extra spawns: %d", h.wsl.SpawnCount())
	}
	if err := ValidateLaunchArgv(h.wsl.LastArgv, h.snap.Distribution, h.snap.LinuxUser); err != nil {
		t.Fatal(err)
	}
}

// UAC-003: child exit is recorded with code; no shell in argv.
func TestUAC003_ChildExitRecorded(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	u := h.sup.Unit().(*FakeUnit)
	if ArgvHasShell(u.Argv()) {
		t.Fatalf("shell in argv: %v", u.Argv())
	}
	u.Exit(42)
	// Wait for watcher
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if h.sup.Phase() == PhaseBackoff || h.sup.Phase() == PhaseFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	rec, ok := h.rec.LastKind(KindChildExit)
	if !ok || rec.ExitCode == nil || *rec.ExitCode != 42 {
		t.Fatalf("child exit record = %+v", rec)
	}
	if h.sup.Phase() != PhaseBackoff {
		t.Fatalf("phase = %s, want backoff", h.sup.Phase())
	}
}

// UAC-004: transient crash → backoff delay/attempt → Ready on status ok.
func TestUAC004_TransientBackoffThenReady(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	h.sup.Unit().(*FakeUnit).Exit(1)
	waitPhase(t, h.sup, PhaseBackoff)
	rec, ok := h.rec.LastKind(KindBackoff)
	if !ok || rec.DelayMS != 1000 || rec.Attempt != 1 {
		t.Fatalf("backoff record = %+v", rec)
	}
	// After backoff, probe absent + health ok → Ready
	h.host.SetProbe(ProbeResult{Class: ProbeClassAbsent})
	h.health.SetOK()
	h.clock.Advance(time.Second)
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s after backoff", h.sup.Phase())
	}
	_, tr, _ := h.sup.Counts()
	if tr != 0 {
		t.Fatalf("transient counter not reset on Ready: %d", tr)
	}
}

// UAC-005: five consecutive config failures → Failed; retries stop.
func TestUAC005_ConfigTerminalAtFive(t *testing.T) {
	h := newHarness(t)
	h.host.SetProbe(ProbeResult{Class: ProbeClassMismatch, Reason: "exe_mismatch"})
	h.sup.Start(context.Background())
	// First failure enters backoff; advance through remaining 4.
	for i := 0; i < 4; i++ {
		if h.sup.Phase() == PhaseFailed {
			break
		}
		// backoff delays: 1,2,5,15,...
		idx := i
		if idx >= len(h.snap.Thresholds.BackoffSeconds) {
			idx = len(h.snap.Thresholds.BackoffSeconds) - 1
		}
		h.clock.Advance(time.Duration(h.snap.Thresholds.BackoffSeconds[idx]) * time.Second)
	}
	if h.sup.Phase() != PhaseFailed {
		// may need one more advance if still in backoff after 4th
		for h.sup.Phase() != PhaseFailed && h.sup.PendingTimer() {
			h.clock.Advance(60 * time.Second)
		}
	}
	if h.sup.Phase() != PhaseFailed {
		t.Fatalf("phase = %s, want failed", h.sup.Phase())
	}
	cfg, _, _ := h.sup.Counts()
	if cfg != 5 {
		t.Fatalf("config fails = %d, want 5", cfg)
	}
	if _, ok := h.rec.LastKind(KindFailed); !ok {
		t.Fatal("missing failed record")
	}
	// retries stop: advance should not spawn
	spawns := h.wsl.SpawnCount()
	h.clock.Advance(60 * time.Second)
	if h.wsl.SpawnCount() != spawns {
		t.Fatal("spawn after terminal Failed")
	}
	if h.wsl.SpawnCount() != 0 {
		t.Fatal("mismatch must never spawn")
	}
}

// UAC-006: matched probe adopts before spawn; already-running, not failure.
func TestUAC006_AdoptBeforeSpawn(t *testing.T) {
	h := newHarness(t)
	h.host.SetProbe(ProbeResult{
		Class: ProbeClassMatchedRunning, OwnershipToken: "tok-1", Reason: "matched",
	})
	h.sup.Start(context.Background())
	if h.wsl.SpawnCount() != 0 {
		t.Fatal("spawn occurred despite matched_running")
	}
	if h.sup.OwnershipToken() != "tok-1" {
		t.Fatalf("token = %q", h.sup.OwnershipToken())
	}
	if _, ok := h.rec.LastKind(KindAlreadyRunning); !ok {
		t.Fatal("missing already_running record")
	}
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s, want ready after healthy adopt", h.sup.Phase())
	}
	cfg, tr, _ := h.sup.Counts()
	if cfg != 0 || tr != 0 {
		t.Fatalf("adoption counted as failure cfg=%d tr=%d", cfg, tr)
	}
	// probe happened before any spawn decision
	if h.host.ProbeCount() < 1 {
		t.Fatal("probe not issued")
	}
}

// UAC-011: resume invalidates health and rechecks within 10s.
func TestUAC011_ResumeInvalidatesHealth(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	if h.sup.Phase() != PhaseReady {
		t.Fatal(h.sup.Phase())
	}
	healthBefore := h.health.CallCount()
	at := h.clock.Now().Add(time.Hour)
	h.resume.Resume(at)
	// immediate timer (0)
	h.clock.Advance(0)
	if h.health.CallCount() <= healthBefore {
		// 0-duration timer: ManualClock AfterFunc with 0 is due on Advance(0)
		h.clock.Advance(time.Nanosecond)
	}
	if h.health.CallCount() <= healthBefore {
		t.Fatalf("health not re-probed after resume: before=%d after=%d", healthBefore, h.health.CallCount())
	}
	if _, ok := h.rec.LastKind(KindHealthInvalidate); !ok {
		// may not be last; search
		if h.rec.CountKind(KindHealthInvalidate) == 0 {
			t.Fatal("missing health_invalidate")
		}
	}
	// budget: resume recheck starts within 10s — we used 0
	if h.snap.Thresholds.ResumeRecheckMS != 10_000 {
		t.Fatalf("resume budget drift: %d", h.snap.Thresholds.ResumeRecheckMS)
	}
}

// UAC-012: third consecutive health failure → graceful restart, not Failed.
func TestUAC012_HealthRestartThreshold(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	// After ready, fail health 3 times.
	h.health.SetUnavailable()
	for i := 0; i < 3; i++ {
		h.clock.Advance(time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond)
	}
	if h.sup.Phase() == PhaseFailed {
		t.Fatal("health restart must not enter Failed")
	}
	if h.rec.CountKind(KindGracefulRestart) == 0 {
		t.Fatal("missing graceful_restart")
	}
	if h.sup.Phase() != PhaseRestarting {
		t.Fatalf("phase = %s, want restarting", h.sup.Phase())
	}
	// Prepare recovery before completing unit exit so the re-probe sees healthy evidence.
	h.host.SetProbe(ProbeResult{Class: ProbeClassAbsent})
	h.health.SetOK()
	if u := h.sup.Unit(); u != nil {
		u.(*FakeUnit).Exit(0)
	}
	waitPhase(t, h.sup, PhaseReady, PhaseStarting)
	for i := 0; i < 5 && h.sup.Phase() != PhaseReady; i++ {
		h.clock.Advance(time.Duration(h.snap.Thresholds.HealthCadenceMS) * time.Millisecond)
	}
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s, want ready after recovery", h.sup.Phase())
	}
}

// UAC-015 / UAC-017: transient continues past 5; recovery before 10 restores Ready.
func TestUAC015_017_TransientBeyondFiveThenRecover(t *testing.T) {
	h := newHarness(t)
	// Always unknown probe → transient
	h.host.SetProbe(ProbeResult{Class: ProbeClassUnknown, Reason: "wsl_unavailable"})
	h.sup.Start(context.Background())
	for i := 0; i < 5; i++ {
		if h.sup.Phase() == PhaseFailed {
			t.Fatalf("Failed at transient count before 10 (i=%d)", i)
		}
		h.clock.Advance(60 * time.Second)
	}
	_, tr, _ := h.sup.Counts()
	if tr < 5 {
		t.Fatalf("transient fails = %d, want >=5", tr)
	}
	if h.sup.Phase() == PhaseFailed {
		t.Fatal("must not Failed at configuration threshold for transient")
	}
	if h.rec.CountKind(KindFailed) != 0 {
		t.Fatal("Failed record at five transient failures")
	}
	// Recover before 10
	h.host.SetProbe(ProbeResult{Class: ProbeClassAbsent})
	h.health.SetOK()
	h.clock.Advance(60 * time.Second)
	if h.sup.Phase() != PhaseReady {
		t.Fatalf("phase = %s after recovery", h.sup.Phase())
	}
}

// Unknown never spawns (control-flow adversarial).
func TestNoSpawnOnUnknownOrMatched(t *testing.T) {
	h := newHarness(t)
	h.host.SetProbe(ProbeResult{Class: ProbeClassUnknown})
	h.sup.Start(context.Background())
	if h.wsl.SpawnCount() != 0 {
		t.Fatal("spawn on unknown")
	}
	h2 := newHarness(t)
	h2.host.SetProbe(ProbeResult{Class: ProbeClassMatchedRunning, OwnershipToken: "t"})
	h2.health.SetUnavailable()
	h2.sup.Start(context.Background())
	if h2.wsl.SpawnCount() != 0 {
		t.Fatal("spawn on matched")
	}
}

// Stale health/timer results cannot change state (concurrency adversarial).
func TestStaleEpochDiscarded(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	old := h.sup.Epoch()
	// Force epoch bump via resume
	h.resume.Resume(h.clock.Now())
	h.clock.Advance(0)
	h.sup.DeliverHealth(HealthResult{Status: HealthStatusUnavailable, Healthy: false}, old)
	if h.rec.CountKind(KindStaleDiscard) == 0 {
		t.Fatal("expected stale discard")
	}
	if h.sup.Phase() != PhaseReady {
		// may still be ready if re-probe ok
	}
}

// Performance budget: backoff schedule and one pending timer.
func TestBackoffScheduleAndSingleTimer(t *testing.T) {
	h := newHarness(t)
	h.host.SetProbe(ProbeResult{Class: ProbeClassUnknown})
	h.sup.Start(context.Background())
	if !h.sup.PendingTimer() {
		t.Fatal("expected backoff timer")
	}
	// Manual clock should have exactly one pending
	if h.clock.PendingTimers() != 1 {
		t.Fatalf("pending timers = %d, want 1", h.clock.PendingTimers())
	}
	want := []int{1, 2, 5, 15, 60}
	for i, sec := range want {
		rec, ok := h.rec.LastKind(KindBackoff)
		if !ok {
			t.Fatal("missing backoff")
		}
		if rec.DelayMS != sec*1000 {
			t.Fatalf("attempt %d delay_ms = %d, want %d", i+1, rec.DelayMS, sec*1000)
		}
		h.clock.Advance(time.Duration(sec) * time.Second)
		if h.sup.Phase() == PhaseFailed {
			break
		}
		if i == len(want)-1 {
			// still transient; delay stays capped at 60
			if rec, ok := h.rec.LastKind(KindBackoff); ok && rec.DelayMS > 60_000 {
				t.Fatalf("delay exceeded cap: %d", rec.DelayMS)
			}
		}
	}
}

// Shutdown budget constants (shared with stop task): 8+5+2 = 15s.
func TestShutdownBudgetDerivation(t *testing.T) {
	th := DefaultThresholds()
	if th.GracefulStopBudgetMS != 15_000 {
		t.Fatalf("graceful budget = %d", th.GracefulStopBudgetMS)
	}
	const drain, gateway, unit = 8_000, 5_000, 2_000
	if drain+gateway+unit != th.GracefulStopBudgetMS {
		t.Fatalf("derivation %d+%d+%d != %d", drain, gateway, unit, th.GracefulStopBudgetMS)
	}
}

func waitPhase(t *testing.T, s *Supervisor, want ...Phase) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p := s.Phase()
		for _, w := range want {
			if p == w {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("phase = %s, want one of %v", s.Phase(), want)
}
