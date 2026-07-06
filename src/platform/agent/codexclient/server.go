package codexclient

import (
	"sync"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-grid/platform/agent/codexschema/v2"
)

// Server wraps a Conn for the server role (e.g. claude-app-server shim).
// It provides convenience emit helpers for common Codex protocol events.
type Server struct {
	conn *Conn

	mu            sync.Mutex
	turnStartedAt map[string]time.Time
	now           func() time.Time
}

// NewServer wraps conn in a Server.
func NewServer(conn *Conn) *Server {
	return &Server{
		conn:          conn,
		turnStartedAt: map[string]time.Time{},
		now:           time.Now,
	}
}

// Conn returns the underlying Conn, e.g. to call Reply/ReplyError directly.
func (s *Server) Conn() *Conn { return s.conn }

// EmitNotification sends an arbitrary server-initiated notification.
func (s *Server) EmitNotification(method string, params any) error {
	return s.conn.Notify(method, params)
}

// EmitThreadStarted emits `thread/started` with the given thread metadata.
func (s *Server) EmitThreadStarted(threadID, cwd string) error {
	return s.EmitThreadStartedWithPath(threadID, cwd, "")
}

// EmitThreadStartedWithPath emits `thread/started` with cwd and, when known,
// the optional Codex rollout path.
func (s *Server) EmitThreadStartedWithPath(threadID, cwd, path string) error {
	thread := map[string]any{"id": threadID, "cwd": cwd}
	if path != "" {
		thread["path"] = path
	}
	return s.conn.Notify(codexschema.MethodThreadStarted, map[string]any{
		"thread": thread,
	})
}

// EmitTurnStarted emits `turn/started`.
func (s *Server) EmitTurnStarted(threadID, turnID string) error {
	startedAt := s.now()
	s.mu.Lock()
	s.turnStartedAt[turnKey(threadID, turnID)] = startedAt
	s.mu.Unlock()
	return s.conn.Notify(codexschema.MethodTurnStarted, TurnStartedParamsAt(threadID, turnID, startedAt))
}

// EmitTurnCompleted emits `turn/completed`.
func (s *Server) EmitTurnCompleted(threadID, turnID string) error {
	completedAt := s.now()
	s.mu.Lock()
	startedAt := s.turnStartedAt[turnKey(threadID, turnID)]
	delete(s.turnStartedAt, turnKey(threadID, turnID))
	s.mu.Unlock()
	return s.conn.Notify(codexschema.MethodTurnCompleted, TurnCompletedParamsAt(threadID, turnID, startedAt, completedAt))
}

// EmitTurnCompletedFinalAnswer emits `turn/completed` with a terminal assistant answer.
func (s *Server) EmitTurnCompletedFinalAnswer(threadID, turnID, itemID, text string) error {
	completedAt := s.now()
	s.mu.Lock()
	startedAt := s.turnStartedAt[turnKey(threadID, turnID)]
	delete(s.turnStartedAt, turnKey(threadID, turnID))
	s.mu.Unlock()
	return s.conn.Notify(codexschema.MethodTurnCompleted,
		TurnCompletedFinalAnswerParamsAt(threadID, turnID, "", startedAt, completedAt, itemID, text))
}

// EmitTurnFailed emits `error` to signal a failed turn.
func (s *Server) EmitTurnFailed(threadID, turnID, message string) error {
	s.mu.Lock()
	delete(s.turnStartedAt, turnKey(threadID, turnID))
	s.mu.Unlock()
	return s.conn.Notify(codexschema.MethodError, map[string]any{
		"threadId": threadID,
		"message":  message,
	})
}

// EmitAgentMessageDelta emits `item/agentMessage/delta` for streaming text.
func (s *Server) EmitAgentMessageDelta(threadID, delta string) error {
	return s.conn.Notify(codexschema.MethodItemAgentMessageDelta, AgentMessageDeltaParams(threadID, "", "", delta))
}

// EmitItemStarted emits `item/started` for a tool invocation.
func (s *Server) EmitItemStarted(threadID, turnID string, item map[string]any) error {
	return s.conn.Notify(codexschema.MethodItemStarted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"item":     item,
	})
}

// EmitItemCompleted emits `item/completed` for a finished tool invocation.
func (s *Server) EmitItemCompleted(threadID, turnID string, item map[string]any) error {
	return s.conn.Notify(codexschema.MethodItemCompleted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"item":     item,
	})
}

// EmitTokenUsage emits `thread/tokenUsage/updated`.
// last is this turn's breakdown; total is the cumulative thread total.
// Both must be maps with TokenUsageBreakdown fields (inputTokens, outputTokens, etc.).
func (s *Server) EmitTokenUsage(threadID, turnID string, last, total map[string]any) error {
	return s.conn.Notify(codexschema.MethodThreadTokenUsageUpdated, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"tokenUsage": map[string]any{
			"last":               last,
			"total":              total,
			"modelContextWindow": nil,
		},
	})
}

// TurnStartedParams returns a v2-compatible `turn/started` payload.
func TurnStartedParams(threadID, turnID string) map[string]any {
	return TurnStartedParamsAt(threadID, turnID, time.Now())
}

// TurnStartedParamsAt returns a v2-compatible `turn/started` payload with an explicit start time.
func TurnStartedParamsAt(threadID, turnID string, startedAt time.Time) map[string]any {
	startedAt = normalizeTurnTime(startedAt, time.Now())
	return map[string]any{
		"threadId": threadID,
		"turn": map[string]any{
			"completedAt": nil,
			"durationMs":  nil,
			"error":       nil,
			"id":          turnID,
			"items":       []any{},
			"itemsView":   "notLoaded",
			"startedAt":   startedAt.Unix(),
			"status":      "inProgress",
		},
	}
}

// TurnCompletedParams returns a v2-compatible `turn/completed` payload.
func TurnCompletedParams(threadID, turnID string) map[string]any {
	now := time.Now()
	return TurnCompletedParamsAt(threadID, turnID, now, now)
}

// TurnCompletedParamsAt returns a v2-compatible `turn/completed` payload with explicit timing.
func TurnCompletedParamsAt(threadID, turnID string, startedAt, completedAt time.Time) map[string]any {
	completedAt = normalizeTurnTime(completedAt, time.Now())
	startedAt = normalizeTurnTime(startedAt, completedAt)
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	return map[string]any{
		"threadId": threadID,
		"turn": map[string]any{
			"completedAt": completedAt.Unix(),
			"durationMs":  completedAt.Sub(startedAt).Milliseconds(),
			"error":       nil,
			"id":          turnID,
			"items":       []any{},
			"itemsView":   "notLoaded",
			"startedAt":   startedAt.Unix(),
			"status":      "completed",
		},
	}
}

// TurnCompletedFinalAnswerParamsAt returns a v2-compatible `turn/completed`
// payload that includes the terminal assistant answer in turn.items.
func TurnCompletedFinalAnswerParamsAt(threadID, turnID, sessionID string, startedAt, completedAt time.Time, itemID, text string) map[string]any {
	params := TurnCompletedParamsAt(threadID, turnID, startedAt, completedAt)
	if sessionID != "" {
		params["sessionId"] = sessionID
	}
	if text == "" {
		return params
	}
	turn, _ := params["turn"].(map[string]any)
	turn["items"] = []any{agentMessageTurnItem(itemID, codexschemav2.FinalAnswer, text)}
	return params
}

// AgentMessageDeltaParams returns a v2-compatible `item/agentMessage/delta` payload.
func AgentMessageDeltaParams(threadID, turnID, itemID, delta string) map[string]any {
	params := map[string]any{
		"threadId": threadID,
		"delta":    delta,
	}
	if turnID != "" {
		params["turnId"] = turnID
	}
	if itemID != "" {
		params["itemId"] = itemID
	}
	return params
}

func turnKey(threadID, turnID string) string {
	return threadID + "\x00" + turnID
}

func normalizeTurnTime(ts, fallback time.Time) time.Time {
	if ts.IsZero() {
		return fallback
	}
	return ts
}

func agentMessageTurnItem(itemID string, phase codexschemav2.MessagePhase, text string) map[string]any {
	item := map[string]any{
		"id":   itemID,
		"type": codexschemav2.AgentMessage,
		"text": text,
	}
	if phase != "" {
		item["phase"] = phase
	}
	return item
}
