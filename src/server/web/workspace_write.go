package web

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const workspaceWriteBodyCap = 1 << 20 // 1 MiB

type workspaceWriteSuccessResponse struct {
	UpdatedMtime string `json:"updated_mtime"`
	Path         string `json:"path"`
}

type workspaceWritePreconditionFailed struct {
	Error        string `json:"error"`
	CurrentMtime string `json:"current_mtime"`
}

type workspaceWriteOversizeBody struct {
	Error string `json:"error"`
}

type workspaceWriteAuditFailed struct {
	Error string `json:"error"`
}

type atomicWriteOutcome int

const (
	atomicWriteFailure atomicWriteOutcome = iota
	atomicWriteUnknown
	atomicWriteInconclusive
)

type workspaceAtomicWriteError struct {
	kind atomicWriteOutcome
	err  error
}

func (e *workspaceAtomicWriteError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "workspace atomic write failed"
}

// workspaceAtomicWriteFn is swapped by tests to simulate syscall outcome partitions.
var workspaceAtomicWriteFn = atomicWriteWorkspaceFile

func handleWorkspaceFileWrite(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveWorkspaceFileWrite(d, w, r)
	}
}

func serveWorkspaceFileWrite(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
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
	if !validateWorkspaceHandle(w, r, id, info) {
		return
	}
	root := info.workspaceRoot
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
	if r.ContentLength > workspaceWriteBodyCap {
		writeJSON(w, http.StatusRequestEntityTooLarge, workspaceWriteOversizeBody{Error: "oversize_body"})
		return
	}
	body, err := readWorkspaceWriteBody(w, r)
	if err != nil {
		return
	}
	if err := checkWorkspaceWritePrecondition(w, r, resolved); err != nil {
		return
	}
	updated, err := workspaceAtomicWriteFn(resolved, body)
	if err != nil {
		writeWorkspaceAtomicWriteError(w, r, err)
		return
	}
	if err := emitOperatorWriteAudit(id, rel); err != nil {
		writeJSON(w, http.StatusInternalServerError, workspaceWriteAuditFailed{Error: "audit_emit_failed"})
		return
	}
	writeJSON(w, http.StatusOK, workspaceWriteSuccessResponse{
		UpdatedMtime: formatWorkspaceMtime(updated),
		Path:         rel,
	})
}

func readWorkspaceWriteBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, workspaceWriteBodyCap)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, workspaceWriteOversizeBody{Error: "oversize_body"})
			return nil, errOversizeBody
		}
		gatewayError(w, r, http.StatusBadRequest, "bad_request", "bad request body", "err", err)
		return nil, err
	}
	return data, nil
}

var errOversizeBody = errors.New("oversize body")

func checkWorkspaceWritePrecondition(w http.ResponseWriter, r *http.Request, resolved string) error {
	header := r.Header.Get("If-Unmodified-Since")
	if header == "" {
		return nil
	}
	fi, err := os.Stat(resolved)
	if err != nil || fi.IsDir() {
		gatewayError(w, r, http.StatusNotFound, "workspace_path_not_found", "path not found")
		return errWorkspacePathMissing
	}
	headerTime, err := http.ParseTime(header)
	if err != nil {
		gatewayError(w, r, http.StatusBadRequest, "invalid_if_unmodified_since", "invalid If-Unmodified-Since")
		return err
	}
	if workspaceMtimeEqual(fi.ModTime(), headerTime) {
		return nil
	}
	writeJSON(w, http.StatusPreconditionFailed, workspaceWritePreconditionFailed{
		Error:        "precondition_failed",
		CurrentMtime: formatWorkspaceMtime(fi.ModTime()),
	})
	return errPreconditionFailed
}

var (
	errWorkspacePathMissing = errors.New("workspace path missing")
	errPreconditionFailed   = errors.New("precondition failed")
)

func formatWorkspaceMtime(t time.Time) string {
	return t.UTC().Format(http.TimeFormat)
}

func workspaceMtimeEqual(fileTime, headerTime time.Time) bool {
	return fileTime.UTC().Truncate(time.Second).Equal(headerTime.UTC().Truncate(time.Second))
}

func writeWorkspaceAtomicWriteError(w http.ResponseWriter, r *http.Request, err error) {
	var awe *workspaceAtomicWriteError
	if errors.As(err, &awe) {
		switch awe.kind {
		case atomicWriteInconclusive:
			gatewayAPIError(w, r, http.StatusInternalServerError, "atomic_write_inconclusive", "atomic write inconclusive", "err", awe.err)
		case atomicWriteUnknown:
			gatewayAPIError(w, r, http.StatusBadGateway, "atomic_write_unknown", "atomic write failed", "err", awe.err)
		default:
			gatewayAPIError(w, r, http.StatusBadGateway, "atomic_write_failed", "atomic write failed", "err", awe.err)
		}
		return
	}
	gatewayAPIError(w, r, http.StatusBadGateway, "atomic_write_failed", "atomic write failed", "err", err)
}

func atomicWriteWorkspaceFile(target string, data []byte) (time.Time, error) {
	prior, hadPrior, err := workspaceFileSnapshot(target)
	if err != nil {
		return time.Time{}, &workspaceAtomicWriteError{kind: atomicWriteFailure, err: err}
	}

	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".workspace-write-*")
	if err != nil {
		return time.Time{}, workspaceAtomicWriteFromSyscall(err)
	}
	tmpName := tmp.Name()
	cleanupTmp := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanupTmp()
		return time.Time{}, workspaceAtomicWriteFromSyscall(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return time.Time{}, workspaceAtomicWriteFromSyscall(err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return time.Time{}, workspaceAtomicWriteFromSyscall(err)
	}

	fi, err := os.Stat(target)
	if err != nil {
		restoreWorkspaceFile(target, prior, hadPrior)
		return time.Time{}, &workspaceAtomicWriteError{kind: atomicWriteUnknown, err: err}
	}
	if fi.Size() != int64(len(data)) {
		restoreWorkspaceFile(target, prior, hadPrior)
		return time.Time{}, &workspaceAtomicWriteError{
			kind: atomicWriteInconclusive,
			err:  errors.New("post-rename size mismatch"),
		}
	}
	return fi.ModTime(), nil
}

func workspaceFileSnapshot(target string) ([]byte, bool, error) {
	fi, err := os.Stat(target)
	if err != nil {
		return nil, false, err
	}
	if fi.IsDir() {
		return nil, false, errors.New("target is directory")
	}
	data, err := os.ReadFile(target) //nolint:gosec // caller guards path under workspace root
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func restoreWorkspaceFile(target string, prior []byte, hadPrior bool) {
	if hadPrior {
		_ = os.WriteFile(target, prior, 0o600) //nolint:gosec // restoring guarded workspace path
		return
	}
	_ = os.Remove(target)
}

func workspaceAtomicWriteFromSyscall(err error) error {
	if err == nil {
		return nil
	}
	return &workspaceAtomicWriteError{kind: atomicWriteFailure, err: err}
}
