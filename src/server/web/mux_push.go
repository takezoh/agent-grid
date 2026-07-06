package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

// apiPushReq is the POST /api/sessions/{id}/push body. The web command palette
// uses this route to push a curated [session].push_commands entry onto the
// session named in the URL path. The web client owns its own
// active-session-per-tab, so the push targets exactly the path id (which must
// be a known session on the daemon).
type apiPushReq struct {
	Command string `json:"command"`
}

// pushBodyLimit caps the JSON body for /api/sessions/{id}/push. 64KiB is
// generous for any plausible /clear-style command picker entry but tight
// enough that a runaway client cannot exhaust gateway memory by streaming
// an unbounded body.
const pushBodyLimit = 64 * 1024

// handlePushCommand pushes a curated command onto the session named in the URL
// path via state.EventPushDriver:
//   - 400 if the JSON body is malformed or command is empty
//   - 400 if the path id violates the session-id allowlist (ADR 0026)
//   - 404 if the path id is not a known session on the daemon
//   - 413 if the body exceeds pushBodyLimit (distinct from 400; see
//     decodePushBody)
//   - 502/504/503 per handleProtoError on RPC failure
//   - 200 on success
//
// Implementation uses ListSessions only to confirm the path id is a known
// session (404 otherwise). This keeps the handler aligned with the
// SendCommand-based shape every other write handler uses (ADR-0045) and avoids
// a new daemon-state cache on the gateway.
func handlePushCommand(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		id := r.PathValue("id")
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id",
				"invalid session id", "id", id)
			return
		}
		command, ok := decodePushBody(w, r)
		if !ok {
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		found, ok := sessionKnown(ctx, d, w, r, id)
		if !ok {
			return
		}
		if !found {
			gatewayError(w, r, http.StatusNotFound, "session_not_found",
				"session not found", "id", id)
			return
		}
		sendPushDriver(ctx, d, w, r, id, command)
	}
}

// decodePushBody reads the JSON body under pushBodyLimit and returns the
// trimmed command. Writes the appropriate 4xx via gatewayError and returns
// ok=false on any validation failure.
//
// Body too large is surfaced as 413 Payload Too Large (distinct from the 400
// returned for malformed JSON). http.MaxBytesReader is used over a plain
// io.LimitReader because the latter would let the json.Decoder fail with a
// generic "unexpected EOF" mid-stream — clients could not tell whether they
// had sent malformed JSON or a body that exceeded the cap. MaxBytesReader
// raises *http.MaxBytesError once the cap is hit, which we map to 413; any
// other decode error stays 400.
func decodePushBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, pushBodyLimit)
	var body apiPushReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			gatewayError(w, r, http.StatusRequestEntityTooLarge, "body_too_large",
				"request body exceeds limit",
				"limit_bytes", pushBodyLimit, "err", err)
			return "", false
		}
		gatewayError(w, r, http.StatusBadRequest, "bad_request",
			"bad request body", "err", err)
		return "", false
	}
	cmd := strings.TrimSpace(body.Command)
	if cmd == "" {
		gatewayError(w, r, http.StatusBadRequest, "empty_command",
			"command must be non-empty")
		return "", false
	}
	return cmd, true
}

// sessionKnown issues a ListSessions RPC and reports whether the path id is a
// known session on the daemon. When ok=false the response has already been
// written.
func sessionKnown(ctx context.Context, d *DaemonClient, w http.ResponseWriter, r *http.Request, id string) (found, ok bool) {
	resp, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventListSessions,
		Payload: json.RawMessage("{}"),
	})
	if err != nil {
		handleProtoError(w, r, err)
		return false, false
	}
	rs, isSessions := resp.(proto.RespSessions)
	if !isSessions {
		gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
			"unexpected response type", "got_type", typeName(resp))
		return false, false
	}
	for _, s := range rs.Sessions {
		if s.ID == id {
			return true, true
		}
	}
	return false, true
}

// sendPushDriver dispatches state.EventPushDriver to the daemon. Writes 502
// on RPC failure (handleProtoError) and 200 on success.
func sendPushDriver(ctx context.Context, d *DaemonClient, w http.ResponseWriter, r *http.Request, id, command string) {
	payload, err := json.Marshal(state.PushDriverParams{
		SessionID: id,
		Command:   command,
	})
	if err != nil {
		gatewayError(w, r, http.StatusInternalServerError, "marshal_error",
			"internal error", "err", err)
		return
	}
	if _, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventPushDriver,
		Payload: json.RawMessage(payload),
	}); err != nil {
		handleProtoError(w, r, err)
		return
	}
	_ = requestID(w, r)
	w.WriteHeader(http.StatusOK)
}
