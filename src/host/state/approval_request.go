package state

import "time"

// ApprovalID is a stable identifier for one outstanding ApprovalRequest.
type ApprovalID string

// ApprovalStatus is the lifecycle status of an ApprovalRequest.
// Transitions are forward-only: pending → resolved | expired | cancelled.
type ApprovalStatus string

const (
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalResolved  ApprovalStatus = "resolved"
	ApprovalExpired   ApprovalStatus = "expired"
	ApprovalCancelled ApprovalStatus = "cancelled"
)

// ApprovalDecision is the accept/deny outcome recorded on an ApprovalRequest.
type ApprovalDecision string

const (
	ApprovalDecisionAccept ApprovalDecision = "accept"
	ApprovalDecisionDeny   ApprovalDecision = "deny"
)

// ApprovalResolutionReason explains why a request left pending.
type ApprovalResolutionReason string

const (
	ApprovalReasonClient    ApprovalResolutionReason = "client"
	ApprovalReasonExpired   ApprovalResolutionReason = "expired"
	ApprovalReasonCancelled ApprovalResolutionReason = "cancelled"
	ApprovalReasonAuto      ApprovalResolutionReason = "auto"
)

// DefaultApprovalTTL is the Phase 0 expiry window captured at creation.
// expires_at is immutable once set (FR-P0-06 / NFR-02).
const DefaultApprovalTTL = 30 * time.Second

// AutoResolvingClientInstanceID is recorded as resolving_client_instance_id
// when the driver-side auto-approve policy resolves a request without a
// human client-instance-id.
const AutoResolvingClientInstanceID = "auto"

// ApprovalRequest is the durable, single-writer domain object for one
// outstanding driver approval (FR-P0-01). It lives in State.PendingApprovals
// while status=pending and is removed on terminal transition after effects
// are emitted (reap-on-terminal keeps maps free of resolved entries).
type ApprovalRequest struct {
	ID                        ApprovalID
	SessionID                 SessionID
	FrameID                   FrameID
	Kind                      string // "command" | "file_change"
	Command                   string
	Path                      string
	Reason                    string
	CreatedAt                 time.Time
	ExpiresAt                 time.Time
	Status                    ApprovalStatus
	// DefaultDecision is captured at creation from the driver's per-session
	// policy (deny by default for destructive kinds). Used at expiry only —
	// never re-read from the live driver policy (TOCTOU-free, NFR-02).
	DefaultDecision           ApprovalDecision
	Decision                  ApprovalDecision
	ResolvingClientInstanceID string
	ResolutionReason          ApprovalResolutionReason
}

// IsTerminal reports whether r has left the pending state.
func (r ApprovalRequest) IsTerminal() bool {
	return r.Status != ApprovalPending && r.Status != ""
}

// PendingApprovalsForSession returns a shallow copy of pending (non-terminal)
// ApprovalRequests for sessionID. Safe for snapshot/resubscribe payloads.
func PendingApprovalsForSession(s State, sessionID SessionID) []ApprovalRequest {
	if s.PendingApprovals == nil {
		return nil
	}
	m := s.PendingApprovals[sessionID]
	if len(m) == 0 {
		return nil
	}
	out := make([]ApprovalRequest, 0, len(m))
	for _, r := range m {
		if r.Status == ApprovalPending {
			out = append(out, r)
		}
	}
	return out
}

// FindApproval looks up an ApprovalRequest by ID across all sessions.
func FindApproval(s State, id ApprovalID) (ApprovalRequest, SessionID, bool) {
	if s.PendingApprovals == nil || id == "" {
		return ApprovalRequest{}, "", false
	}
	for sessID, m := range s.PendingApprovals {
		if r, ok := m[id]; ok {
			return r, sessID, true
		}
	}
	return ApprovalRequest{}, "", false
}

func clonePendingApprovals(in map[SessionID]map[ApprovalID]ApprovalRequest) map[SessionID]map[ApprovalID]ApprovalRequest {
	if in == nil {
		return nil
	}
	out := make(map[SessionID]map[ApprovalID]ApprovalRequest, len(in))
	for sid, m := range in {
		if m == nil {
			out[sid] = nil
			continue
		}
		cm := make(map[ApprovalID]ApprovalRequest, len(m))
		for id, r := range m {
			cm[id] = r
		}
		out[sid] = cm
	}
	return out
}

func putApproval(s State, r ApprovalRequest) State {
	s.PendingApprovals = clonePendingApprovals(s.PendingApprovals)
	if s.PendingApprovals == nil {
		s.PendingApprovals = map[SessionID]map[ApprovalID]ApprovalRequest{}
	}
	m := s.PendingApprovals[r.SessionID]
	if m == nil {
		m = map[ApprovalID]ApprovalRequest{}
	} else {
		nm := make(map[ApprovalID]ApprovalRequest, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		m = nm
	}
	m[r.ID] = r
	s.PendingApprovals[r.SessionID] = m
	return s
}

func deleteApproval(s State, sessionID SessionID, id ApprovalID) State {
	if s.PendingApprovals == nil {
		return s
	}
	m := s.PendingApprovals[sessionID]
	if m == nil {
		return s
	}
	if _, ok := m[id]; !ok {
		return s
	}
	s.PendingApprovals = clonePendingApprovals(s.PendingApprovals)
	nm := make(map[ApprovalID]ApprovalRequest, len(m)-1)
	for k, v := range s.PendingApprovals[sessionID] {
		if k != id {
			nm[k] = v
		}
	}
	if len(nm) == 0 {
		delete(s.PendingApprovals, sessionID)
	} else {
		s.PendingApprovals[sessionID] = nm
	}
	return s
}
