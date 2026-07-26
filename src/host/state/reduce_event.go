package state

import "log/slog"

func reduceEvent(s State, e EvEvent) (State, []Effect) {
	fn := eventHandlers[e.Event]
	return fn(s, e)
}

func reduceDriverHook(s State, e EvDriverEvent) (State, []Effect) {
	if e.SenderID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "driver event requires sender_id: "+e.Event)}
	}
	if _, _, _, ok := findFrame(s, e.SenderID); !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "unknown session")}
	}

	next, rawEffs, ok := stepDriver(s, e.SenderID, DEvHook{
		Event:          e.Event,
		Timestamp:      e.Timestamp,
		RoostSessionID: string(e.SenderID),
		Payload:        e.Payload,
	})
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInternal, "no driver for session")}
	}
	s = next

	// Resolve EffPushDriver effects emitted by the driver.
	s, effs := resolvePushDriverEffects(s, rawEffs)

	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	if e.ConnID != 0 {
		effs = append(effs, okResp(e.ConnID, e.ReqID, nil))
	}
	return s, effs
}

func reduceSubsystem(s State, e EvSubsystem) (State, []Effect) {
	if e.FrameID == "" {
		slog.Debug("state: subsystem event rejected", "reason", "missing_frame_id", "source", e.Source, "kind", e.Kind)
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "subsystem event requires frame_id")}
	}
	sessID, sess, frameIdx, ok := findFrame(s, e.FrameID)
	if !ok {
		slog.Debug("state: subsystem event rejected", "reason", "unknown_frame", "frame", e.FrameID, "source", e.Source, "kind", e.Kind)
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "unknown session")}
	}
	frame := sess.Frames[frameIdx]
	if targetID := TargetID(e.Payload.TargetID); targetID != "" && targetID != frame.TargetID {
		slog.Debug("state: subsystem target updated",
			"frame", e.FrameID,
			"from", frame.TargetID,
			"to", targetID,
			"source", e.Source,
			"kind", e.Kind)
		sess.Frames = append([]SessionFrame(nil), sess.Frames...)
		sess.Frames[frameIdx].TargetID = targetID
		s.Sessions = cloneSessions(s.Sessions)
		s.Sessions[sessID] = sess
	}
	slog.Debug("state: subsystem event routed",
		"frame", e.FrameID,
		"target", sess.Frames[frameIdx].TargetID,
		"source", e.Source,
		"kind", e.Kind)

	next, rawEffs, ok := stepDriver(s, e.FrameID, DEvSubsystem{
		Source:    e.Source,
		Kind:      e.Kind,
		Timestamp: e.Timestamp,
		Payload:   e.Payload,
	})
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInternal, "no driver for session")}
	}
	s = next

	s, effs := resolvePushDriverEffects(s, rawEffs)

	// Materialise durable ApprovalRequest / QuestionRequest in the same
	// Reduce cycle as the driver subsystem event (FR-P0-02).
	if next, aEffs, handled := maybeCreateApprovalFromSubsystem(s, e); handled {
		s = next
		effs = append(effs, aEffs...)
	}
	if next, qEffs, handled := maybeCreateQuestionFromSubsystem(s, e); handled {
		s = next
		effs = append(effs, qEffs...)
	}
	if next, resolvedEffs, handled := maybeResolveHumanInputFromSubsystem(s, e); handled {
		s = next
		effs = append(effs, resolvedEffs...)
	}
	if e.Kind == SubsystemTurnCompleted ||
		e.Kind == SubsystemApprovalRequested ||
		e.Kind == SubsystemQuestionRequested {
		s.StatusSignals = cloneStatusSignals(s.StatusSignals)
		s.StatusSignals[sessID]++
	}

	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	if e.ConnID != 0 {
		effs = append(effs, okResp(e.ConnID, e.ReqID, nil))
	}
	return s, effs
}

func maybeResolveHumanInputFromSubsystem(s State, e EvSubsystem) (State, []Effect, bool) {
	switch e.Kind {
	case SubsystemApprovalResolved:
		if e.Payload.Approval == nil {
			return s, nil, false
		}
		request, _, ok := FindApproval(s, ApprovalID(e.Payload.Approval.ID))
		if !ok || request.Status != ApprovalPending {
			return s, nil, ok
		}
		request.Status = ApprovalResolved
		request.ResolutionReason = ApprovalReasonServer
		s = putApproval(s, request)
		return s, []Effect{approvalResolvedBroadcast(request)}, true
	case SubsystemQuestionResolved:
		if e.Payload.Question == nil {
			return s, nil, false
		}
		request, _, ok := FindQuestion(s, QuestionID(e.Payload.Question.ID))
		if !ok || request.Status != QuestionPending {
			return s, nil, ok
		}
		request.Status = QuestionResolved
		request.ResolutionReason = QuestionReasonServer
		s = putQuestion(s, request)
		return s, []Effect{questionResolvedBroadcast(request)}, true
	default:
		return s, nil, false
	}
}

func cloneStatusSignals(in map[SessionID]uint64) map[SessionID]uint64 {
	out := make(map[SessionID]uint64, len(in)+1)
	for id, sequence := range in {
		out[id] = sequence
	}
	return out
}
