package state

import "time"

// reduceQuestionRespond commits a free-text answer for a pending QuestionRequest.
func reduceQuestionRespond(s State, e EvCmdQuestionRespond) (State, []Effect) {
	if e.QuestionID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "question_id required")}
	}
	q, _, ok := FindQuestion(s, e.QuestionID)
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "question not found")}
	}
	if q.Status != QuestionPending {
		return s, []Effect{questionResolvedByOtherError(e.ConnID, e.ReqID, q)}
	}
	q.Status = QuestionResolved
	q.Answer = e.Answer
	q.ResolvingClientInstanceID = e.ClientInstanceID
	q.ResolutionReason = QuestionReasonClient
	s = putQuestion(s, q)
	effs := []Effect{
		okResp(e.ConnID, e.ReqID, nil),
		EffReplyHeldQuestion{
			FrameID:    q.FrameID,
			QuestionID: q.ID,
			Answer:     q.Answer,
		},
		questionResolvedBroadcast(q),
	}
	return s, effs
}

// reduceQuestionCancel cancels a pending QuestionRequest.
func reduceQuestionCancel(s State, e EvCmdQuestionCancel) (State, []Effect) {
	if e.QuestionID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "question_id required")}
	}
	q, _, ok := FindQuestion(s, e.QuestionID)
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "question not found")}
	}
	if q.Status != QuestionPending {
		return s, []Effect{questionResolvedByOtherError(e.ConnID, e.ReqID, q)}
	}
	q.Status = QuestionCancelled
	q.ResolvingClientInstanceID = e.ClientInstanceID
	q.ResolutionReason = QuestionReasonCancelled
	s = putQuestion(s, q)
	effs := []Effect{
		okResp(e.ConnID, e.ReqID, nil),
		EffReplyHeldQuestion{
			FrameID:    q.FrameID,
			QuestionID: q.ID,
			Error:      "connection-lost",
		},
		questionResolvedBroadcast(q),
	}
	return s, effs
}

// maybeCreateQuestionFromSubsystem materialises a durable QuestionRequest when
// a driver emits SubsystemQuestionRequested.
func maybeCreateQuestionFromSubsystem(s State, e EvSubsystem) (State, []Effect, bool) {
	if e.Kind != SubsystemQuestionRequested || e.Payload.Question == nil {
		return s, nil, false
	}
	qp := e.Payload.Question
	id := QuestionID(qp.ID)
	if id == "" {
		return s, nil, false
	}
	if _, _, exists := FindQuestion(s, id); exists {
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
	q := QuestionRequest{
		ID:        id,
		SessionID: sessID,
		FrameID:   e.FrameID,
		Prompt:    qp.Prompt,
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultQuestionTTL),
		Status:    QuestionPending,
	}
	s = putQuestion(s, q)
	return s, []Effect{questionRequestedBroadcast(q)}, true
}

// expirePendingQuestions transitions pending questions past expires_at.
// Free-text questions have no default answer; expiry cancels with empty answer.
func expirePendingQuestions(s State, now time.Time) (State, []Effect) {
	if s.PendingQuestions == nil || now.IsZero() {
		return s, nil
	}
	type hit struct {
		sess SessionID
		q    QuestionRequest
	}
	var hits []hit
	for sessID, m := range s.PendingQuestions {
		for _, q := range m {
			if q.Status == QuestionPending && !q.ExpiresAt.IsZero() && !now.Before(q.ExpiresAt) {
				hits = append(hits, hit{sess: sessID, q: q})
			}
		}
	}
	var effs []Effect
	for _, h := range hits {
		q := h.q
		q.Status = QuestionExpired
		q.ResolutionReason = QuestionReasonExpired
		s = putQuestion(s, q)
		effs = append(effs,
			EffReplyHeldQuestion{
				FrameID:    q.FrameID,
				QuestionID: q.ID,
				Error:      "connection-lost",
			},
			questionResolvedBroadcast(q),
		)
	}
	return s, effs
}

// cancelQuestionsForFrame reaps every pending QuestionRequest owned by frameID.
func cancelQuestionsForFrame(s State, frameID FrameID) (State, []Effect) {
	if s.PendingQuestions == nil || frameID == "" {
		return s, nil
	}
	type hit struct {
		sess SessionID
		q    QuestionRequest
	}
	var hits []hit
	for sessID, m := range s.PendingQuestions {
		for _, q := range m {
			if q.Status == QuestionPending && q.FrameID == frameID {
				hits = append(hits, hit{sess: sessID, q: q})
			}
		}
	}
	var effs []Effect
	for _, h := range hits {
		q := h.q
		q.Status = QuestionCancelled
		q.ResolutionReason = QuestionReasonCancelled
		s = deleteQuestion(s, h.sess, q.ID)
		effs = append(effs,
			EffReplyHeldQuestion{
				FrameID:    q.FrameID,
				QuestionID: q.ID,
				Error:      "connection-lost",
			},
			questionResolvedBroadcast(q),
		)
	}
	return s, effs
}

// cancelQuestionsForSession reaps every QuestionRequest for a session.
func cancelQuestionsForSession(s State, sessionID SessionID) (State, []Effect) {
	if s.PendingQuestions == nil || sessionID == "" {
		return s, nil
	}
	m := s.PendingQuestions[sessionID]
	if len(m) == 0 {
		return s, nil
	}
	var effs []Effect
	ids := make([]QuestionID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	for _, id := range ids {
		q := m[id]
		if q.Status == QuestionPending {
			q.Status = QuestionCancelled
			q.ResolutionReason = QuestionReasonCancelled
			effs = append(effs,
				EffReplyHeldQuestion{
					FrameID:    q.FrameID,
					QuestionID: q.ID,
					Error:      "connection-lost",
				},
				questionResolvedBroadcast(q),
			)
		}
		s = deleteQuestion(s, sessionID, id)
	}
	return s, effs
}

func questionResolvedByOtherError(connID ConnID, reqID string, q QuestionRequest) Effect {
	return EffSendError{
		ConnID:  connID,
		ReqID:   reqID,
		Code:    ErrCodeResolvedByOther,
		Message: "question already resolved by another client",
		Details: map[string]any{
			"answer":                       q.Answer,
			"resolving_client_instance_id": q.ResolvingClientInstanceID,
			"resolution_reason":            string(q.ResolutionReason),
			"status":                       string(q.Status),
		},
	}
}

func questionRequestedBroadcast(q QuestionRequest) Effect {
	return EffBroadcastEvent{
		Name:      EvtNameQuestionRequested,
		Payload:   q,
		FilterTag: EvtNameQuestionRequested,
	}
}

func questionResolvedBroadcast(q QuestionRequest) Effect {
	return EffBroadcastEvent{
		Name:      EvtNameQuestionResolved,
		Payload:   q,
		FilterTag: EvtNameQuestionResolved,
	}
}
