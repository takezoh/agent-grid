package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

func TestResumeCapsuleSnapshotRoundTrip(t *testing.T) {
	generated := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	flagged := generated.Add(30 * time.Minute)
	in := &state.ResumeCapsule{
		Objective:    "ship the login fix",
		CurrentState: "root cause identified",
		NextAction:   "write the regression test",
		Unresolved:   []string{"flaky CI"},
		Changed:      []string{"fixed token refresh"},
		Verified:     []string{"unit tests pass"},
		Remaining:    []string{"regression test"},
		Attention:    "review the fix",
		GeneratedAt:  generated,
		SourceDigest: "digest-1",
		SourceTurns:  4,
		FlaggedWrong: true,
		FlaggedAt:    flagged,
		LastError:    "timeout",
		LastErrorAt:  flagged,
	}

	got := restoreResumeCapsule(snapshotResumeCapsule(in))
	if got == nil {
		t.Fatal("round trip must not drop the capsule")
	}
	if got.Objective != in.Objective || got.CurrentState != in.CurrentState ||
		got.NextAction != in.NextAction || got.SourceDigest != in.SourceDigest ||
		got.SourceTurns != in.SourceTurns || got.LastError != in.LastError {
		t.Errorf("round trip mismatch: got %+v", got)
	}
	if !got.GeneratedAt.Equal(generated) || !got.FlaggedAt.Equal(flagged) || !got.LastErrorAt.Equal(flagged) {
		t.Errorf("timestamps mismatch: %+v", got)
	}
	if len(got.Changed) != 1 || len(got.Verified) != 1 || len(got.Remaining) != 1 ||
		got.Attention != "review the fix" {
		t.Errorf("completion fields mismatch: %+v", got)
	}
	if !got.FlaggedWrong {
		t.Error("flag must survive restart")
	}
}

func TestResumeCapsuleSnapshotNilAndInvalid(t *testing.T) {
	if snapshotResumeCapsule(nil) != nil {
		t.Error("nil capsule → nil snapshot")
	}
	if restoreResumeCapsule(nil) != nil {
		t.Error("nil snapshot → nil capsule")
	}
	// A capsule without a valid GeneratedAt cannot honour the staleness
	// contract and must be dropped on restore.
	if restoreResumeCapsule(&ResumeCapsuleSnapshot{Objective: "x"}) != nil {
		t.Error("missing GeneratedAt must drop the capsule")
	}
}
