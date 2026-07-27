package view

import (
	"encoding/json"
	"time"
)

// Status is the user-facing operational state of an agent session. The
// set is closed: drivers must report one of these or no status at all.
type Status int

const (
	StatusRunning Status = iota
	StatusWaiting
	StatusIdle
	StatusStopped
	StatusPending // waiting for tool permission approval
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	case StatusIdle:
		return "idle"
	case StatusStopped:
		return "stopped"
	case StatusPending:
		return "pending"
	default:
		return "unknown"
	}
}

// SymbolKey returns a semantic key for the status icon, resolved to a
// glyph by the tui/glyphs package.
func (s Status) SymbolKey() string {
	switch s {
	case StatusRunning:
		return "status.running"
	case StatusWaiting:
		return "status.waiting"
	case StatusIdle:
		return "status.idle"
	case StatusStopped:
		return "status.stopped"
	case StatusPending:
		return "status.pending"
	default:
		return ""
	}
}

// ParseStatus is the inverse of Status.String().
func ParseStatus(name string) (Status, bool) {
	switch name {
	case "running":
		return StatusRunning, true
	case "waiting":
		return StatusWaiting, true
	case "idle":
		return StatusIdle, true
	case "stopped":
		return StatusStopped, true
	case "pending":
		return StatusPending, true
	default:
		return 0, false
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, ok := ParseStatus(name)
	if !ok {
		return &json.UnmarshalTypeError{Value: name}
	}
	*s = parsed
	return nil
}

// StatusInfo bundles a status with the time it was entered.
type StatusInfo struct {
	Status    Status
	ChangedAt time.Time
}

// SessionStatus is the composite, driver-neutral status shipped to clients.
// Driver Status remains an internal lifecycle observation; clients never see it.
type SessionStatus struct {
	Lifecycle  LifecycleStatus `json:"lifecycle"`
	HumanInput bool            `json:"human_input"`
	Failure    FailureStatus   `json:"failure"`
	ChangedAt  *time.Time      `json:"changed_at,omitempty"`
	Signal     *StatusSignal   `json:"signal,omitempty"`
}

type LifecycleStatus string

const (
	LifecycleActive  LifecycleStatus = "active"
	LifecycleIdle    LifecycleStatus = "idle"
	LifecycleStopped LifecycleStatus = "stopped"
)

type FailureStatus string

const (
	FailureNone   FailureStatus = "none"
	FailureFailed FailureStatus = "failed"
)

// StatusSignal identifies a transient attention signal. Clients consume each
// (epoch, sequence) at most once; it is deliberately independent of HumanInput.
type StatusSignal struct {
	Kind     string `json:"kind"`
	Epoch    string `json:"epoch"`
	Sequence uint64 `json:"sequence"`
}

// CompositeStatus projects one internal lifecycle observation and host-owned
// request/signal state into the only status shape exposed on the wire.
func CompositeStatus(
	status Status,
	changedAt time.Time,
	humanInput bool,
	failed bool,
	signal *StatusSignal,
) SessionStatus {
	lifecycle := LifecycleIdle
	failure := FailureNone
	if failed {
		failure = FailureFailed
	}
	switch status {
	case StatusRunning:
		lifecycle = LifecycleActive
	case StatusStopped:
		lifecycle = LifecycleStopped
	case StatusWaiting, StatusIdle, StatusPending:
		lifecycle = LifecycleIdle
	}
	var changed *time.Time
	if !changedAt.IsZero() {
		value := changedAt
		changed = &value
	}
	return SessionStatus{
		Lifecycle:  lifecycle,
		HumanInput: humanInput,
		Failure:    failure,
		ChangedAt:  changed,
		Signal:     signal,
	}
}
