package proto

import "time"

// Wire names for approval / question IPC commands and events.
const (
	CmdNameApprovalRespond = "approval.respond"
	CmdNameApprovalCancel  = "approval.cancel"
	CmdNameQuestionRespond = "question.respond"
	CmdNameQuestionCancel  = "question.cancel"

	EvtNameApprovalRequested = "approval-requested"
	EvtNameApprovalResolved  = "approval-resolved"
	EvtNameQuestionRequested = "question-requested"
	EvtNameQuestionResolved  = "question-resolved"

	ErrResolvedByOther ErrCode = "resolved_by_other"
)

// CmdApprovalRespond commits a human decision for a pending ApprovalRequest.
type CmdApprovalRespond struct {
	SessionID        string `json:"session_id"`
	ApprovalID       string `json:"approval_id"`
	Decision         string `json:"decision"` // "accept" | "deny"
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

func (CmdApprovalRespond) isCommand()          {}
func (CmdApprovalRespond) CommandName() string { return CmdNameApprovalRespond }

// CmdApprovalCancel cancels a pending ApprovalRequest.
type CmdApprovalCancel struct {
	SessionID        string `json:"session_id"`
	ApprovalID       string `json:"approval_id"`
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

func (CmdApprovalCancel) isCommand()          {}
func (CmdApprovalCancel) CommandName() string { return CmdNameApprovalCancel }

// CmdQuestionRespond commits a free-text answer for a pending QuestionRequest.
type CmdQuestionRespond struct {
	SessionID        string `json:"session_id"`
	QuestionID       string `json:"question_id"`
	Answer           string `json:"answer"`
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

func (CmdQuestionRespond) isCommand()          {}
func (CmdQuestionRespond) CommandName() string { return CmdNameQuestionRespond }

// CmdQuestionCancel cancels a pending QuestionRequest.
type CmdQuestionCancel struct {
	SessionID        string `json:"session_id"`
	QuestionID       string `json:"question_id"`
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

func (CmdQuestionCancel) isCommand()          {}
func (CmdQuestionCancel) CommandName() string { return CmdNameQuestionCancel }

// ApprovalWire is the on-the-wire shape for ApprovalRequest payloads.
type ApprovalWire struct {
	ID                        string `json:"id"`
	SessionID                 string `json:"session_id"`
	FrameID                   string `json:"frame_id,omitempty"`
	Kind                      string `json:"kind,omitempty"`
	Command                   string `json:"command,omitempty"`
	Path                      string `json:"path,omitempty"`
	Reason                    string `json:"reason,omitempty"`
	CreatedAt                 string `json:"created_at,omitempty"`
	ExpiresAt                 string `json:"expires_at,omitempty"`
	Status                    string `json:"status"`
	DefaultDecision           string `json:"default_decision,omitempty"`
	Decision                  string `json:"decision,omitempty"`
	ResolvingClientInstanceID string `json:"resolving_client_instance_id,omitempty"`
	ResolutionReason          string `json:"resolution_reason,omitempty"`
}

// QuestionWire is the on-the-wire shape for QuestionRequest payloads.
type QuestionWire struct {
	ID                        string `json:"id"`
	SessionID                 string `json:"session_id"`
	FrameID                   string `json:"frame_id,omitempty"`
	Prompt                    string `json:"prompt,omitempty"`
	CreatedAt                 string `json:"created_at,omitempty"`
	ExpiresAt                 string `json:"expires_at,omitempty"`
	Status                    string `json:"status"`
	Answer                    string `json:"answer,omitempty"`
	ResolvingClientInstanceID string `json:"resolving_client_instance_id,omitempty"`
	ResolutionReason          string `json:"resolution_reason,omitempty"`
}

// EvtApprovalRequested notifies subscribers of a new pending approval.
type EvtApprovalRequested struct {
	Approval ApprovalWire `json:"approval"`
}

func (EvtApprovalRequested) isEvent()          {}
func (EvtApprovalRequested) EventName() string { return EvtNameApprovalRequested }

// EvtApprovalResolved notifies subscribers that an approval left pending.
type EvtApprovalResolved struct {
	Approval ApprovalWire `json:"approval"`
}

func (EvtApprovalResolved) isEvent()          {}
func (EvtApprovalResolved) EventName() string { return EvtNameApprovalResolved }

// EvtQuestionRequested notifies subscribers of a new pending free-text question.
type EvtQuestionRequested struct {
	Question QuestionWire `json:"question"`
}

func (EvtQuestionRequested) isEvent()          {}
func (EvtQuestionRequested) EventName() string { return EvtNameQuestionRequested }

// EvtQuestionResolved notifies subscribers that a question left pending.
type EvtQuestionResolved struct {
	Question QuestionWire `json:"question"`
}

func (EvtQuestionResolved) isEvent()          {}
func (EvtQuestionResolved) EventName() string { return EvtNameQuestionResolved }

// RespPendingHumanInput is the authoritative pending set returned on
// resubscribe / hello (FR-P0-08).
type RespPendingHumanInput struct {
	Approvals []ApprovalWire `json:"approvals,omitempty"`
	Questions []QuestionWire `json:"questions,omitempty"`
}

func (RespPendingHumanInput) isResponse() {}

// FormatRFC3339 formats t for the wire, or "" when zero.
func FormatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
