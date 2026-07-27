package wslhost

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Phase is the supervised lifecycle phase (read model; SCM Running is separate).
type Phase string

const (
	PhaseStopped        Phase = "stopped"
	PhaseStarting       Phase = "starting"
	PhaseReady          Phase = "ready"
	PhaseBackoff        Phase = "backoff"
	PhaseFailed         Phase = "failed"
	PhaseRestarting     Phase = "restarting"
	PhaseAlreadyRunning Phase = "already_running"
)

// FailureClass partitions retry policy.
type FailureClass string

const (
	FailureNone      FailureClass = ""
	FailureConfig    FailureClass = "configuration"
	FailureTransient FailureClass = "transient"
)

// EventKind names a structured lifecycle record (allowlist field).
type EventKind string

const (
	KindStart            EventKind = "start"
	KindProbe            EventKind = "probe"
	KindAdopt            EventKind = "adopt"
	KindSpawn            EventKind = "spawn"
	KindAlreadyRunning   EventKind = "already_running"
	KindReady            EventKind = "ready"
	KindHealth           EventKind = "health"
	KindChildExit        EventKind = "child_exit"
	KindBackoff          EventKind = "backoff"
	KindFailed           EventKind = "failed"
	KindResume           EventKind = "resume"
	KindHealthInvalidate EventKind = "health_invalidate"
	KindGracefulRestart  EventKind = "graceful_restart"
	KindStaleDiscard     EventKind = "stale_discard"
	KindServiceStop      EventKind = "service_stop"
	KindBudgetExpiry     EventKind = "budget_expiry"
	KindEscalation       EventKind = "escalation"
)

// LifecycleEvent is one structured lifecycle observation (no secrets).
// Named distinctly from the operator-facing Record in record.go.
type LifecycleEvent struct {
	At         time.Time `json:"at"`
	Kind       EventKind `json:"kind"`
	Phase      Phase     `json:"phase"`
	Generation string    `json:"generation"`
	Epoch      uint64    `json:"epoch"`
	Class      string    `json:"class,omitempty"`
	Attempt    int       `json:"attempt,omitempty"`
	DelayMS    int       `json:"delay_ms,omitempty"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Setting    string    `json:"setting,omitempty"`
}

// Recorder sinks structured lifecycle records.
type Recorder interface {
	Record(r LifecycleEvent)
}

// MemoryRecorder is the in-process fake Recorder.
type MemoryRecorder struct {
	mu      sync.Mutex
	Records []LifecycleEvent
}

// Record implements Recorder.
func (m *MemoryRecorder) Record(r LifecycleEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Records = append(m.Records, r)
}

// Snapshot copies records.
func (m *MemoryRecorder) Snapshot() []LifecycleEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LifecycleEvent(nil), m.Records...)
}

// LastKind returns the last record of kind, if any.
func (m *MemoryRecorder) LastKind(k EventKind) (LifecycleEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Records) - 1; i >= 0; i-- {
		if m.Records[i].Kind == k {
			return m.Records[i], true
		}
	}
	return LifecycleEvent{}, false
}

// CountKind returns how many records match kind.
func (m *MemoryRecorder) CountKind(k EventKind) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.Records {
		if r.Kind == k {
			n++
		}
	}
	return n
}

// Deps are injected external seams for the lifecycle controller.
type Deps struct {
	WSL    WSLRunner
	Host   HostControl
	Health HealthProber
	Clock  Clock
	Timers TimerScheduler
	Resume ResumeSource
	Record Recorder
}

// RecoveryStop executes the health-restart stop path without SCM child-kill
// escalation. It is wired by ServiceHost after the shared StopCoordinator exists.
type RecoveryStop func(context.Context) StopOutcome

// Supervisor owns generation-serialized probe/spawn/health/retry policy.
// A single mutex serializes SCM, exit, timer, health, and resume events.
type Supervisor struct {
	mu sync.Mutex

	snap Snapshot
	deps Deps

	phase      Phase
	epoch      uint64 // request generation for timer/health linearization
	generation string

	configFails    int
	transientFails int
	healthFails    int
	retryAttempt   int

	ownershipToken string
	unit           SupervisedUnit
	unitEpoch      uint64 // epoch at spawn/adopt for exit authority
	adopted        bool

	pendingTimer   Timer
	healthInFlight bool
	spawnInFlight  bool
	healthValid    bool
	startupSince   time.Time
	recoveryStop   RecoveryStop
	recoveryActive bool

	unsubscribeResume func()
	closed            bool
}

// NewSupervisor constructs a controller bound to snap. Call Start to run.
func NewSupervisor(snap Snapshot, deps Deps) (*Supervisor, error) {
	if err := ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	if deps.Clock == nil {
		deps.Clock = RealClock{}
	}
	if deps.Timers == nil {
		deps.Timers = RealScheduler{}
	}
	if deps.Record == nil {
		deps.Record = &MemoryRecorder{}
	}
	if deps.WSL == nil || deps.Host == nil || deps.Health == nil {
		return nil, fmt.Errorf("wslhost: WSL, Host, and Health deps are required")
	}
	return &Supervisor{
		snap:       snap,
		deps:       deps,
		phase:      PhaseStopped,
		generation: snap.Generation,
	}, nil
}

// Phase returns the current phase.
func (s *Supervisor) Phase() Phase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

// Epoch returns the current event generation epoch.
func (s *Supervisor) Epoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

// Counts returns config/transient/health failure counters.
func (s *Supervisor) Counts() (config, transient, health int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configFails, s.transientFails, s.healthFails
}

// RetryAttempt returns the current backoff attempt counter.
func (s *Supervisor) RetryAttempt() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryAttempt
}

// OwnershipToken returns the adopted/spawned stop authority token.
func (s *Supervisor) OwnershipToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ownershipToken
}

// Unit returns the current supervised unit, if any.
func (s *Supervisor) Unit() SupervisedUnit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unit
}

// PendingTimer reports whether a timer is scheduled.
func (s *Supervisor) PendingTimer() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingTimer != nil
}

// HealthInFlight reports an outstanding health request.
func (s *Supervisor) HealthInFlight() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.healthInFlight
}

// Start begins probe-before-spawn for the bound generation.
func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.epoch++
	s.cancelTimerLocked()
	s.healthInFlight = false
	s.spawnInFlight = false
	s.phase = PhaseStarting
	s.startupSince = s.deps.Clock.Now()
	s.recordLocked(KindStart, "", "lifecycle_start")
	if s.deps.Resume != nil && s.unsubscribeResume == nil {
		s.unsubscribeResume = s.deps.Resume.Subscribe(func(at time.Time) {
			s.OnResume(at)
		})
	}
	s.runProbeCycleLocked(ctx)
}

// SetRecoveryStop connects health restart escalation to the service stop
// coordinator. The callback is invoked outside the supervisor mutex.
func (s *Supervisor) SetRecoveryStop(stop RecoveryStop) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoveryStop = stop
}

// Close unsubscribes resume and cancels the pending timer.
func (s *Supervisor) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.cancelTimerLocked()
	if s.unsubscribeResume != nil {
		s.unsubscribeResume()
		s.unsubscribeResume = nil
	}
}

// OnResume invalidates health and schedules a fresh probe within resume_recheck_ms.
func (s *Supervisor) OnResume(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.phase == PhaseFailed || s.phase == PhaseStopped {
		return
	}
	s.healthValid = false
	s.healthFails = 0
	s.healthInFlight = false
	s.cancelTimerLocked()
	// Invalidate in-flight health by bumping epoch; unitEpoch stays so child exit still counts.
	s.epoch++
	s.recordLocked(KindResume, "", fmt.Sprintf("windows_resume_at_%s", at.UTC().Format(time.RFC3339Nano)))
	s.recordLocked(KindHealthInvalidate, "", "pre_resume_health_discarded")
	// Immediate recheck (0 delay) — within fixed resume_recheck_ms budget.
	s.scheduleLocked(0, func(epoch uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed || epoch != s.epoch {
			s.recordLocked(KindStaleDiscard, "", "stale_resume_timer")
			return
		}
		if s.phase == PhaseFailed || s.phase == PhaseStopped {
			return
		}
		if s.phase == PhaseReady || s.phase == PhaseAlreadyRunning {
			s.issueHealthLocked(context.Background())
			return
		}
		s.phase = PhaseStarting
		s.runProbeCycleLocked(context.Background())
	})
}

// OnChildExit delivers a unit exit. Stale unit epochs are discarded.
func (s *Supervisor) OnChildExit(exitCode int, unitEpoch uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if unitEpoch != s.unitEpoch {
		s.recordLocked(KindStaleDiscard, "", "stale_child_exit")
		return
	}
	code := exitCode
	r := LifecycleEvent{
		At: s.deps.Clock.Now(), Kind: KindChildExit, Phase: s.phase,
		Generation: s.generation, Epoch: s.epoch, ExitCode: &code, Reason: "unit_exited",
	}
	s.deps.Record.Record(r)
	s.unit = nil
	s.ownershipToken = ""
	s.adopted = false
	s.spawnInFlight = false
	if s.phase == PhaseFailed || s.phase == PhaseStopped {
		return
	}
	if s.phase == PhaseRestarting {
		if s.recoveryActive {
			return
		}
		s.phase = PhaseStarting
		s.startupSince = s.deps.Clock.Now()
		s.runProbeCycleLocked(context.Background())
		return
	}
	s.noteTransientLocked("child_exit")
}

// DeliverHealth applies a health result captured under epoch.
func (s *Supervisor) DeliverHealth(res HealthResult, epoch uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if epoch != s.epoch {
		s.recordLocked(KindStaleDiscard, "", "stale_health")
		return
	}
	s.healthInFlight = false
	s.onHealthLocked(res)
}

func (s *Supervisor) cancelTimerLocked() {
	if s.pendingTimer != nil {
		s.pendingTimer.Stop()
		s.pendingTimer = nil
	}
}

func (s *Supervisor) recordLocked(kind EventKind, class, reason string) {
	s.deps.Record.Record(LifecycleEvent{
		At:         s.deps.Clock.Now(),
		Kind:       kind,
		Phase:      s.phase,
		Generation: s.generation,
		Epoch:      s.epoch,
		Class:      class,
		Attempt:    s.retryAttempt,
		Reason:     reason,
	})
}

func (s *Supervisor) runProbeCycleLocked(ctx context.Context) {
	s.cancelTimerLocked()
	pr, err := s.deps.Host.Probe(ctx, ProbeRequest{
		DataDir:      s.snap.DataDir,
		ExpectedExe:  s.snap.ServerExecutable,
		ExpectedArgv: expectedServerArgv(s.snap),
		HealthURL:    s.snap.HealthURL,
		Timeout:      time.Duration(s.snap.Thresholds.HealthProbeTimeoutMS) * time.Millisecond,
	})
	if err != nil {
		s.recordLocked(KindProbe, ProbeClassUnknown, "probe_error: "+err.Error())
		s.noteTransientLocked("probe_error")
		return
	}
	s.recordLocked(KindProbe, pr.Class, pr.Reason)
	switch pr.Class {
	case ProbeClassMatchedRunning:
		s.ownershipToken = pr.OwnershipToken
		s.adopted = true
		s.unitEpoch = s.epoch
		s.phase = PhaseAlreadyRunning
		s.recordLocked(KindAdopt, pr.Class, "adopted_matched_instance")
		s.recordLocked(KindAlreadyRunning, pr.Class, "already_running")
		s.issueHealthLocked(ctx)
	case ProbeClassAbsent:
		s.spawnOnceLocked(ctx)
	case ProbeClassMismatch:
		s.noteConfigLocked("probe_mismatch")
	case ProbeClassUnknown:
		s.noteTransientLocked("probe_unknown")
	default:
		s.noteTransientLocked("probe_unclassified")
	}
}

func expectedServerArgv(s Snapshot) []string {
	spec := BuildLaunchSpec(s)
	return append([]string{s.ServerExecutable}, spec.ServerArgs...)
}

func (s *Supervisor) spawnOnceLocked(ctx context.Context) {
	if s.spawnInFlight {
		return
	}
	// Bound: at most one spawn attempt per probe cycle; spawnInFlight until exit/error.
	s.spawnInFlight = true
	spec := BuildLaunchSpec(s.snap)
	argv := BuildWSLExecArgv(spec)
	if err := ValidateLaunchArgv(argv, s.snap.Distribution, s.snap.LinuxUser); err != nil {
		s.spawnInFlight = false
		s.noteConfigLocked("invalid_launch: " + err.Error())
		return
	}
	unit, err := s.deps.WSL.StartForeground(ctx, spec)
	if err != nil {
		s.spawnInFlight = false
		s.noteTransientLocked("spawn_error: " + err.Error())
		return
	}
	s.unit = unit
	s.adopted = false
	s.unitEpoch = s.epoch
	s.recordLocked(KindSpawn, "", "direct_foreground_exec")
	ep := s.unitEpoch
	go func(u SupervisedUnit, unitEp uint64) {
		code, _ := u.Wait()
		s.OnChildExit(code, unitEp)
	}(unit, ep)
	s.issueHealthLocked(ctx)
}

func (s *Supervisor) issueHealthLocked(ctx context.Context) {
	if s.healthInFlight {
		return
	}
	s.healthInFlight = true
	epoch := s.epoch
	timeout := time.Duration(s.snap.Thresholds.HealthProbeTimeoutMS) * time.Millisecond
	url := s.snap.HealthURL
	health := s.deps.Health
	// Probe while holding the lock: fakes must not re-enter Supervisor.
	res, err := health.Probe(ctx, url, timeout)
	if err != nil {
		res = HealthResult{Reason: "health_error: " + err.Error(), Healthy: false}
	}
	if s.closed || epoch != s.epoch {
		s.healthInFlight = false
		s.recordLocked(KindStaleDiscard, "", "stale_health_result")
		return
	}
	s.healthInFlight = false
	if res.At.IsZero() {
		res.At = s.deps.Clock.Now()
	}
	s.onHealthLocked(res)
}

func (s *Supervisor) onHealthLocked(res HealthResult) {
	s.recordLocked(KindHealth, res.Status, res.Reason)
	if res.Healthy && res.Status == HealthStatusOK {
		s.healthValid = true
		s.healthFails = 0
		s.configFails = 0
		s.transientFails = 0
		s.retryAttempt = 0
		s.phase = PhaseReady
		s.startupSince = time.Time{}
		s.recordLocked(KindReady, HealthStatusOK, "status_ok")
		s.scheduleHealthCadenceLocked()
		return
	}
	switch s.phase {
	case PhaseReady:
		s.healthFails++
		if s.healthFails >= s.snap.Thresholds.HealthRestartThreshold {
			s.beginGracefulRestartLocked()
			return
		}
		s.scheduleHealthCadenceLocked()
	case PhaseStarting, PhaseAlreadyRunning, PhaseBackoff, PhaseRestarting:
		if s.startupBudgetExpiredLocked() {
			s.recordLocked(KindBudgetExpiry, string(FailureTransient), "startup_budget_expired")
			s.noteTransientLocked("startup_budget_expired")
			return
		}
		s.scheduleHealthCadenceLocked()
	default:
		s.scheduleHealthCadenceLocked()
	}
}
