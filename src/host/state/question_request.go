package state

import "time"

// QuestionID is a stable identifier for one outstanding QuestionRequest.
type QuestionID string

// QuestionStatus mirrors ApprovalStatus for free-text human-input requests.
type QuestionStatus string

const (
	QuestionPending   QuestionStatus = "pending"
	QuestionResolved  QuestionStatus = "resolved"
	QuestionExpired   QuestionStatus = "expired"
	QuestionCancelled QuestionStatus = "cancelled"
)

// QuestionResolutionReason explains why a QuestionRequest left pending.
type QuestionResolutionReason string

const (
	QuestionReasonClient    QuestionResolutionReason = "client"
	QuestionReasonExpired   QuestionResolutionReason = "expired"
	QuestionReasonCancelled QuestionResolutionReason = "cancelled"
)

// DefaultQuestionTTL is the Phase 0 expiry window for free-text questions.
const DefaultQuestionTTL = 30 * time.Second

// QuestionRequest is the durable domain object for one free-text human-input
// request from a driver (FR-P0-07). Phase 0/1 carries a single free-text
// answer field; structured per-question schemas are deferred.
type QuestionRequest struct {
	ID                        QuestionID
	SessionID                 SessionID
	FrameID                   FrameID
	Prompt                    string
	CreatedAt                 time.Time
	ExpiresAt                 time.Time
	Status                    QuestionStatus
	Answer                    string // free-text only
	ResolvingClientInstanceID string
	ResolutionReason          QuestionResolutionReason
}

// IsTerminal reports whether q has left the pending state.
func (q QuestionRequest) IsTerminal() bool {
	return q.Status != QuestionPending && q.Status != ""
}

// PendingQuestionsForSession returns pending QuestionRequests for sessionID.
func PendingQuestionsForSession(s State, sessionID SessionID) []QuestionRequest {
	if s.PendingQuestions == nil {
		return nil
	}
	m := s.PendingQuestions[sessionID]
	if len(m) == 0 {
		return nil
	}
	out := make([]QuestionRequest, 0, len(m))
	for _, q := range m {
		if q.Status == QuestionPending {
			out = append(out, q)
		}
	}
	return out
}

// FindQuestion looks up a QuestionRequest by ID across all sessions.
func FindQuestion(s State, id QuestionID) (QuestionRequest, SessionID, bool) {
	if s.PendingQuestions == nil || id == "" {
		return QuestionRequest{}, "", false
	}
	for sessID, m := range s.PendingQuestions {
		if q, ok := m[id]; ok {
			return q, sessID, true
		}
	}
	return QuestionRequest{}, "", false
}

func clonePendingQuestions(in map[SessionID]map[QuestionID]QuestionRequest) map[SessionID]map[QuestionID]QuestionRequest {
	if in == nil {
		return nil
	}
	out := make(map[SessionID]map[QuestionID]QuestionRequest, len(in))
	for sid, m := range in {
		if m == nil {
			out[sid] = nil
			continue
		}
		cm := make(map[QuestionID]QuestionRequest, len(m))
		for id, q := range m {
			cm[id] = q
		}
		out[sid] = cm
	}
	return out
}

func putQuestion(s State, q QuestionRequest) State {
	s.PendingQuestions = clonePendingQuestions(s.PendingQuestions)
	if s.PendingQuestions == nil {
		s.PendingQuestions = map[SessionID]map[QuestionID]QuestionRequest{}
	}
	m := s.PendingQuestions[q.SessionID]
	if m == nil {
		m = map[QuestionID]QuestionRequest{}
	} else {
		nm := make(map[QuestionID]QuestionRequest, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		m = nm
	}
	m[q.ID] = q
	s.PendingQuestions[q.SessionID] = m
	return s
}

func deleteQuestion(s State, sessionID SessionID, id QuestionID) State {
	if s.PendingQuestions == nil {
		return s
	}
	m := s.PendingQuestions[sessionID]
	if m == nil {
		return s
	}
	if _, ok := m[id]; !ok {
		return s
	}
	s.PendingQuestions = clonePendingQuestions(s.PendingQuestions)
	nm := make(map[QuestionID]QuestionRequest, len(m)-1)
	for k, v := range s.PendingQuestions[sessionID] {
		if k != id {
			nm[k] = v
		}
	}
	if len(nm) == 0 {
		delete(s.PendingQuestions, sessionID)
	} else {
		s.PendingQuestions[sessionID] = nm
	}
	return s
}
