package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/lib/claude/streamjson"
)

// turnReq carries a decoded turn/start notification payload.
type turnReq struct {
	threadID       string // empty on first turn; shim generates one
	cwd            string
	prompt         string
	approvalPolicy string // logged but not enforced; container is the boundary
	sandboxPolicy  string // logged but not enforced
}

// turnRunner processes turns sequentially. It holds the mutable shim state.
type turnRunner struct {
	ctx      context.Context
	srv      *codexclient.Server
	writeMu  *sync.Mutex
	threads  map[string]string           // threadID → claude session_id
	cumUsage map[string]streamjson.Usage // threadID → cumulative token usage
	dynTools map[string][]dynToolSpec    // threadID → advertised dynamic tools (§10.5)
	mu       sync.Mutex
	launch   claudeLauncher
	newID    func() string
}

// startThread allocates a thread id and records its advertised dynamic tools.
// Called when the orchestrator sends thread/start.
func (r *turnRunner) startThread(tools []dynToolSpec) string {
	threadID := r.newID()
	r.mu.Lock()
	r.dynTools[threadID] = tools
	r.mu.Unlock()
	return threadID
}

func (r *turnRunner) run(turns <-chan turnReq, stopCh <-chan struct{}) {
	for {
		select {
		case req, ok := <-turns:
			if !ok {
				return
			}
			r.runTurn(req)
		case <-stopCh:
			return
		}
	}
}

func (r *turnRunner) runTurn(req turnReq) {
	threadID := req.threadID
	isNewThread := threadID == ""
	if isNewThread {
		threadID = r.newID()
	}
	turnID := r.newID()
	sessionID := threadID + "-" + turnID

	if req.approvalPolicy != "" || req.sandboxPolicy != "" {
		slog.Warn("approval/sandbox policy received but not enforced by shim; container is the boundary",
			"approvalPolicy", req.approvalPolicy,
			"sandboxPolicy", req.sandboxPolicy,
		)
	}

	if isNewThread {
		if err := r.emit(func() error { return r.srv.EmitThreadStarted(threadID, req.cwd) }); err != nil {
			slog.Error("emit thread/started", "err", err)
			return
		}
	}
	if err := r.emit(func() error {
		return r.srv.Conn().Notify(codexschema.MethodTurnStarted, map[string]any{
			"threadId": threadID, "turnId": turnID, "sessionId": sessionID,
		})
	}); err != nil {
		slog.Error("emit turn/started", "err", err)
		return
	}

	r.mu.Lock()
	tools := r.dynTools[threadID]
	r.mu.Unlock()
	r.runTurnLoop(threadID, turnID, sessionID, req.cwd, req.prompt, buildToolSystemPrompt(tools))
}

// runTurnLoop drives one codex turn. With dynamic tools advertised it simulates
// codex's client-tool round-trips: launch claude, and when claude emits a
// tool-call sentinel, forward it to the orchestrator and resume claude with the
// result. The whole loop is presented as a single turn (one turn/started ..
// turn/completed); internal claude resumes are hidden from the orchestrator.
func (r *turnRunner) runTurnLoop(threadID, turnID, sessionID, cwd, prompt, sysPrompt string) {
	var turnUsage streamjson.Usage
	for iter := 0; ; iter++ {
		r.mu.Lock()
		resumeID := r.threads[threadID]
		r.mu.Unlock()

		stdout, wait, err := r.launch(r.ctx, cwd, resumeID, sysPrompt, prompt)
		if err != nil {
			slog.Error("launch claude", "err", err)
			_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, err.Error()) })
			return
		}
		scan := r.scanStream(threadID, turnID, streamjson.NewScanner(stdout))
		werr := wait()

		// No result line means the process ended without turn/completed or error.
		// Always emit a failure so the orchestrator does not wait out turn_timeout.
		if !scan.resultReceived {
			msg := "claude exited without emitting a result"
			if werr != nil {
				msg = fmt.Sprintf("claude exited: %v", werr)
			}
			_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, msg) })
			return
		}
		turnUsage = addUsage(turnUsage, scan.usage)
		if scan.isError {
			_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, scan.resultText) })
			return
		}

		call, isCall := toolCall{}, false
		if sysPrompt != "" {
			call, isCall = parseToolCall(scan.resultText)
		}
		if !isCall {
			r.completeTurn(threadID, turnID, sessionID, turnUsage, scan.resultText)
			return
		}
		if iter >= maxToolCalls {
			_ = r.emit(func() error {
				return r.srv.EmitTurnFailed(threadID, fmt.Sprintf("exceeded max external tool calls (%d)", maxToolCalls))
			})
			return
		}
		prompt = r.runToolCall(threadID, turnID, call)
	}
}

// runToolCall forwards a dynamic-tool invocation to the orchestrator via
// item/tool/call and returns the resume prompt carrying the result. The
// orchestrator executes the tool (it holds the credentials); the shim never
// sees the raw token.
func (r *turnRunner) runToolCall(threadID, turnID string, call toolCall) string {
	res, err := r.srv.Conn().Request(codexschema.MethodItemToolCall, map[string]any{
		"tool":      call.Tool,
		"arguments": call.Arguments,
		"callId":    r.newID(),
		"threadId":  threadID,
		"turnId":    turnID,
	})
	if err != nil {
		return fmt.Sprintf("External tool `%s` failed: %v\n\nContinue with the task without it.", call.Tool, err)
	}
	return formatToolResult(call, res)
}

// turnScanResult captures the outcome of scanning one claude invocation.
type turnScanResult struct {
	resultReceived bool
	isError        bool
	resultText     string
	usage          streamjson.Usage
}

// scanStream processes claude stream-json events for one claude invocation,
// emitting per-item Codex notifications, and returns the final result. It does
// not emit turn/completed or turn/failed — runTurnLoop decides that after
// checking for a tool call.
func (r *turnRunner) scanStream(threadID, turnID string, sc *streamjson.Scanner) turnScanResult {
	toolNames := map[string]string{} // toolUseID → name for item/completed correlation
	var out turnScanResult
	for sc.Scan() {
		switch ev := sc.Event().(type) {
		case streamjson.SystemInit:
			r.mu.Lock()
			r.threads[threadID] = ev.SessionID
			r.mu.Unlock()

		case streamjson.AssistantMessage:
			if ev.Text != "" {
				_ = r.emit(func() error { return r.srv.EmitAgentMessageDelta(threadID, ev.Text) })
			}
			for _, tu := range ev.ToolUses {
				toolNames[tu.ID] = tu.Name
				id, name, input := tu.ID, tu.Name, tu.Input
				_ = r.emit(func() error {
					return r.srv.EmitItemStarted(threadID, turnID, map[string]any{
						"id": id, "type": "dynamicToolCall", "tool": name, "arguments": input,
					})
				})
			}

		case streamjson.ToolResult:
			status := "completed"
			if ev.IsError {
				status = "failed"
			}
			id, tool, content := ev.ToolUseID, toolNames[ev.ToolUseID], ev.Content
			_ = r.emit(func() error {
				return r.srv.EmitItemCompleted(threadID, turnID, map[string]any{
					"id": id, "type": "dynamicToolCall", "tool": tool, "status": status, "output": content,
				})
			})

		case streamjson.Result:
			out.resultReceived = true
			out.isError = ev.IsError
			out.resultText = ev.ResultText
			out.usage = ev.Usage
		}
	}
	if err := sc.Err(); err != nil {
		slog.Error("stream scan", "err", err)
	}
	return out
}

// completeTurn accumulates token usage for the thread and emits the final
// thread/tokenUsage/updated + turn/completed for the turn.
func (r *turnRunner) completeTurn(threadID, turnID, sessionID string, u streamjson.Usage, text string) {
	r.mu.Lock()
	cum := addUsage(r.cumUsage[threadID], u)
	r.cumUsage[threadID] = cum
	r.mu.Unlock()

	last, total := usageBreakdown(u), usageBreakdown(cum)
	_ = r.emit(func() error { return r.srv.EmitTokenUsage(threadID, turnID, last, total) })
	_ = r.emit(func() error {
		return r.srv.Conn().Notify(codexschema.MethodTurnCompleted, map[string]any{
			"threadId": threadID, "turnId": turnID, "sessionId": sessionID, "text": text,
		})
	})
}

// emit serializes all conn writes through writeMu.
func (r *turnRunner) emit(fn func() error) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return fn()
}

// addUsage returns a+b with TotalTokens recomputed as input+output.
func addUsage(a, b streamjson.Usage) streamjson.Usage {
	a.InputTokens += b.InputTokens
	a.OutputTokens += b.OutputTokens
	a.TotalTokens = a.InputTokens + a.OutputTokens
	return a
}

// usageBreakdown converts a streamjson.Usage to the codex TokenUsageBreakdown map shape.
func usageBreakdown(u streamjson.Usage) map[string]any {
	return map[string]any{
		"inputTokens":           u.InputTokens,
		"outputTokens":          u.OutputTokens,
		"totalTokens":           u.Total(),
		"cachedInputTokens":     0,
		"reasoningOutputTokens": 0,
	}
}

// parseTurnStart decodes the turn/start notification params.
func parseTurnStart(params json.RawMessage) turnReq {
	var p struct {
		ThreadID       string          `json:"threadId"`
		CWD            string          `json:"cwd"`
		Message        string          `json:"message"`
		ApprovalPolicy json.RawMessage `json:"approvalPolicy"`
		SandboxPolicy  json.RawMessage `json:"sandboxPolicy"`
	}
	_ = json.Unmarshal(params, &p)
	return turnReq{
		threadID:       p.ThreadID,
		cwd:            p.CWD,
		prompt:         p.Message,
		approvalPolicy: string(p.ApprovalPolicy),
		sandboxPolicy:  string(p.SandboxPolicy),
	}
}
