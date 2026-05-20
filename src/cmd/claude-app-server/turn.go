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
	threadID string // empty on first turn; shim generates one
	cwd      string
	prompt   string
}

// turnRunner processes turns sequentially. It holds the mutable shim state.
type turnRunner struct {
	ctx     context.Context
	srv     *codexclient.Server
	writeMu *sync.Mutex
	threads map[string]string // threadID → claude session_id
	mu      sync.Mutex
	launch  claudeLauncher
	newID   func() string
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

	if isNewThread {
		if err := r.emit(func() error {
			return r.srv.EmitThreadStarted(threadID, req.cwd)
		}); err != nil {
			slog.Error("emit thread/started", "err", err)
			return
		}
	}

	if err := r.emit(func() error {
		return r.srv.Conn().Notify(codexschema.MethodTurnStarted, map[string]any{
			"threadId":  threadID,
			"turnId":    turnID,
			"sessionId": sessionID,
		})
	}); err != nil {
		slog.Error("emit turn/started", "err", err)
		return
	}

	r.mu.Lock()
	resumeID := r.threads[threadID]
	r.mu.Unlock()

	stdout, wait, err := r.launch(r.ctx, req.cwd, resumeID, req.prompt)
	if err != nil {
		slog.Error("launch claude", "err", err)
		_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, err.Error()) })
		return
	}

	resultReceived := false
	sc := streamjson.NewScanner(stdout)
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
		case streamjson.Result:
			resultReceived = true
			if ev.IsError {
				_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, ev.ResultText) })
			} else {
				_ = r.emit(func() error {
					return r.srv.Conn().Notify(codexschema.MethodTurnCompleted, map[string]any{
						"threadId":  threadID,
						"turnId":    turnID,
						"sessionId": sessionID,
						"text":      ev.ResultText,
					})
				})
			}
		}
	}
	if err := sc.Err(); err != nil {
		slog.Error("stream scan", "err", err)
	}

	if err := wait(); err != nil && !resultReceived {
		msg := fmt.Sprintf("claude exited: %v", err)
		_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, msg) })
	}
}

// emit serializes all conn writes through writeMu.
func (r *turnRunner) emit(fn func() error) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return fn()
}

// parseTurnStart decodes the turn/start notification params.
func parseTurnStart(params json.RawMessage) turnReq {
	var p struct {
		ThreadID string `json:"threadId"`
		CWD      string `json:"cwd"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal(params, &p)
	return turnReq{threadID: p.ThreadID, cwd: p.CWD, prompt: p.Message}
}
