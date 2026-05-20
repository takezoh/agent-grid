package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/takezoh/agent-roost/orchestrator/lineargql"
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

// toolCallParams is the shape of DynamicToolCallParams from the codex protocol.
type toolCallParams struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	CallID    string          `json:"callId"`
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
}

// linearArgs is the §10.5 input shape for the linear_graphql tool.
type linearArgs struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables"`
}

// turnHandler dispatches codex protocol notifications to the spawn goroutine.
type turnHandler struct {
	conn         *codexclient.Conn
	linearClient *lineargql.Client // nil when linear_graphql is not configured
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

func (h *turnHandler) OnServerRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodItemCommandExecutionRequestApproval,
		codexschema.MethodItemFileChangeRequestApproval:
		_ = h.conn.Reply(id, map[string]any{"decision": codexschema.ApprovalAcceptForSession})
	case codexschema.MethodItemToolCall:
		h.handleToolCall(id, params)
	default:
		_ = h.conn.ReplyError(id, "unsupported")
	}
}

func (h *turnHandler) handleToolCall(id int64, params json.RawMessage) {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		_ = h.conn.ReplyError(id, "invalid tool call params")
		return
	}
	if p.Tool != "linear_graphql" || h.linearClient == nil {
		_ = h.conn.ReplyError(id, "unknown tool: "+p.Tool)
		return
	}

	var args linearArgs
	if err := json.Unmarshal(p.Arguments, &args); err != nil {
		_ = h.conn.ReplyError(id, "invalid linear_graphql arguments")
		return
	}

	result, err := h.linearClient.Execute(context.Background(), args.Query, args.Variables)
	if err != nil {
		_ = h.conn.ReplyError(id, "linear_graphql internal error")
		return
	}
	_ = h.conn.Reply(id, result)
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
