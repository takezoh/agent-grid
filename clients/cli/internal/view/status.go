// Package view projects what a server published into what the CLI prints. It
// makes no network calls and holds no state: everything here is a pure function
// over decoded values.
package view

import "github.com/takezoh/agent-grid/clients/cli/internal/gateway"

// status.go is the CLI's copy of the shared status display matrix
// (adr-20260814-cli-status-display-matrix-pinning).
//
// protocol/session-status-display.json is canonical for this consumer, and
// TestStatusMatrixCopyMatchesRepositoryCanonical pins the copy to it. The copy
// exists because clients/cli is an independent module that cannot import a Go
// package to get the values, and reading the JSON at runtime would make a
// missing file a runtime failure of `ls`.
//
// What this must not become is a CLI dialect. Every word below appears in the
// canonical file; the CLI adds none of its own.

// State is one word from the shared vocabulary.
type State string

// The state words, all of them taken from the canonical matrix.
const (
	StateFailed     State = "failed"
	StateHumanInput State = "human_input"
	StateActive     State = "active"
	StateStopped    State = "stopped"
	StateIdle       State = "idle"
	// StateUnknown is the matrix's own entry for a status this build cannot
	// classify. It is in the canonical file's visuals but not in its
	// precedence, because it is what is left when precedence matches nothing.
	StateUnknown State = "unknown"
)

// Precedence is the order simultaneous conditions collapse in, taken from the
// canonical file's `precedence` array.
//
// This is deliberately not the favicon aggregation order used elsewhere in the
// repository: that order is for summarising a whole fleet into one browser tab
// icon and puts idle above stopped, which for a single session would report an
// ended session as idle.
var Precedence = []State{
	StateFailed,
	StateHumanInput,
	StateActive,
	StateStopped,
	StateIdle,
}

// Labels is the human-readable word for each state, taken from the canonical
// file's `visuals[*].label`.
var Labels = map[State]string{
	StateFailed:     "Failed",
	StateHumanInput: "Action required",
	StateActive:     "Working",
	StateStopped:    "Stopped",
	StateIdle:       "Idle",
	StateUnknown:    "Unknown",
}

// SignalKind is the transient attention signal the matrix defines. It rides
// alongside a state rather than replacing one: the matrix declares
// bell-with-action-required and bell-with-working both allowed.
const SignalKind = "bell"

// Collapse projects one published status onto a single state.
//
// The order is the matrix's precedence, and it is the same order every shipped
// client already applies, so the same session reads the same way in the CLI and
// in the browser. The one invariant the matrix states explicitly —
// spinner-and-action-required is forbidden — falls out of human_input
// outranking active rather than being enforced separately.
func Collapse(status gateway.SessionStatus) State {
	switch {
	case status.Failure == "failed":
		return StateFailed
	case status.HumanInput:
		return StateHumanInput
	case status.Lifecycle == "active":
		return StateActive
	case status.Lifecycle == "stopped":
		return StateStopped
	case status.Lifecycle == "idle":
		return StateIdle
	default:
		return StateUnknown
	}
}

// Label is the word printed for a state.
func Label(state State) string {
	if label, known := Labels[state]; known {
		return label
	}
	return Labels[StateUnknown]
}

// HasSignal reports whether a status carries the transient attention signal.
// It is independent of the collapsed state, which is why it is a separate
// question rather than a sixth state word.
func HasSignal(status gateway.SessionStatus) bool {
	return status.Signal != nil && status.Signal.Kind == SignalKind
}
