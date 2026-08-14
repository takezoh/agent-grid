package view

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
)

// Contract: contract-status-vocabulary
// (adr-20260814-cli-status-display-matrix-pinning).

// TestStatusMatrixCopyMatchesRepositoryCanonical is ver-matrix-copy-matches-canonical.
//
// A checked-in copy of a shared table drifts silently by default: the canonical
// file gains a state or renames a label, and this module keeps printing the old
// word forever with nothing to notice. Reading the canonical file in the test —
// rather than at runtime — keeps `ls` working when the file is absent while
// still failing the build when the copy is wrong.
func TestStatusMatrixCopyMatchesRepositoryCanonical(t *testing.T) {
	t.Parallel()

	canonical := readCanonicalMatrix(t)

	if len(canonical.Precedence) != len(Precedence) {
		t.Fatalf("the canonical matrix declares %d states in precedence, this copy has %d: %v vs %v",
			len(canonical.Precedence), len(Precedence), canonical.Precedence, Precedence)
	}
	for index, name := range canonical.Precedence {
		if got := string(Precedence[index]); got != name {
			t.Errorf("precedence position %d: canonical %q, copy %q", index, name, got)
		}
	}

	for name, visual := range canonical.Visuals {
		label, known := Labels[State(name)]
		if !known {
			t.Errorf("the canonical matrix defines state %q and this copy does not", name)
			continue
		}
		if label != visual.Label {
			t.Errorf("state %q label: canonical %q, copy %q", name, visual.Label, label)
		}
	}
	for name := range Labels {
		if _, defined := canonical.Visuals[string(name)]; !defined {
			t.Errorf("this copy carries state %q, which the canonical matrix does not define — "+
				"the CLI must not introduce a state word of its own", name)
		}
	}
	if canonical.Signal.Kind != SignalKind {
		t.Errorf("signal kind: canonical %q, copy %q", canonical.Signal.Kind, SignalKind)
	}
}

// TestStatusPrecedenceMatchesSharedMatrix is
// ver-precedence-collapses-like-existing-clients.
//
// Each case sets two conditions at once, so an implementation that checks them
// in a different order returns a different word. The stopped-versus-idle pair
// is the one that catches the favicon aggregation order, which reverses them:
// copying that order would report an ended session as idle.
func TestStatusPrecedenceMatchesSharedMatrix(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]struct {
		status gateway.SessionStatus
		state  State
		label  string
	}{
		"failed outranks everything": {
			status: gateway.SessionStatus{Lifecycle: "active", HumanInput: true, Failure: "failed"},
			state:  StateFailed, label: "Failed",
		},
		"action required outranks working": {
			status: gateway.SessionStatus{Lifecycle: "active", HumanInput: true, Failure: "none"},
			state:  StateHumanInput, label: "Action required",
		},
		"working when nothing outranks it": {
			status: gateway.SessionStatus{Lifecycle: "active", Failure: "none"},
			state:  StateActive, label: "Working",
		},
		"stopped is not idle": {
			status: gateway.SessionStatus{Lifecycle: "stopped", Failure: "none"},
			state:  StateStopped, label: "Stopped",
		},
		"idle": {
			status: gateway.SessionStatus{Lifecycle: "idle", Failure: "none"},
			state:  StateIdle, label: "Idle",
		},
		"an unclassifiable status is unknown, not idle": {
			status: gateway.SessionStatus{Lifecycle: "", Failure: ""},
			state:  StateUnknown, label: "Unknown",
		},
	} {
		if got := Collapse(expected.status); got != expected.state {
			t.Errorf("%s: collapsed to %q, want %q", name, got, expected.state)
		}
		if got := Label(expected.state); got != expected.label {
			t.Errorf("%s: label %q, want %q", name, got, expected.label)
		}
	}
}

// TestSpinnerAndActionRequiredCannotCoexist pins the one invariant the matrix
// states in words rather than in its precedence array.
func TestSpinnerAndActionRequiredCannotCoexist(t *testing.T) {
	t.Parallel()

	both := gateway.SessionStatus{Lifecycle: "active", HumanInput: true, Failure: "none"}
	if got := Collapse(both); got == StateActive {
		t.Error("a session awaiting a human decision must not read as working")
	}
}

// TestSignalIsIndependentOfTheCollapsedState pins the other two invariants:
// a bell may accompany action-required and may accompany working.
func TestSignalIsIndependentOfTheCollapsedState(t *testing.T) {
	t.Parallel()

	bell := &gateway.StatusSignal{Kind: "bell", Epoch: "e", Sequence: 1}
	working := gateway.SessionStatus{Lifecycle: "active", Failure: "none", Signal: bell}
	awaiting := gateway.SessionStatus{Lifecycle: "idle", HumanInput: true, Failure: "none", Signal: bell}

	if Collapse(working) != StateActive || !HasSignal(working) {
		t.Error("a bell must not change a working session's state")
	}
	if Collapse(awaiting) != StateHumanInput || !HasSignal(awaiting) {
		t.Error("a bell must not change an action-required session's state")
	}
	if HasSignal(gateway.SessionStatus{Lifecycle: "idle"}) {
		t.Error("a status with no signal reported one")
	}
}

type canonicalMatrix struct {
	Precedence []string `json:"precedence"`
	Visuals    map[string]struct {
		Label string `json:"label"`
	} `json:"visuals"`
	Signal struct {
		Kind string `json:"kind"`
	} `json:"signal"`
}

func readCanonicalMatrix(t *testing.T) canonicalMatrix {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "protocol", "session-status-display.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the canonical status matrix: %v", err)
	}
	var matrix canonicalMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("decode the canonical status matrix: %v", err)
	}
	return matrix
}

// repositoryRoot walks up to the directory holding protocol/. The module root
// is not enough: clients/cli is its own module and the shared contract lives
// two levels above it.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve package directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "protocol", "protocol-version.json")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("no repository root above the package directory")
		}
		directory = parent
	}
}
