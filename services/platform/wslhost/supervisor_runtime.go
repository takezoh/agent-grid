package wslhost

import (
	"context"
	"time"
)

func (s *Supervisor) scheduleHealthCadenceLocked() {
	if s.phase == PhaseFailed || s.phase == PhaseStopped {
		return
	}
	cadence := time.Duration(s.snap.Thresholds.HealthCadenceMS) * time.Millisecond
	if !s.startupSince.IsZero() {
		remaining := time.Duration(s.snap.Thresholds.StartupBudgetMS)*time.Millisecond -
			s.deps.Clock.Now().Sub(s.startupSince)
		if remaining < cadence {
			cadence = max(remaining, 0)
		}
	}
	s.scheduleLocked(cadence, func(epoch uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed || epoch != s.epoch {
			s.recordLocked(KindStaleDiscard, "", "stale_health_timer")
			return
		}
		if s.phase == PhaseFailed || s.phase == PhaseStopped || s.phase == PhaseBackoff {
			return
		}
		s.issueHealthLocked(context.Background())
	})
}

func (s *Supervisor) beginGracefulRestartLocked() {
	s.recordLocked(KindGracefulRestart, "", "health_restart_threshold")
	s.phase = PhaseRestarting
	s.healthFails = 0
	s.cancelTimerLocked()
	s.healthInFlight = false
	s.startupSince = s.deps.Clock.Now()
	if s.recoveryStop != nil && !s.recoveryActive {
		s.recoveryActive = true
		stop := s.recoveryStop
		go func() {
			out := stop(context.Background())
			s.onRecoveryStop(out)
		}()
		return
	}
	token := s.ownershipToken
	if token != "" {
		_, _ = s.deps.Host.Stop(context.Background(), StopRequest{
			DataDir: s.snap.DataDir, OwnershipToken: token,
		})
	}
	if s.unit != nil {
		// Exit watcher (same unitEpoch) will re-enter probe-before-spawn.
		return
	}
	// Adopted without a local unit handle: stop was issued; re-probe immediately.
	s.phase = PhaseStarting
	s.runProbeCycleLocked(context.Background())
}

func (s *Supervisor) onRecoveryStop(out StopOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoveryActive = false
	if s.closed || s.phase != PhaseRestarting {
		return
	}
	if out.Rejected {
		s.noteTransientLocked("recovery_stop_rejected: " + out.Reason)
		return
	}
	s.unit = nil
	s.ownershipToken = ""
	s.adopted = false
	s.spawnInFlight = false
	s.epoch++
	s.phase = PhaseStarting
	s.startupSince = s.deps.Clock.Now()
	s.runProbeCycleLocked(context.Background())
}

func (s *Supervisor) startupBudgetExpiredLocked() bool {
	if s.startupSince.IsZero() {
		s.startupSince = s.deps.Clock.Now()
		return false
	}
	budget := time.Duration(s.snap.Thresholds.StartupBudgetMS) * time.Millisecond
	return s.deps.Clock.Now().Sub(s.startupSince) >= budget
}

func (s *Supervisor) noteConfigLocked(reason string) {
	s.configFails++
	s.retryAttempt++
	if s.configFails >= s.snap.Thresholds.ConfigTerminalThreshold {
		s.enterFailedLocked(FailureConfig, reason)
		return
	}
	s.enterBackoffLocked(FailureConfig, reason)
}

func (s *Supervisor) noteTransientLocked(reason string) {
	s.transientFails++
	s.retryAttempt++
	if s.transientFails >= s.snap.Thresholds.TransientTerminalThreshold {
		s.enterFailedLocked(FailureTransient, reason)
		return
	}
	s.enterBackoffLocked(FailureTransient, reason)
}

func (s *Supervisor) enterFailedLocked(class FailureClass, reason string) {
	s.phase = PhaseFailed
	s.cancelTimerLocked()
	s.deps.Record.Record(LifecycleEvent{
		At: s.deps.Clock.Now(), Kind: KindFailed, Phase: s.phase,
		Generation: s.generation, Epoch: s.epoch, Class: string(class),
		Attempt: s.retryAttempt, Reason: reason, Setting: reason,
	})
}

func (s *Supervisor) enterBackoffLocked(class FailureClass, reason string) {
	s.phase = PhaseBackoff
	delay := s.backoffDelayLocked()
	s.deps.Record.Record(LifecycleEvent{
		At: s.deps.Clock.Now(), Kind: KindBackoff, Phase: s.phase,
		Generation: s.generation, Epoch: s.epoch, Class: string(class),
		Attempt: s.retryAttempt, DelayMS: int(delay / time.Millisecond), Reason: reason,
	})
	s.scheduleLocked(delay, func(epoch uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed || epoch != s.epoch {
			s.recordLocked(KindStaleDiscard, "", "stale_backoff_timer")
			return
		}
		if s.phase != PhaseBackoff {
			return
		}
		s.epoch++ // new cycle: discard late results from prior attempt
		s.healthInFlight = false
		s.spawnInFlight = false
		s.phase = PhaseStarting
		s.startupSince = s.deps.Clock.Now()
		s.runProbeCycleLocked(context.Background())
	})
}

func (s *Supervisor) backoffDelayLocked() time.Duration {
	secs := s.snap.Thresholds.BackoffSeconds
	if len(secs) == 0 {
		return time.Second
	}
	idx := s.retryAttempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(secs) {
		idx = len(secs) - 1
	}
	d := secs[idx]
	if d > 60 {
		d = 60
	}
	return time.Duration(d) * time.Second
}

// scheduleLocked ensures at most one pending timer for the current epoch.
func (s *Supervisor) scheduleLocked(d time.Duration, fn func(epoch uint64)) {
	s.cancelTimerLocked()
	epoch := s.epoch
	s.pendingTimer = s.deps.Timers.AfterFunc(d, func() {
		fn(epoch)
	})
}
