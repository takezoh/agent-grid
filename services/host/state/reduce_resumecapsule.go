package state

import "log/slog"

type ResumeCapsuleFlagParams struct {
	SessionID string `json:"session_id"`
	Flagged   bool   `json:"flagged"`
}

// ResumeCapsulesReply is a placeholder marker: the runtime materializes
// the actual capsule list from State (same pattern as SessionsReply).
type ResumeCapsulesReply struct{}

func init() {
	RegisterEvent[struct{}](EventListResumeCapsules, reduceListResumeCapsules)
	RegisterEvent[ResumeCapsuleFlagParams](EventFlagResumeCapsule, reduceFlagResumeCapsule)
}

func reduceListResumeCapsules(s State, connID ConnID, reqID string, _ struct{}) (State, []Effect) {
	return s, []Effect{EffSendResponseSync{ConnID: connID, ReqID: reqID, Body: ResumeCapsulesReply{}}}
}

// reduceFlagResumeCapsule records the user's one-action "this capsule is
// wrong" judgment. The flag is cleared automatically by the next
// successful regeneration; the event-log line keeps a durable trace so
// flag counts survive regeneration (§1.4 capsule-trust criterion).
func reduceFlagResumeCapsule(s State, connID ConnID, reqID string, p ResumeCapsuleFlagParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if sid == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session_id required")}
	}
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	if sess.ResumeCapsule == nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session has no resume capsule")}
	}
	capsule := *sess.ResumeCapsule
	capsule.FlaggedWrong = p.Flagged
	if p.Flagged {
		capsule.FlaggedAt = s.Now
	}
	s.Sessions = cloneSessions(s.Sessions)
	sess.ResumeCapsule = &capsule
	s.Sessions[sid] = sess
	effs := []Effect{
		okResp(connID, reqID, nil),
		EffPersistSnapshot{},
		EffBroadcastSessionsChanged{},
	}
	if frame, ok := rootFrame(sess); ok {
		line := "resume-capsule flag cleared"
		if p.Flagged {
			line = "resume-capsule flagged wrong"
		}
		effs = append(effs, EffEventLogAppend{FrameID: frame.ID, Line: line})
	}
	return s, effs
}

// reduceCapsuleJobResult applies a finished capsule generation job to the
// owning session. Called from reduceJobResult before driver routing.
func reduceCapsuleJobResult(s State, meta JobMeta, result CapsuleJobResult, jobErr error) (State, []Effect) {
	s.CapsuleGenerating = cloneMap(s.CapsuleGenerating)
	delete(s.CapsuleGenerating, meta.SessionID)
	sess, ok := s.Sessions[meta.SessionID]
	if !ok {
		return s, nil
	}
	if jobErr != nil {
		if sess.ResumeCapsule == nil {
			slog.Debug("state: resume capsule generation failed, no prior capsule",
				"session", meta.SessionID, "err", jobErr)
			return s, nil
		}
		capsule := *sess.ResumeCapsule
		capsule.LastError = jobErr.Error()
		capsule.LastErrorAt = s.Now
		s.Sessions = cloneSessions(s.Sessions)
		sess.ResumeCapsule = &capsule
		s.Sessions[meta.SessionID] = sess
		return s, []Effect{EffPersistSnapshot{}, EffBroadcastSessionsChanged{}}
	}
	if result.SourceDigest == "" {
		// Runner not configured (empty capsule command) — treat as no-op.
		return s, nil
	}
	capsule := ResumeCapsule{
		Objective:    result.Objective,
		CurrentState: result.CurrentState,
		NextAction:   result.NextAction,
		Unresolved:   result.Unresolved,
		Changed:      result.Changed,
		Verified:     result.Verified,
		Remaining:    result.Remaining,
		Attention:    result.Attention,
		GeneratedAt:  s.Now,
		SourceDigest: result.SourceDigest,
		SourceTurns:  result.SourceTurns,
	}
	s.Sessions = cloneSessions(s.Sessions)
	sess.ResumeCapsule = &capsule
	s.Sessions[meta.SessionID] = sess
	return s, []Effect{EffPersistSnapshot{}, EffBroadcastSessionsChanged{}}
}
