package stream

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	codexschemav2 "github.com/takezoh/agent-grid/platform/agent/codexschema/v2"
)

func (b *Backend) handleNotification(method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodThreadStarted:
		b.handleThreadStarted(params)
	case codexschema.MethodTurnStarted:
		b.handleTurnStarted(params)
	case codexschema.MethodTurnCompleted:
		b.handleTurnCompleted(params)
	case codexschema.MethodThreadSettingsUpdated:
		b.handleThreadSettingsUpdated(params)
	case codexschema.MethodTurnPlanUpdated:
		b.emitToThread(extractThreadID(params), state.SubsystemPlanUpdated, func(p *state.SubsystemPayload) {
			p.Plan = &state.SubsystemPlan{Summary: summarizePlan(params)}
		})
	case codexschema.MethodTurnDiffUpdated:
		b.emitToThread(extractThreadID(params), state.SubsystemDiffUpdated, func(p *state.SubsystemPayload) {
			p.Diff = &state.SubsystemDiff{Summary: summarizeDiff(params), Paths: diffPaths(params)}
		})
	case codexschema.MethodItemStarted:
		b.emitItemLifecycle(codexschema.MethodItemStarted, params)
	case codexschema.MethodItemCompleted:
		b.emitItemLifecycle(codexschema.MethodItemCompleted, params)
	case codexschema.MethodThreadStatusChanged:
		b.handleThreadStatusChanged(params)
	case codexschema.MethodThreadNameUpdated:
		b.handleThreadNameUpdated(params)
	case codexschema.MethodItemAgentMessageDelta:
		b.handleAgentMessageDelta(params)
	case codexschema.MethodError:
		slog.Error("stream backend: app-server error", "subsystem", b.subsystemID, "params", string(params))
	case codexschema.MethodWarning, codexschema.MethodGuardianWarning, codexschema.MethodDeprecationNotice:
		slog.Warn("stream backend: app-server notice", "method", method, "subsystem", b.subsystemID, "params", string(params))
	}
}

func (b *Backend) handleTurnStarted(raw json.RawMessage) {
	meta := normalizeCodexThreadMetadata(raw)
	threadID := firstNonEmpty(meta.threadID, extractThreadID(raw))
	turnID := extractTurnID(raw)
	frameID := b.frameForThread(threadID)
	if frameID != "" {
		b.mu.Lock()
		if binding := b.frames[frameID]; binding != nil {
			binding.activeTurnID = turnID
			binding.turnAssistant = ""
			binding.lastAssistant = ""
		}
		b.mu.Unlock()
	}
	b.emitMetadata(meta)
	b.emitToThread(threadID, state.SubsystemTurnStarted, func(p *state.SubsystemPayload) {
		p.TurnID = turnID
	})
}

func (b *Backend) handleRequest(id codexclient.RequestID, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodItemCommandExecutionRequestApproval, codexschema.MethodItemFileChangeRequestApproval:
		threadID := extractThreadID(params)
		frameID := b.frameForThread(threadID)
		if frameID == "" {
			return
		}
		approval := approvalFromParams(method, params, b.autoApprove)
		b.emit(frameID, state.SubsystemApprovalRequested, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
		result := codexschema.ApprovalAccept
		if b.autoApprove {
			result = codexschema.ApprovalAcceptForSession
		}
		_ = b.conn.Reply(id, result)
		approval.Resolved = true
		b.emit(frameID, state.SubsystemApprovalResolved, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
	default:
		slog.Warn("stream backend: rejecting unhandled server request",
			"method", method, "subsystem", b.subsystemID)
		_ = b.conn.ReplyError(id, "method not supported by client")
	}
}

func (b *Backend) handleThreadStarted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	if threadID == "" {
		return
	}
	// Two arrival paths:
	//
	//   - Known thread — the CLI resumed a persisted id we recorded in
	//     b.threads at BindFrame time (recovery). We just confirm the binding.
	//   - Unknown thread — the CLI issued its own `thread/start` on its
	//     connection (fresh cold-start). Adopt it into the single pending
	//     frame reserved by the initState reservation (ADR-0081). If no frame is
	//     pending the notification is a legitimate no-op (e.g. an old thread
	//     replayed by the server, or an out-of-band client).
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		frameID = b.adoptPendingFrame(threadID)
		if frameID == "" {
			slog.Debug("stream backend: thread/started with no pending frame — dropping",
				"subsystem", b.subsystemID, "thread", threadID)
			return
		}
	}
	b.mu.Lock()
	if binding := b.frames[frameID]; binding != nil {
		binding.threadID = threadID
		if sessionID := extractThreadSessionID(raw); sessionID != "" {
			binding.sessionID = sessionID
		}
		if binding.requestedID == "" {
			binding.requestedID = threadID
		}
		binding.observedID = threadID
		if threadPath := extractThreadPath(raw); threadPath != "" {
			if _, hostPath, err := translateRolloutPath(threadPath, b.mounts); err == nil {
				binding.rolloutPath = hostPath
			}
		}
	}
	b.mu.Unlock()
	b.emitMetadata(normalizeCodexThreadMetadata(raw))
	b.startObserverSubscription(frameID)
}

// adoptPendingFrame links an unknown incoming thread id to the frame that is
// currently occupying the pending slot. Serialization is what makes this
// deterministic: acquirePendingSlot ensures at most one such frame exists
// per Backend, so the take here selects the unique owner without any
// heuristic. Returns "" if no frame is pending.
func (b *Backend) adoptPendingFrame(threadID string) state.FrameID {
	slot := b.initState.takeAny()
	if slot == nil {
		return ""
	}
	b.mu.Lock()
	binding := b.frames[slot.frameID]
	if binding == nil {
		// Frame was killed between slot acquire and adopt. Drop the thread.
		b.mu.Unlock()
		return ""
	}
	binding.threadID = threadID
	b.threads[threadID] = slot.frameID
	b.mu.Unlock()
	slog.Debug("stream backend: adopted CLI-created thread for pending frame",
		"subsystem", b.subsystemID, "frame", slot.frameID, "thread", threadID)
	return slot.frameID
}

func (b *Backend) handleTurnCompleted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	turnID := extractTurnID(raw)
	completed := strings.TrimSpace(extractText(raw))
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	if turnID != "" && binding.activeTurnID != "" && turnID != binding.activeTurnID {
		b.mu.Unlock()
		return
	}
	binding.activeTurnID = ""
	if completed == "" {
		completed = strings.TrimSpace(binding.turnAssistant)
	}
	binding.turnAssistant = ""
	binding.lastAssistant = completed
	last := completed
	if last != "" {
		appendHistory(&binding.history, "assistant", last)
	}
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemTurnCompleted, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *Backend) handleAgentMessageDelta(raw json.RawMessage) {
	params, ok := decodeAgentMessageDelta(raw)
	if !ok {
		return
	}
	text := params.text
	if text == "" {
		return
	}
	frameID := b.frameForThread(params.threadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	if params.turnID != "" && binding.activeTurnID != "" && params.turnID != binding.activeTurnID {
		b.mu.Unlock()
		return
	}
	binding.turnAssistant += text
	last := binding.turnAssistant
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemMessageUpdated, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

type decodedAgentMessageDelta struct {
	threadID string
	turnID   string
	text     string
}

func decodeAgentMessageDelta(raw json.RawMessage) (decodedAgentMessageDelta, bool) {
	notification, err := codexschemav2.UnmarshalAgentMessageDeltaNotification(raw)
	if err == nil && notification.ThreadID != "" && notification.Delta != "" {
		return decodedAgentMessageDelta{
			threadID: notification.ThreadID,
			turnID:   notification.TurnID,
			text:     notification.Delta,
		}, true
	}
	var legacy struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Delta    string `json:"delta"`
		Text     string `json:"text"`
	}
	if json.Unmarshal(raw, &legacy) != nil || legacy.ThreadID == "" {
		return decodedAgentMessageDelta{}, false
	}
	text := legacy.Delta
	if text == "" {
		text = legacy.Text
	}
	return decodedAgentMessageDelta{
		threadID: legacy.ThreadID,
		turnID:   legacy.TurnID,
		text:     text,
	}, true
}

func (b *Backend) handleThreadNameUpdated(raw json.RawMessage) {
	b.emitMetadata(normalizeCodexThreadMetadata(raw))
}

func (b *Backend) handleThreadSettingsUpdated(raw json.RawMessage) {
	meta := normalizeCodexThreadSettings(raw)
	b.applyThreadSettings(meta.threadID, meta.model, meta.modelSet, meta.effort, meta.effortSet)
	b.emitMetadata(meta)
}

func (b *Backend) emitMetadata(meta codexThreadMetadata) {
	if meta.threadID == "" || (!meta.titleSet && meta.preview == "" && meta.prompt == "" && !meta.modelSet && !meta.effortSet) {
		return
	}
	frameID := b.frameForThread(meta.threadID)
	if frameID == "" {
		return
	}
	if meta.prompt != "" {
		b.mu.Lock()
		if binding := b.frames[frameID]; binding != nil {
			appendHistory(&binding.history, "user", meta.prompt)
		}
		b.mu.Unlock()
	}
	b.emit(frameID, state.SubsystemMetadataUpdated, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.Title = meta.title
		p.TitleSet = meta.titleSet
		p.Preview = meta.preview
		p.Prompt = meta.prompt
		p.Model = meta.model
		p.ModelSet = meta.modelSet
		p.Effort = meta.effort
		p.EffortSet = meta.effortSet
	}))
}

func (b *Backend) handleThreadStatusChanged(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	prevStatus, prevWaiting := binding.threadStatus, binding.waitApproval
	events, nextStatus, nextWaiting := threadStatusEvents(raw, threadID, prevStatus, prevWaiting)
	binding.threadStatus = nextStatus
	binding.waitApproval = nextWaiting
	b.mu.Unlock()
	for _, ev := range events {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *Backend) emitItemLifecycle(method string, raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	for _, ev := range itemLifecycleEvents(method, raw, threadID) {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *Backend) emitToThread(threadID string, kind state.SubsystemEventKind, mutate func(*state.SubsystemPayload)) {
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.emit(frameID, kind, b.payloadWith(frameID, mutate))
}

func (b *Backend) payload(frameID state.FrameID) state.SubsystemPayload {
	return b.payloadWith(frameID, nil)
}

func (b *Backend) payloadWith(frameID state.FrameID, mutate func(*state.SubsystemPayload)) state.SubsystemPayload {
	b.mu.Lock()
	binding := b.frames[frameID]
	payload := state.SubsystemPayload{}
	if binding != nil {
		payload = payloadFromBinding(binding)
	}
	b.mu.Unlock()
	if mutate != nil {
		mutate(&payload)
	}
	return payload
}

func payloadFromBinding(binding *frameBinding) state.SubsystemPayload {
	return state.SubsystemPayload{
		SessionID:          binding.threadID,
		ColdStartSessionID: binding.sessionID,
		TargetID:           binding.threadID,
		RequestedTargetID:  binding.requestedID,
		ObservedTargetID:   binding.observedID,
		ResumePhase:        binding.resumePhase,
		TranscriptPath:     binding.rolloutPath,
		Model:              binding.model,
		ModelSet:           binding.modelSet,
		Effort:             binding.effort,
		EffortSet:          binding.effortSet,
	}
}

func (b *Backend) applyThreadSettings(threadID, model string, modelSet bool, effort string, effortSet bool) {
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if binding := b.frames[frameID]; binding != nil {
		if modelSet {
			binding.model = strings.TrimSpace(model)
			binding.modelSet = true
		}
		if effortSet {
			binding.effort = strings.TrimSpace(effort)
			binding.effortSet = true
		}
	}
}

func (b *Backend) withTracking(frameID state.FrameID, payload state.SubsystemPayload) state.SubsystemPayload {
	base := b.payload(frameID)
	if payload.SessionID == "" {
		payload.SessionID = base.SessionID
	}
	if payload.ColdStartSessionID == "" {
		payload.ColdStartSessionID = base.ColdStartSessionID
	}
	if payload.TargetID == "" {
		payload.TargetID = base.TargetID
	}
	payload.RequestedTargetID = base.RequestedTargetID
	payload.ObservedTargetID = base.ObservedTargetID
	payload.ResumePhase = base.ResumePhase
	return payload
}

func (b *Backend) failFrame(frameID state.FrameID, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil || binding.failureReported {
		b.mu.Unlock()
		return
	}
	binding.failureReported = true
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemFailed, state.SubsystemPayload{Error: msg})
}
