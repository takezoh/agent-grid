package runtime

import (
	"testing"

	"github.com/takezoh/agent-grid/host/proto"
)

func TestTerminalLifecycleActorAcceptedWaitingAreNonterminal(t *testing.T) {
	a := NewTerminalLifecycleActor()
	c := proto.PublicCorrelation{ClientInstanceID: "c", ConnectionGeneration: 1, ClientRevision: 1}
	if got := a.Admit(c); got.Status != proto.RevisionAccepted {
		t.Fatalf("admit status = %q", got.Status)
	}
	waiting, ok := a.MarkWaiting(c)
	if !ok || waiting.Status != proto.RevisionWaiting {
		t.Fatalf("waiting = %#v, ok=%v", waiting, ok)
	}
	if _, err := a.Complete(c, proto.RevisionWaiting, 0, 0, ""); err == nil {
		t.Fatal("nonterminal completion unexpectedly accepted")
	}
}

func TestTerminalLifecycleActorAssignsOneTerminalOutcomeAndFencesLateRevision(t *testing.T) {
	a := NewTerminalLifecycleActor()
	c1 := proto.PublicCorrelation{ClientInstanceID: "c", ConnectionGeneration: 1, ClientRevision: 1}
	c2 := proto.PublicCorrelation{ClientInstanceID: "c", ConnectionGeneration: 1, ClientRevision: 2}
	a.Admit(c1)
	if _, err := a.Complete(c1, proto.RevisionApplied, 3, 3, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Complete(c1, proto.RevisionRejected, 0, 0, "late"); err == nil {
		t.Fatal("second terminal outcome unexpectedly accepted")
	}
	if _, ok := a.MarkWaiting(c2); ok {
		t.Fatal("unadmitted newer revision was not fenced")
	}
	a.Admit(c2)
	if _, err := a.Complete(c1, proto.RevisionApplied, 3, 3, "late"); err == nil {
		t.Fatal("late older revision was not fenced after newer admission")
	}
}

func TestTerminalLifecycleActorBoundsRecords(t *testing.T) {
	a := NewTerminalLifecycleActor()
	for i := uint64(1); i <= lifecycleRecordLimit+4; i++ {
		c := proto.PublicCorrelation{ClientInstanceID: "c", ConnectionGeneration: 1, ClientRevision: i}
		a.Admit(c)
	}
	if got := len(a.current); got > lifecycleRecordLimit {
		t.Fatalf("records = %d, want <= %d", got, lifecycleRecordLimit)
	}
}
