package codexclient

import "time"

const (
	defaultApprovalPolicy    = "never"
	defaultApprovalsReviewer = "user"
	defaultModel             = "claude"
	defaultModelProvider     = "anthropic"
	defaultThreadSource      = "appServer"
)

// ThreadDescriptor carries the minimum thread metadata needed to build
// schema-compatible thread responses and notifications.
type ThreadDescriptor struct {
	ThreadID    string
	SessionID   string
	CWD         string
	RolloutPath string
	Preview     string
}

// ThreadStartedParams returns a v2-compatible `thread/started` payload.
func ThreadStartedParams(thread ThreadDescriptor) map[string]any {
	return map[string]any{
		"thread": threadPayload(thread, time.Now()),
	}
}

// ThreadStartResponse returns a v2-compatible `thread/start` response.
func ThreadStartResponse(thread ThreadDescriptor) map[string]any {
	return threadSessionResponse(thread)
}

// ThreadResumeResponse returns a v2-compatible `thread/resume` response.
func ThreadResumeResponse(thread ThreadDescriptor) map[string]any {
	return threadSessionResponse(thread)
}

// TurnStartResponseAt returns a v2-compatible `turn/start` response with an
// explicit start time.
func TurnStartResponseAt(turnID string, startedAt time.Time) map[string]any {
	startedAt = normalizeTurnTime(startedAt, time.Now())
	return map[string]any{
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

func threadSessionResponse(thread ThreadDescriptor) map[string]any {
	thread = normalizeThreadDescriptor(thread)
	return map[string]any{
		"approvalPolicy":    defaultApprovalPolicy,
		"approvalsReviewer": defaultApprovalsReviewer,
		"cwd":               thread.CWD,
		"model":             defaultModel,
		"modelProvider":     defaultModelProvider,
		"sandbox":           map[string]any{"type": "dangerFullAccess"},
		"thread":            threadPayload(thread, time.Now()),
	}
}

func threadPayload(thread ThreadDescriptor, now time.Time) map[string]any {
	thread = normalizeThreadDescriptor(thread)
	ts := now.Unix()
	payload := map[string]any{
		"agentNickname": nil,
		"agentRole":     nil,
		"cliVersion":    "agent-grid",
		"createdAt":     ts,
		"cwd":           thread.CWD,
		"ephemeral":     false,
		"forkedFromId":  nil,
		"gitInfo":       nil,
		"id":            thread.ThreadID,
		"modelProvider": defaultModelProvider,
		"name":          nil,
		"preview":       thread.Preview,
		"sessionId":     thread.SessionID,
		"source":        defaultThreadSource,
		"status":        map[string]any{"type": "idle"},
		"threadSource":  nil,
		"turns":         []any{},
		"updatedAt":     ts,
	}
	if thread.RolloutPath != "" {
		payload["path"] = thread.RolloutPath
	}
	return payload
}

func normalizeThreadDescriptor(thread ThreadDescriptor) ThreadDescriptor {
	if thread.ThreadID == "" {
		thread.ThreadID = "thread"
	}
	if thread.SessionID == "" {
		thread.SessionID = thread.ThreadID
	}
	if thread.CWD == "" {
		thread.CWD = "/"
	}
	return thread
}
