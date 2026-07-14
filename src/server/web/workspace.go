package web

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
	gitlib "github.com/takezoh/agent-grid/platform/lib/git"
)

type workspaceRootHandle struct {
	SessionID        string `json:"session_id"`
	FrameGeneration  int    `json:"frame_generation"`
	ResolvedRootPath string `json:"resolved_root_path"`
}

type workspaceHandleStaleError struct {
	Error            string `json:"error"`
	SessionID        string `json:"session_id"`
	FrameGeneration  int    `json:"frame_generation"`
	PinnedGeneration int    `json:"pinned_frame_generation"`
	ResolvedRootPath string `json:"resolved_root_path,omitempty"`
}

type workspaceTreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type workspaceTreeResponse struct {
	Outcome string               `json:"outcome"`
	Path    string               `json:"path,omitempty"`
	Entries []workspaceTreeEntry `json:"entries,omitempty"`
}

type workspaceFileResponse struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsBinary    bool   `json:"is_binary"`
	ContentType string `json:"content_type,omitempty"`
	Content     string `json:"content,omitempty"`
}

type workspaceDiffResponse struct {
	Outcome string `json:"outcome"`
	Diff    string `json:"diff,omitempty"`
}

func handleWorkspaceRootHandle(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveWorkspaceRootHandle(d, w, r)
	}
}

func handleWorkspaceTree(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveWorkspaceTree(d, w, r)
	}
}

func handleWorkspaceFile(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveWorkspaceFile(d, w, r)
	}
}

func handleWorkspaceDiff(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveWorkspaceDiff(d, w, r)
	}
}

func serveWorkspaceRootHandle(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionIDPattern.MatchString(id) {
		gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
		return
	}
	if !d.Health() {
		gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
		return
	}
	info, err := resolveWorkspaceSession(d, r, id)
	if err != nil {
		writeWorkspaceResolveError(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaceRootHandle{
		SessionID:        info.sessionID,
		FrameGeneration:  info.frameGeneration,
		ResolvedRootPath: info.workspaceRoot,
	})
}

func serveWorkspaceTree(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionIDPattern.MatchString(id) {
		gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
		return
	}
	if !d.Health() {
		gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
		return
	}
	info, err := resolveWorkspaceSession(d, r, id)
	if err != nil {
		writeWorkspaceResolveError(w, r, id, err)
		return
	}
	stale, pinnedRoot := workspaceHandleStale(w, r, info)
	if stale {
		return
	}
	root := pinnedRootOrCurrent(pinnedRoot, info.workspaceRoot)
	if _, err := os.Stat(root); err != nil {
		writeJSON(w, http.StatusOK, workspaceTreeResponse{Outcome: "root_unreachable"})
		return
	}
	rel := r.URL.Query().Get("path")
	resolved, err := GuardWorkspacePath(root, rel)
	if err != nil {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return
	}
	out := make([]workspaceTreeEntry, 0, len(entries))
	prefix := rel
	if prefix != "" {
		prefix += string(filepath.Separator)
	}
	for _, ent := range entries {
		entryType := "file"
		if ent.IsDir() {
			entryType = "dir"
		}
		out = append(out, workspaceTreeEntry{
			Name: ent.Name(),
			Path: prefix + ent.Name(),
			Type: entryType,
		})
	}
	writeJSON(w, http.StatusOK, workspaceTreeResponse{
		Outcome: "ok",
		Path:    rel,
		Entries: out,
	})
}

func serveWorkspaceFile(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionIDPattern.MatchString(id) {
		gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
		return
	}
	if !d.Health() {
		gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
		return
	}
	info, err := resolveWorkspaceSession(d, r, id)
	if err != nil {
		writeWorkspaceResolveError(w, r, id, err)
		return
	}
	stale, pinnedRoot := workspaceHandleStale(w, r, info)
	if stale {
		return
	}
	root := pinnedRootOrCurrent(pinnedRoot, info.workspaceRoot)
	rel := r.URL.Query().Get("path")
	if rel == "" {
		gatewayError(w, r, http.StatusBadRequest, "path_required", "path required")
		return
	}
	resolved, err := GuardWorkspacePath(root, rel)
	if err != nil {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return
	}
	fi, err := os.Stat(resolved)
	if err != nil || fi.IsDir() {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return
	}
	resp := workspaceFileResponse{
		Path: rel,
		Size: fi.Size(),
	}
	data, err := os.ReadFile(resolved) //nolint:gosec // path guarded under workspace root
	if err != nil {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return
	}
	if workspaceSniffBinary(data) {
		resp.IsBinary = true
		if len(data) > 512 {
			resp.ContentType = http.DetectContentType(data[:512])
		} else {
			resp.ContentType = http.DetectContentType(data)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.ContentType = workspaceTextContentType(resolved, data)
	resp.Content = string(data)
	writeJSON(w, http.StatusOK, resp)
}

func serveWorkspaceDiff(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionIDPattern.MatchString(id) {
		gatewayError(w, r, http.StatusBadRequest, "invalid_session_id", "invalid session id", "id", id)
		return
	}
	if !d.Health() {
		gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
		return
	}
	info, err := resolveWorkspaceSession(d, r, id)
	if err != nil {
		writeWorkspaceResolveError(w, r, id, err)
		return
	}
	stale, pinnedRoot := workspaceHandleStale(w, r, info)
	if stale {
		return
	}
	root := pinnedRootOrCurrent(pinnedRoot, info.workspaceRoot)
	rel := r.URL.Query().Get("path")
	if rel != "" {
		if _, err := GuardWorkspacePath(root, rel); err != nil {
			gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
			return
		}
	}
	ctx, cancel := rpcContext(r)
	defer cancel()
	result := gitlib.DiffHeadVsWorktree(ctx, root, rel)
	writeJSON(w, http.StatusOK, workspaceDiffResponse{
		Outcome: string(result.Outcome),
		Diff:    result.Diff,
	})
}

type workspaceSession struct {
	sessionID       string
	frameGeneration int
	workspaceRoot   string
}

func resolveWorkspaceSession(d *DaemonClient, r *http.Request, id string) (workspaceSession, error) {
	ctx, cancel := rpcContext(r)
	defer cancel()
	resp, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventListSessions,
		Payload: json.RawMessage("{}"),
	})
	if err != nil {
		return workspaceSession{}, err
	}
	rs, ok := resp.(proto.RespSessions)
	if !ok {
		return workspaceSession{}, errors.New("unexpected response type")
	}
	for i := range rs.Sessions {
		if rs.Sessions[i].ID == id {
			si := rs.Sessions[i]
			if si.WorkspaceRoot == "" {
				return workspaceSession{}, errSessionNotFound
			}
			return workspaceSession{
				sessionID:       si.ID,
				frameGeneration: si.FrameGeneration,
				workspaceRoot:   si.WorkspaceRoot,
			}, nil
		}
	}
	return workspaceSession{}, errSessionNotFound
}

func pinnedRootOrCurrent(pinned, current string) string {
	if pinned != "" {
		return pinned
	}
	return current
}

func workspaceHandleStale(w http.ResponseWriter, r *http.Request, info workspaceSession) (bool, string) {
	handleStr := r.URL.Query().Get("handle")
	if handleStr == "" {
		return false, r.URL.Query().Get("root")
	}
	pinned, err := strconv.Atoi(handleStr)
	if err != nil {
		gatewayError(w, r, http.StatusBadRequest, "invalid_handle", "invalid handle")
		return true, ""
	}
	if pinned == info.frameGeneration {
		root := r.URL.Query().Get("root")
		if root != "" {
			return false, root
		}
		return false, info.workspaceRoot
	}
	pinnedRoot := r.URL.Query().Get("root")
	writeJSON(w, http.StatusConflict, workspaceHandleStaleError{
		Error:            "handle_stale",
		SessionID:        info.sessionID,
		FrameGeneration:  info.frameGeneration,
		PinnedGeneration: pinned,
		ResolvedRootPath: pinnedRoot,
	})
	return true, ""
}

func writeWorkspaceResolveError(w http.ResponseWriter, r *http.Request, id string, err error) {
	switch {
	case errors.Is(err, errSessionNotFound):
		gatewayError(w, r, http.StatusNotFound, "session_not_found", "session not found", "id", id)
	default:
		handleProtoError(w, r, err)
	}
}

func workspaceSniffBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	if strings.IndexByte(string(sample), 0) >= 0 {
		return true
	}
	return !isUTF8Text(sample)
}

func isUTF8Text(data []byte) bool {
	return utf8.Valid(data)
}

func workspaceTextContentType(path string, data []byte) string {
	if ext := filepath.Ext(path); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	const sniff = 512
	if len(data) > sniff {
		return http.DetectContentType(data[:sniff])
	}
	return http.DetectContentType(data)
}

// workspaceReadOnlyRoutes lists GET workspace endpoints checked by read-only verb tests.
var workspaceReadOnlyRoutes = []string{
	"/api/sessions/{id}/workspace/root-handle",
	"/api/sessions/{id}/workspace/tree",
	"/api/sessions/{id}/workspace/file",
	"/api/sessions/{id}/workspace/diff",
}

// workspaceMutationVerbs are rejected on workspace routes.
var workspaceMutationVerbs = []string{
	http.MethodPut,
	http.MethodPost,
	http.MethodPatch,
	http.MethodDelete,
}
