package state

import "time"

// Approval / question event names used by EffBroadcastEvent and proto.
const (
	EvtNameApprovalRequested = "approval-requested"
	EvtNameApprovalResolved  = "approval-resolved"
	EvtNameQuestionRequested = "question-requested"
	EvtNameQuestionResolved  = "question-resolved"
)

// ErrCodeResolvedByOther is returned to the loser of a two-client race
// (FR-P0-04). Details carry the winning decision and client-instance-id.
const ErrCodeResolvedByOther = "resolved_by_other"

// reduceApprovalRespond commits a client decision for a pending ApprovalRequest
// (first-writer-wins under the single-writer Reduce loop).
func reduceApprovalRespond(s State, e EvCmdApprovalRespond) (State, []Effect) {
	if e.ApprovalID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "approval_id required")}
	}
	if e.Decision != ApprovalDecisionAccept && e.Decision != ApprovalDecisionDeny {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "decision must be accept or deny")}
	}
	r, _, ok := FindApproval(s, e.ApprovalID)
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "approval not found")}
	}
	if r.Status != ApprovalPending {
		return s, []Effect{resolvedByOtherError(e.ConnID, e.ReqID, r)}
	}
	r.Status = ApprovalResolved
	r.Decision = e.Decision
	r.ResolvingClientInstanceID = e.ClientInstanceID
	r.ResolutionReason = ApprovalReasonClient
	// Keep a terminal tombstone so a late CmdApprovalRespond receives
	// resolved-by-other with the winning decision (FR-P0-04). Pending
	// snapshots filter status==pending via PendingApprovalsForSession.
	s = putApproval(s, r)
	effs := []Effect{
		okResp(e.ConnID, e.ReqID, nil),
		EffReplyHeldApproval{
			FrameID:    r.FrameID,
			ApprovalID: r.ID,
			Decision:   r.Decision,
		},
		approvalResolvedBroadcast(r),
	}
	return s, effs
}

// reduceApprovalCancel cancels a pending ApprovalRequest owned by the caller.
func reduceApprovalCancel(s State, e EvCmdApprovalCancel) (State, []Effect) {
	if e.ApprovalID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "approval_id required")}
	}
	r, _, ok := FindApproval(s, e.ApprovalID)
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "approval not found")}
	}
	if r.Status != ApprovalPending {
		return s, []Effect{resolvedByOtherError(e.ConnID, e.ReqID, r)}
	}
	r.Status = ApprovalCancelled
	r.Decision = ApprovalDecisionDeny
	r.ResolvingClientInstanceID = e.ClientInstanceID
	r.ResolutionReason = ApprovalReasonCancelled
	s = putApproval(s, r)
	effs := []Effect{
		okResp(e.ConnID, e.ReqID, nil),
		EffReplyHeldApproval{
			FrameID:    r.FrameID,
			ApprovalID: r.ID,
			Decision:   ApprovalDecisionDeny,
			Error:      "connection-lost",
		},
		approvalResolvedBroadcast(r),
	}
	return s, effs
}

// maybeCreateApprovalFromSubsystem materialises a durable ApprovalRequest when
// a driver emits SubsystemApprovalRequested. Duplicate IDs are ignored
// (monotonic lifecycle). Auto-approve captures default_decision=accept and
// resolves immediately in the same Reduce cycle.
func maybeCreateApprovalFromSubsystem(s State, e EvSubsystem) (State, []Effect, bool) {
	if e.Kind != SubsystemApprovalRequested || e.Payload.Approval == nil {
		return s, nil, false
	}
	a := e.Payload.Approval
	id := ApprovalID(a.ID)
	if id == "" {
		return s, nil, false
	}
	if _, _, exists := FindApproval(s, id); exists {
		// Duplicate driver event: no second pending entry, no backward transition.
		return s, nil, true
	}
	sessID, _, _, ok := findFrame(s, e.FrameID)
	if !ok {
		return s, nil, false
	}
	now := e.Timestamp
	if now.IsZero() {
		now = s.Now
	}
	defaultDecision := ApprovalDecisionDeny
	if a.AutoApprove {
		defaultDecision = ApprovalDecisionAccept
	}
	r := ApprovalRequest{
		ID:              id,
		SessionID:       sessID,
		FrameID:         e.FrameID,
		Kind:            a.Kind,
		Command:         a.Command,
		Path:            a.Path,
		Reason:          a.Reason,
		CreatedAt:       now,
		ExpiresAt:       now.Add(DefaultApprovalTTL),
		Status:          ApprovalPending,
		DefaultDecision: defaultDecision,
	}
	s = putApproval(s, r)
	effs := []Effect{approvalRequestedBroadcast(r)}
	if a.AutoApprove {
		r.Status = ApprovalResolved
		r.Decision = ApprovalDecisionAccept
		r.ResolvingClientInstanceID = AutoResolvingClientInstanceID
		r.ResolutionReason = ApprovalReasonAuto
		s = putApproval(s, r)
		effs = append(effs,
			EffReplyHeldApproval{
				FrameID:    r.FrameID,
				ApprovalID: r.ID,
				Decision:   ApprovalDecisionAccept,
			},
			approvalResolvedBroadcast(r),
		)
	}
	return s, effs, true
}

// expirePendingApprovals transitions pending requests whose expires_at has
// elapsed, applying the decision captured at creation (not live policy).
func expirePendingApprovals(s State, now time.Time) (State, []Effect) {
	if s.PendingApprovals == nil || now.IsZero() {
		return s, nil
	}
	var effs []Effect
	// Collect first to avoid mutating while iterating nested maps.
	type hit struct {
		sess SessionID
		r    ApprovalRequest
	}
	var hits []hit
	for sessID, m := range s.PendingApprovals {
		for _, r := range m {
			if r.Status == ApprovalPending && !r.ExpiresAt.IsZero() && !now.Before(r.ExpiresAt) {
				hits = append(hits, hit{sess: sessID, r: r})
			}
		}
	}
	for _, h := range hits {
		r := h.r
		r.Status = ApprovalExpired
		r.Decision = r.DefaultDecision
		if r.Decision == "" {
			r.Decision = ApprovalDecisionDeny
		}
		r.ResolutionReason = ApprovalReasonExpired
		s = putApproval(s, r)
		effs = append(effs,
			EffReplyHeldApproval{
				FrameID:    r.FrameID,
				ApprovalID: r.ID,
				Decision:   r.Decision,
			},
			approvalResolvedBroadcast(r),
		)
	}
	return s, effs
}

// cancelApprovalsForFrame reaps every pending ApprovalRequest owned by frameID.
func cancelApprovalsForFrame(s State, frameID FrameID) (State, []Effect) {
	if s.PendingApprovals == nil || frameID == "" {
		return s, nil
	}
	var effs []Effect
	type hit struct {
		sess SessionID
		r    ApprovalRequest
	}
	var hits []hit
	for sessID, m := range s.PendingApprovals {
		for _, r := range m {
			if r.Status == ApprovalPending && r.FrameID == frameID {
				hits = append(hits, hit{sess: sessID, r: r})
			}
		}
	}
	for _, h := range hits {
		r := h.r
		r.Status = ApprovalCancelled
		r.Decision = ApprovalDecisionDeny
		r.ResolutionReason = ApprovalReasonCancelled
		// Teardown reaps: drain held request then drop the map entry.
		s = deleteApproval(s, h.sess, r.ID)
		effs = append(effs,
			EffReplyHeldApproval{
				FrameID:    r.FrameID,
				ApprovalID: r.ID,
				Decision:   ApprovalDecisionDeny,
				Error:      "connection-lost",
			},
			approvalResolvedBroadcast(r),
		)
	}
	return s, effs
}

// cancelApprovalsForSession reaps every ApprovalRequest for a session
// (pending → cancelled + drain; terminal tombstones dropped).
func cancelApprovalsForSession(s State, sessionID SessionID) (State, []Effect) {
	if s.PendingApprovals == nil || sessionID == "" {
		return s, nil
	}
	m := s.PendingApprovals[sessionID]
	if len(m) == 0 {
		return s, nil
	}
	var effs []Effect
	// Snapshot keys so we can delete while iterating.
	ids := make([]ApprovalID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	for _, id := range ids {
		r := m[id]
		if r.Status == ApprovalPending {
			r.Status = ApprovalCancelled
			r.Decision = ApprovalDecisionDeny
			r.ResolutionReason = ApprovalReasonCancelled
			effs = append(effs,
				EffReplyHeldApproval{
					FrameID:    r.FrameID,
					ApprovalID: r.ID,
					Decision:   ApprovalDecisionDeny,
					Error:      "connection-lost",
				},
				approvalResolvedBroadcast(r),
			)
		}
		s = deleteApproval(s, sessionID, id)
	}
	return s, effs
}

func resolvedByOtherError(connID ConnID, reqID string, r ApprovalRequest) Effect {
	return EffSendError{
		ConnID:  connID,
		ReqID:   reqID,
		Code:    ErrCodeResolvedByOther,
		Message: "approval already resolved by another client",
		Details: map[string]any{
			"decision":                     string(r.Decision),
			"resolving_client_instance_id": r.ResolvingClientInstanceID,
			"resolution_reason":            string(r.ResolutionReason),
			"status":                       string(r.Status),
		},
	}
}

func approvalRequestedBroadcast(r ApprovalRequest) Effect {
	return EffBroadcastEvent{
		Name:      EvtNameApprovalRequested,
		Payload:   r,
		FilterTag: EvtNameApprovalRequested,
	}
}

func approvalResolvedBroadcast(r ApprovalRequest) Effect {
	return EffBroadcastEvent{
		Name:      EvtNameApprovalResolved,
		Payload:   r,
		FilterTag: EvtNameApprovalResolved,
	}
}
