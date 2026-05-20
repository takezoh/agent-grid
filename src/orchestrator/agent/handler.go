package agent

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

type sessionIDs struct {
	threadID string
	turnID   string
}

func (s sessionIDs) sessionID() string { return s.threadID + "-" + s.turnID }

type turnResult struct {
	failed bool
	err    error
}

// turnHandler dispatches codex protocol notifications to the spawn goroutine.
type turnHandler struct {
	conn         *codexclient.Conn
	mu           sync.Mutex
	threadID     string
	sessionReady chan<- sessionIDs
	turnDone     chan<- turnResult
}

func (h *turnHandler) OnNotification(method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodThreadStarted:
		h.mu.Lock()
		h.threadID = extractThreadID(params)
		h.mu.Unlock()

	case codexschema.MethodTurnStarted:
		turnID := extractString(params, "turnId")
		h.mu.Lock()
		threadID := h.threadID
		h.mu.Unlock()
		select {
		case h.sessionReady <- sessionIDs{threadID: threadID, turnID: turnID}:
		default:
		}

	case codexschema.MethodTurnCompleted:
		select {
		case h.turnDone <- turnResult{}:
		default:
		}

	case codexschema.MethodError:
		msg := extractString(params, "message")
		select {
		case h.turnDone <- turnResult{failed: true, err: errors.New(msg)}:
		default:
		}
	}
}

func (h *turnHandler) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodItemCommandExecutionRequestApproval,
		codexschema.MethodItemFileChangeRequestApproval:
		_ = h.conn.Reply(id, map[string]any{"decision": codexschema.ApprovalAcceptForSession})
	default:
		_ = h.conn.ReplyError(id, "unsupported")
	}
}

func extractThreadID(params json.RawMessage) string {
	var p struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(params, &p)
	return p.Thread.ID
}

func extractString(params json.RawMessage, key string) string {
	var p map[string]json.RawMessage
	if err := json.Unmarshal(params, &p); err != nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(p[key], &s)
	return s
}
