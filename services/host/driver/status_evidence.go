package driver

import (
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

type StatusEvidenceSource string

const (
	StatusSourceProcess  StatusEvidenceSource = "process"
	StatusSourceHook     StatusEvidenceSource = "hook"
	StatusSourceTerminal StatusEvidenceSource = "terminal"
	StatusSourceProgress StatusEvidenceSource = "progress"
)

type StatusObservation struct {
	Status     state.Status
	ObservedAt time.Time
	Valid      bool
}

// StatusEvidence preserves independent external observations. Status remains
// the single canonical projection consumed by every Agent Grid client.
type StatusEvidence struct {
	Process                StatusObservation
	Hook                   StatusObservation
	Terminal               StatusObservation
	Progress               StatusObservation
	TerminalActionRequired StatusObservation
}

func (c *CommonState) ObserveStatus(source StatusEvidenceSource, status state.Status, at time.Time) {
	if at.IsZero() {
		at = c.StatusChangedAt
	}
	observation := StatusObservation{Status: status, ObservedAt: at, Valid: true}
	switch source {
	case StatusSourceProcess:
		c.StatusEvidence.Process = observation
	case StatusSourceHook:
		c.StatusEvidence.Hook = observation
	case StatusSourceTerminal:
		c.StatusEvidence.Terminal = observation
		if status == state.StatusPending {
			c.StatusEvidence.TerminalActionRequired = observation
		} else {
			c.StatusEvidence.TerminalActionRequired.Valid = false
		}
	case StatusSourceProgress:
		c.StatusEvidence.Progress = observation
	}
	c.ResolveStatus(at)
}

func (c *CommonState) ResolveStatus(_ time.Time) {
	if c.StatusEvidence.Process.Valid &&
		c.StatusEvidence.Process.Status == state.StatusStopped {
		setResolvedStatus(c, state.StatusStopped, c.StatusEvidence.Process.ObservedAt)
		return
	}
	if c.StatusEvidence.Hook.Valid && c.StatusEvidence.Hook.Status == state.StatusPending {
		setResolvedStatus(c, state.StatusPending, c.StatusEvidence.Hook.ObservedAt)
		return
	}
	if c.StatusEvidence.TerminalActionRequired.Valid {
		setResolvedStatus(c, state.StatusPending, c.StatusEvidence.TerminalActionRequired.ObservedAt)
		return
	}
	best := StatusObservation{}
	for _, observation := range []StatusObservation{
		c.StatusEvidence.Hook,
		c.StatusEvidence.Terminal,
		c.StatusEvidence.Progress,
	} {
		if !observation.Valid {
			continue
		}
		if !best.Valid || observation.ObservedAt.After(best.ObservedAt) ||
			(observation.ObservedAt.Equal(best.ObservedAt) &&
				statusPriority(observation.Status) > statusPriority(best.Status)) {
			best = observation
		}
	}
	if !best.Valid {
		return
	}
	setResolvedStatus(c, best.Status, best.ObservedAt)
}

func statusPriority(status state.Status) int {
	switch status {
	case state.StatusStopped:
		return 5
	case state.StatusPending:
		return 4
	case state.StatusRunning:
		return 3
	case state.StatusWaiting:
		return 2
	case state.StatusIdle:
		return 1
	default:
		return 0
	}
}

func setResolvedStatus(c *CommonState, status state.Status, at time.Time) {
	if c.Status == status {
		return
	}
	c.Status = status
	if !at.IsZero() {
		c.StatusChangedAt = at
	}
}

func progressStatus(progressState int) state.Status {
	switch progressState {
	case 0:
		return state.StatusWaiting
	default:
		return state.StatusRunning
	}
}
