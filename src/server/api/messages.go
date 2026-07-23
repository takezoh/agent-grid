package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

type apiSessionMessages struct {
	SessionID string                       `json:"session_id"`
	Summary   *proto.FrameMessagingSummary `json:"summary,omitempty"`
	Messages  []proto.SessionMessage       `json:"messages"`
}

type apiReadSessionMessagesReq struct {
	LastReadMessageID string `json:"last_read_message_id,omitempty"`
}

func handleGetSessionMessages(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		id := r.PathValue("id")
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		resp, err := d.SendCommand(ctx, proto.CmdEvent{
			Event:   state.EventListSessionMessages,
			Payload: mustJSON(state.SessionMessagesParams{SessionID: id}),
		})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		msgs, ok := resp.(proto.RespSessionMessages)
		if !ok {
			gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
				"unexpected response type", "got_type", typeName(resp))
			return
		}
		if msgs.Messages == nil {
			msgs.Messages = []proto.SessionMessage{}
		}
		writeJSON(w, http.StatusOK, apiSessionMessages{
			SessionID: msgs.SessionID,
			Summary:   msgs.Summary,
			Messages:  msgs.Messages,
		})
	}
}

func handleReadSessionMessages(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		id := r.PathValue("id")
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		body, ok := decodeReadSessionMessagesBody(w, r)
		if !ok {
			return
		}
		if err := sendSessionMessagesRead(ctx, d, id, body.LastReadMessageID); err != nil {
			handleProtoError(w, r, err)
			return
		}
		_ = requestID(w, r)
		w.WriteHeader(http.StatusNoContent)
	}
}

func sendSessionMessagesRead(ctx context.Context, d *DaemonClient, sessionID, lastReadMessageID string) error {
	_, err := d.SendCommand(ctx, proto.CmdEvent{
		Event: state.EventReadSessionMessages,
		Payload: mustJSON(state.SessionMessagesParams{
			SessionID:         sessionID,
			LastReadMessageID: lastReadMessageID,
		}),
	})
	return err
}

func decodeReadSessionMessagesBody(w http.ResponseWriter, r *http.Request) (apiReadSessionMessagesReq, bool) {
	var body apiReadSessionMessagesReq
	if r.Body == nil {
		return body, true
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return body, true
		}
		gatewayError(w, r, http.StatusBadRequest, "bad_request",
			"bad request body", "err", err)
		return apiReadSessionMessagesReq{}, false
	}
	return body, true
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}
