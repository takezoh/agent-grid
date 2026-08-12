package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/services/host/config"
	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

// apiCreateReq is the POST /api/sessions body. Required: version 2, project,
// profile id and profile revision. Optional: typed parameters, terminal
// cols/rows hint, and worktree.
type apiCreateReq struct {
	Version         int            `json:"version,omitempty"`
	Project         string         `json:"project"`
	ProfileID       string         `json:"profile_id,omitempty"`
	ProfileRevision string         `json:"profile_revision,omitempty"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	Cols            int            `json:"cols,omitempty"`
	Rows            int            `json:"rows,omitempty"`
	Worktree        bool           `json:"worktree,omitempty"`
}

func handleCreateSession(
	d *DaemonClient,
	loadConfig func() (*config.Config, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		req, ok := decodeCreateRequest(w, r)
		if !ok {
			return
		}
		command, ok := materializeCreateCommand(w, r, req, loadConfig)
		if !ok {
			return
		}
		createSession(w, r, d, req, command)
	}
}

func decodeCreateRequest(
	w http.ResponseWriter,
	r *http.Request,
) (apiCreateReq, bool) {
	var req apiCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gatewayError(w, r, http.StatusBadRequest, "bad_request",
			"bad request body", "err", err)
		return req, false
	}
	if !isAbsoluteProjectPath(req.Project) {
		gatewayError(w, r, http.StatusBadRequest, "project_not_absolute",
			"project must be an absolute path (e.g. /home/me/myrepo); got "+strconv.Quote(req.Project),
			"project", req.Project)
		return req, false
	}
	if err := state.ValidateSizeHint(req.Cols, req.Rows); err != nil {
		gatewayAPIError(w, r, http.StatusBadRequest, "invalid_cols_rows", err.Error())
		return req, false
	}
	if req.Version != 2 || req.ProfileID == "" || req.ProfileRevision == "" {
		gatewayAPIError(w, r, http.StatusBadRequest, "invalid_create_version",
			"version 2 requires profile_id and profile_revision")
		return req, false
	}
	return req, true
}

func materializeCreateCommand(
	w http.ResponseWriter,
	r *http.Request,
	req apiCreateReq,
	loadConfig func() (*config.Config, error),
) (string, bool) {
	cfg, err := loadConfig()
	if err != nil {
		gatewayError(w, r, http.StatusInternalServerError, "config_load_failed",
			"settings.toml load failed", "err", err)
		return "", false
	}
	profiles, err := cfg.Session.ResolveProfiles(req.Project)
	if err != nil {
		gatewayError(w, r, http.StatusInternalServerError, "project_config_load_failed",
			"project settings.toml load failed", "project", req.Project, "err", err)
		return "", false
	}
	argv, err := config.MaterializeResolvedProfile(
		profiles, req.ProfileID, req.ProfileRevision, req.Parameters)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_profile_parameters"
		if strings.Contains(err.Error(), "stale profile revision") {
			status = http.StatusConflict
			code = "stale_profile_revision"
		}
		gatewayAPIError(w, r, status, code, err.Error())
		return "", false
	}
	return joinShellCommand(argv), true
}

func createSession(
	w http.ResponseWriter,
	r *http.Request,
	d *DaemonClient,
	req apiCreateReq,
	command string,
) {
	params := state.CreateSessionParams{
		Project: req.Project,
		Command: command,
		Options: state.LaunchOptions{
			Cols:     uint16(req.Cols),
			Rows:     uint16(req.Rows),
			Worktree: state.WorktreeOption{Enabled: req.Worktree},
		},
	}
	payload, err := json.Marshal(params)
	if err != nil {
		gatewayError(w, r, http.StatusInternalServerError, "marshal_error",
			"internal error", "err", err)
		return
	}
	ctx, cancel := rpcContext(r)
	defer cancel()
	resp, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventCreateSession,
		Payload: json.RawMessage(payload),
	})
	if err != nil {
		handleProtoError(w, r, err)
		return
	}
	rc, ok := resp.(proto.RespCreateSession)
	if !ok {
		gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
			"unexpected response type", "got_type", typeName(resp))
		return
	}
	writeJSON(w, http.StatusCreated, apiSessionInfo{
		ID:        rc.SessionID,
		Project:   req.Project,
		Command:   command,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func joinShellCommand(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
				(r < '0' || r > '9') && !strings.ContainsRune("_-./:@", r)
		}) == -1 {
			quoted[i] = arg
			continue
		}
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
}
