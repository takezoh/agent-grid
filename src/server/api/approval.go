package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/takezoh/agent-grid/host/proto"
)

// HeaderClientInstanceID is the REST header REST callers may use to present
// the same ephemeral client-instance-id minted with their WS ticket
// (FR-P0-12 / adr-20260724-approval-answerer-identity-per-ws-instance).
const HeaderClientInstanceID = "X-Client-Instance-ID"

type approvalRespondBody struct {
	Decision         string `json:"decision"`
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

type questionRespondBody struct {
	Answer           string `json:"answer"`
	ClientInstanceID string `json:"client_instance_id,omitempty"`
}

func handleApprovalRespond(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		approvalID := r.PathValue("approvalId")
		if sessionID == "" || approvalID == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "session id and approval id required"})
			return
		}
		var body approvalRespondBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "invalid json body"})
			return
		}
		if body.Decision != "accept" && body.Decision != "deny" {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "decision must be accept or deny"})
			return
		}
		ci := body.ClientInstanceID
		if ci == "" {
			ci = r.Header.Get(HeaderClientInstanceID)
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		_, err := d.SendCommand(ctx, proto.CmdApprovalRespond{
			SessionID:        sessionID,
			ApprovalID:       approvalID,
			Decision:         body.Decision,
			ClientInstanceID: ci,
		})
		if err != nil {
			writeDaemonError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handleQuestionRespond(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		questionID := r.PathValue("questionId")
		if sessionID == "" || questionID == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "session id and question id required"})
			return
		}
		// Free-text only: reject structured JSON objects for answer.
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "invalid json body"})
			return
		}
		answerRaw, ok := raw["answer"]
		if !ok {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "answer required"})
			return
		}
		var answer string
		if err := json.Unmarshal(answerRaw, &answer); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorBody{Code: "invalid_argument", Message: "answer must be a free-text string"})
			return
		}
		var body questionRespondBody
		body.Answer = answer
		if ciRaw, ok := raw["client_instance_id"]; ok {
			_ = json.Unmarshal(ciRaw, &body.ClientInstanceID)
		}
		ci := body.ClientInstanceID
		if ci == "" {
			ci = r.Header.Get(HeaderClientInstanceID)
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		_, err := d.SendCommand(ctx, proto.CmdQuestionRespond{
			SessionID:        sessionID,
			QuestionID:       questionID,
			Answer:           body.Answer,
			ClientInstanceID: ci,
		})
		if err != nil {
			writeDaemonError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handlePendingHumanInput is a Phase 0/1 placeholder: full authoritative
// resubscribe payload lands with chunk-p0-06. For now returns empty lists so
// clients can poll without 404.
func handlePendingHumanInput(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = d
		_ = r.PathValue("id")
		writeJSON(w, http.StatusOK, proto.RespPendingHumanInput{})
	}
}

func handleCapabilities() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    BundledCapabilities,
			"axis":            "bundled",
		})
	}
}

func writeDaemonError(w http.ResponseWriter, err error) {
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		status := http.StatusBadGateway
		switch eb.Code {
		case proto.ErrNotFound:
			status = http.StatusNotFound
		case proto.ErrInvalidArgument:
			status = http.StatusBadRequest
		case proto.ErrResolvedByOther:
			status = http.StatusConflict
		case proto.ErrUnknown, proto.ErrInternal, proto.ErrSessionStopped,
			proto.ErrAlreadyExists, proto.ErrUnsupported, proto.ErrFrameNotReady,
			proto.ErrResourceExhausted:
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]any{
			"code":    string(eb.Code),
			"message": eb.Message,
			"details": eb.Details,
		})
		return
	}
	writeJSON(w, http.StatusBadGateway, apiErrorBody{Code: "daemon_error", Message: err.Error()})
}

func handleLifecycleApprovalRespond(ctx context.Context, sess Attacher, responses *lifecycleResponder, msg *inbound, clientInstanceID string) {
	if msg.SessionID == "" || msg.ApprovalID == "" {
		responses.err(ctx, msg.ReqID, "invalid_argument", "sessionId and approvalId required")
		return
	}
	if msg.Decision != "accept" && msg.Decision != "deny" {
		responses.err(ctx, msg.ReqID, "invalid_argument", "decision must be accept or deny")
		return
	}
	adapter, ok := sess.(*DaemonAdapter)
	if !ok {
		responses.err(ctx, msg.ReqID, "internal", "approval respond requires DaemonAdapter")
		return
	}
	_, err := adapter.d.SendCommand(ctx, proto.CmdApprovalRespond{
		SessionID:        msg.SessionID,
		ApprovalID:       msg.ApprovalID,
		Decision:         msg.Decision,
		ClientInstanceID: clientInstanceID,
	})
	if err != nil {
		code, message := unwrapProtoError(err)
		slog.Warn("server/api: lifecycle approval respond", "err", err)
		responses.err(ctx, msg.ReqID, code, message)
		return
	}
	responses.ok(ctx, msg.ReqID)
}

func handleLifecycleQuestionRespond(ctx context.Context, sess Attacher, responses *lifecycleResponder, msg *inbound, clientInstanceID string) {
	if msg.SessionID == "" || msg.QuestionID == "" {
		responses.err(ctx, msg.ReqID, "invalid_argument", "sessionId and questionId required")
		return
	}
	adapter, ok := sess.(*DaemonAdapter)
	if !ok {
		responses.err(ctx, msg.ReqID, "internal", "question respond requires DaemonAdapter")
		return
	}
	_, err := adapter.d.SendCommand(ctx, proto.CmdQuestionRespond{
		SessionID:        msg.SessionID,
		QuestionID:       msg.QuestionID,
		Answer:           msg.Answer,
		ClientInstanceID: clientInstanceID,
	})
	if err != nil {
		code, message := unwrapProtoError(err)
		slog.Warn("server/api: lifecycle question respond", "err", err)
		responses.err(ctx, msg.ReqID, code, message)
		return
	}
	responses.ok(ctx, msg.ReqID)
}
