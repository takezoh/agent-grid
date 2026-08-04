package api

import (
	"encoding/json"
	"net/http"

	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

// Resume-capsule routes (plans/cognitive-work-orchestration-ux.md Phase 0).
// The gateway stays a stateless proxy: capsules live daemon-side and every
// handler is a thin CmdEvent forward.

func registerResumeCapsuleRoutes(mux *http.ServeMux, deps muxDependencies) {
	mux.HandleFunc("GET /api/resume-capsules", handleListResumeCapsules(deps.daemon))
	mux.HandleFunc("POST /api/sessions/{id}/resume-capsule/flag", handleFlagResumeCapsule(deps.daemon))
}

func handleListResumeCapsules(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d == nil || !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		response, err := d.SendCommand(ctx, proto.CmdEvent{Event: state.EventListResumeCapsules})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		capsules, ok := response.(proto.RespResumeCapsules)
		if !ok {
			gatewayError(w, r, http.StatusBadGateway, "response_type_mismatch", "unexpected resume-capsule response")
			return
		}
		if capsules.ResumeCapsules == nil {
			capsules.ResumeCapsules = []proto.ResumeCapsuleInfo{}
		}
		writeJSON(w, http.StatusOK, capsules)
	}
}

type resumeCapsuleFlagRequest struct {
	Flagged bool `json:"flagged"`
}

func handleFlagResumeCapsule(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d == nil || !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		id := r.PathValue("id")
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id")
			return
		}
		req := resumeCapsuleFlagRequest{Flagged: true}
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				gatewayError(w, r, http.StatusBadRequest, "invalid_body", "invalid flag request body")
				return
			}
		}
		payload, _ := json.Marshal(state.ResumeCapsuleFlagParams{SessionID: id, Flagged: req.Flagged})
		ctx, cancel := rpcContext(r)
		defer cancel()
		if _, err := d.SendCommand(ctx, proto.CmdEvent{Event: state.EventFlagResumeCapsule, Payload: payload}); err != nil {
			handleProtoError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"flagged": req.Flagged})
	}
}
