package proto

const CmdNameLifecycleDesired = "lifecycle.desired"

// CmdLifecycleDesired is the complete desired publication sent by a current
// lifecycle-v2 browser. It replaces imperative subscribe/unsubscribe intent at
// the protocol boundary; legacy frames remain available only to existing
// non-v2 callers.
type CmdLifecycleDesired struct {
	Correlation PublicCorrelation `json:"correlation"`
	SessionID   string            `json:"session_id"`
	// SubscriberID is a server-only relay capability carried on the daemon
	// IPC hop. It never crosses the browser lifecycle wire or appears in an
	// authoritative outcome.
	SubscriberID string `json:"subscriber_id,omitempty"`
	Cols         uint16 `json:"cols"`
	Rows         uint16 `json:"rows"`
	Desired      bool   `json:"desired"`
}

func (CmdLifecycleDesired) isCommand()          {}
func (CmdLifecycleDesired) CommandName() string { return CmdNameLifecycleDesired }

// Lifecycle protocol v2 types. Browser-visible correlation deliberately
// excludes the daemon's private owner key, ticket and IPC generation.
type PublicCorrelation struct {
	ClientInstanceID     string `json:"clientInstanceID"`
	ConnectionGeneration uint64 `json:"connectionGeneration"`
	ClientRevision       uint64 `json:"clientRevision"`
}

type RevisionOutcomeKind string

const (
	RevisionAccepted   RevisionOutcomeKind = "accepted"
	RevisionWaiting    RevisionOutcomeKind = "waiting"
	RevisionApplied    RevisionOutcomeKind = "applied"
	RevisionRejected   RevisionOutcomeKind = "rejected"
	RevisionSuperseded RevisionOutcomeKind = "superseded"
	RevisionReleased   RevisionOutcomeKind = "released"
	RevisionDegraded   RevisionOutcomeKind = "degraded"
	RevisionNoOutput   RevisionOutcomeKind = "no_output"
)

// RevisionOutcome is daemon authority for a daemon-admitted revision.
// accepted and waiting are intentionally non-terminal.
type RevisionOutcome struct {
	Correlation PublicCorrelation   `json:"correlation"`
	Status      RevisionOutcomeKind `json:"status"`
	OutputSeq   uint64              `json:"output_seq,omitempty"`
	FinalSeq    uint64              `json:"final_sequence,omitempty"`
	Reason      string              `json:"reason,omitempty"`
}

type LifecycleOutput struct {
	Correlation PublicCorrelation `json:"correlation"`
	Sequence    uint64            `json:"sequence"`
	Final       bool              `json:"final,omitempty"`
	Digest      string            `json:"digest,omitempty"`
}

type LifecycleDiagnostic struct {
	Correlation PublicCorrelation `json:"correlation"`
	Watermark   uint64            `json:"watermark"`
	DropCount   uint64            `json:"drop_count,omitempty"`
	Unknown     bool              `json:"unknown,omitempty"`
}

type DiagnosticDisposition string

const (
	DiagnosticDeterminate DiagnosticDisposition = "determinate"
	DiagnosticNoOutput    DiagnosticDisposition = "no_output"
	DiagnosticUnknown     DiagnosticDisposition = "unknown"
	DiagnosticDrop        DiagnosticDisposition = "drop"
)

type EvtLifecycleOutcome struct{ RevisionOutcome }

func (EvtLifecycleOutcome) isEvent()          {}
func (EvtLifecycleOutcome) EventName() string { return EvtNameLifecycleOutcome }

type EvtLifecycleOutput struct{ LifecycleOutput }

func (EvtLifecycleOutput) isEvent()          {}
func (EvtLifecycleOutput) EventName() string { return EvtNameLifecycleOutput }

type EvtLifecycleDiagnostic struct{ LifecycleDiagnostic }

func (EvtLifecycleDiagnostic) isEvent()          {}
func (EvtLifecycleDiagnostic) EventName() string { return EvtNameLifecycleDiagnostic }
