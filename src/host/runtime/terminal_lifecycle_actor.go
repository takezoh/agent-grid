package runtime

import (
	"fmt"

	"github.com/takezoh/agent-grid/host/proto"
)

// TerminalLifecycleActor is the single reducer-side author of daemon
// RevisionOutcome values. It contains no I/O and is intended to be called by
// the runtime's lifecycle control lane, never by gateway or effect workers.
type TerminalLifecycleActor struct {
	current map[string]proto.RevisionOutcome
	latest  map[string]uint64
	order   []string
}

const lifecycleRecordLimit = 24

func NewTerminalLifecycleActor() *TerminalLifecycleActor {
	return &TerminalLifecycleActor{
		current: make(map[string]proto.RevisionOutcome),
		latest:  make(map[string]uint64),
	}
}

// Admit records the non-terminal accepted outcome for a new revision.
func (a *TerminalLifecycleActor) Admit(c proto.PublicCorrelation) proto.RevisionOutcome {
	base := correlationBaseKey(c)
	if latest, ok := a.latest[base]; ok && c.ClientRevision < latest {
		return proto.RevisionOutcome{Correlation: c, Status: proto.RevisionRejected, Reason: "stale_revision"}
	}
	if latest, ok := a.latest[base]; ok && latest == c.ClientRevision {
		if current, exists := a.current[correlationKey(c)]; exists {
			return current
		}
		return proto.RevisionOutcome{Correlation: c, Status: proto.RevisionRejected, Reason: "record_evicted"}
	}
	out := proto.RevisionOutcome{Correlation: c, Status: proto.RevisionAccepted}
	a.current[correlationKey(c)] = out
	a.latest[base] = c.ClientRevision
	a.order = append(a.order, correlationKey(c))
	if len(a.order) > lifecycleRecordLimit {
		oldest := a.order[0]
		a.order = a.order[1:]
		delete(a.current, oldest)
	}
	return out
}

// MarkWaiting records a non-terminal waiting outcome. It cannot replace a
// newer revision because the correlation tuple is checked before mutation.
func (a *TerminalLifecycleActor) MarkWaiting(c proto.PublicCorrelation) (proto.RevisionOutcome, bool) {
	if !a.isCurrent(c) {
		return proto.RevisionOutcome{}, false
	}
	if current := a.current[correlationKey(c)]; isTerminalOutcome(current.Status) {
		return current, false
	}
	out := proto.RevisionOutcome{Correlation: c, Status: proto.RevisionWaiting}
	a.current[correlationKey(c)] = out
	return out, true
}

// Lookup reports the currently recorded outcome without mutating actor state.
// Runtime uses it to make equal-revision replay idempotent without starting a
// second effect worker.
func (a *TerminalLifecycleActor) Lookup(c proto.PublicCorrelation) (proto.RevisionOutcome, bool) {
	out, ok := a.current[correlationKey(c)]
	return out, ok
}

// Complete assigns a terminal outcome exactly once for the admitted revision.
// Transport failures are deliberately not accepted here; they belong to the
// browser TransportObservation namespace.
func (a *TerminalLifecycleActor) Complete(
	c proto.PublicCorrelation,
	status proto.RevisionOutcomeKind,
	outputSeq, finalSeq uint64,
	reason string,
) (proto.RevisionOutcome, error) {
	if !isTerminalOutcome(status) {
		return proto.RevisionOutcome{}, fmt.Errorf("runtime: nonterminal outcome %q", status)
	}
	key := correlationKey(c)
	if latest, ok := a.latest[correlationBaseKey(c)]; !ok || latest != c.ClientRevision {
		return proto.RevisionOutcome{}, fmt.Errorf("runtime: stale revision")
	}
	previous, ok := a.current[key]
	if !ok {
		return proto.RevisionOutcome{}, fmt.Errorf("runtime: revision not admitted")
	}
	if isTerminalOutcome(previous.Status) {
		return proto.RevisionOutcome{}, fmt.Errorf("runtime: revision outcome already assigned")
	}
	out := proto.RevisionOutcome{
		Correlation: c,
		Status:      status,
		OutputSeq:   outputSeq,
		FinalSeq:    finalSeq,
		Reason:      reason,
	}
	a.current[key] = out
	return out, nil
}

func (a *TerminalLifecycleActor) isCurrent(c proto.PublicCorrelation) bool {
	latest, ok := a.latest[correlationBaseKey(c)]
	if !ok || latest != c.ClientRevision {
		return false
	}
	_, ok = a.current[correlationKey(c)]
	return ok
}

func correlationKey(c proto.PublicCorrelation) string {
	return fmt.Sprintf("%s/%d/%d", c.ClientInstanceID, c.ConnectionGeneration, c.ClientRevision)
}

func correlationBaseKey(c proto.PublicCorrelation) string {
	return fmt.Sprintf("%s/%d", c.ClientInstanceID, c.ConnectionGeneration)
}

func isTerminalOutcome(status proto.RevisionOutcomeKind) bool {
	switch status {
	case proto.RevisionApplied, proto.RevisionRejected, proto.RevisionSuperseded, proto.RevisionReleased, proto.RevisionDegraded, proto.RevisionNoOutput:
		return true
	default:
		return false
	}
}
