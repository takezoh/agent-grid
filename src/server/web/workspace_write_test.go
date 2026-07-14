package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func authedPut(path string, body []byte, extraHeaders map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	for k, v := range extraHeaders {
		r.Header.Set(k, v)
	}
	return r
}

func workspaceWriteURL(path string) string {
	return "/api/sessions/ws1/workspace/file?path=" + path + "&handle=0"
}

func TestWorkspaceWriteHandlerHappyPath(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	target := filepath.Join(root, "edit.txt")
	if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req := authedPut(workspaceWriteURL("edit.txt"), []byte("after"), map[string]string{
		"If-Unmodified-Since": formatWorkspaceMtime(fi.ModTime()),
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp workspaceWriteSuccessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Path != "edit.txt" || resp.UpdatedMtime == "" {
		t.Fatalf("resp = %+v", resp)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "after" {
		t.Fatalf("disk = %q, want after", data)
	}
}

func TestWorkspaceWriteHandlerPathGuard(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req := authedPut(workspaceWriteURL("escape/secret.txt"), []byte("hack"), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("escape: got %d, want 404", w.Code)
	}
}

func TestWorkspaceWriteHandlerBodyCap(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cap.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	over := make([]byte, workspaceWriteBodyCap+1)
	req := authedPut(workspaceWriteURL("cap.txt"), over, nil)
	req.ContentLength = int64(len(over))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("content-length cap: got %d, want 413", w.Code)
	}
	var body workspaceWriteOversizeBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "oversize_body" {
		t.Fatalf("error = %q", body.Error)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req = authedPut(workspaceWriteURL("cap.txt"), over, nil)
	req.ContentLength = -1
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("streaming cap: got %d, want 413", w.Code)
	}
}

func TestWorkspaceWriteHandlerPreconditionFailed(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	target := filepath.Join(root, "stale.txt")
	if err := os.WriteFile(target, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req := authedPut(workspaceWriteURL("stale.txt"), []byte("mine"), map[string]string{
		"If-Unmodified-Since": formatWorkspaceMtime(staleTime),
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", w.Code)
	}
	var resp workspaceWritePreconditionFailed
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "precondition_failed" || resp.CurrentMtime == "" {
		t.Fatalf("resp = %+v", resp)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "current" {
		t.Fatalf("disk changed: %q", data)
	}
}

func TestWorkspaceWriteHandlerAuth(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")
	req := httptest.NewRequest(http.MethodPut, workspaceWriteURL("x.txt"), strings.NewReader("x"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestWorkspaceWriteAtomicity(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		d, fd := newDaemonPair(t)
		mux := NewMux(d, "tok")
		root := t.TempDir()
		target := filepath.Join(root, "atomic.txt")
		if err := os.WriteFile(target, []byte("keep-or-replace"), 0o600); err != nil {
			t.Fatal(err)
		}
		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		req := authedPut(workspaceWriteURL("atomic.txt"), []byte("replaced"), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "replaced" {
			t.Fatalf("disk = %q", data)
		}
	})

	t.Run("syscall failure leaves original", func(t *testing.T) {
		d, fd := newDaemonPair(t)
		mux := NewMux(d, "tok")
		root := t.TempDir()
		sub := filepath.Join(root, "locked")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(sub, "file.txt")
		if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(sub, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		req := authedPut(workspaceWriteURL("locked/file.txt"), []byte("new"), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code < 500 {
			t.Fatalf("status = %d, want typed 5xx", w.Code)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Fatalf("disk = %q, want original", data)
		}
	})

	t.Run("inconclusive restores original", func(t *testing.T) {
		prev := workspaceAtomicWriteFn
		workspaceAtomicWriteFn = func(target string, data []byte) (time.Time, error) {
			prior, hadPrior, err := workspaceFileSnapshot(target)
			if err != nil {
				return time.Time{}, err
			}
			if err := os.WriteFile(target, []byte("wrong-size"), 0o600); err != nil { //nolint:gosec // test fixture
				return time.Time{}, err
			}
			restoreWorkspaceFile(target, prior, hadPrior)
			return time.Time{}, &workspaceAtomicWriteError{kind: atomicWriteInconclusive, err: errors.New("size mismatch")}
		}
		t.Cleanup(func() { workspaceAtomicWriteFn = prev })

		d, fd := newDaemonPair(t)
		mux := NewMux(d, "tok")
		root := t.TempDir()
		target := filepath.Join(root, "verify.txt")
		if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}

		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		req := authedPut(workspaceWriteURL("verify.txt"), []byte("new-bytes"), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Fatalf("disk = %q, want original preserved by hook restore path", data)
		}
	})
}

func TestWorkspaceWritePermissionParity(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req := authedPut(workspaceWriteURL(".env"), []byte("SECRET=2"), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf(".env write: got %d %s", w.Code, w.Body.String())
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req = authedPut(workspaceWriteURL("escape/secret.txt"), []byte("hack"), nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("out-of-root: got %d, want 404", w.Code)
	}
}

type recordingOperatorAuditor struct {
	calls []struct {
		sessionID string
		relPath   string
	}
	err error
}

func (a *recordingOperatorAuditor) EmitOperatorWrite(sessionID, relPath string) error {
	a.calls = append(a.calls, struct {
		sessionID string
		relPath   string
	}{sessionID, relPath})
	return a.err
}

func TestWorkspaceWriteAudit(t *testing.T) {
	t.Run("happy_path", func(t *testing.T) {
		auditor := &recordingOperatorAuditor{}
		SetWorkspaceOperatorAuditor(auditor)
		t.Cleanup(func() { SetWorkspaceOperatorAuditor(nil) })

		d, fd := newDaemonPair(t)
		mux := NewMux(d, "tok")
		root := t.TempDir()
		target := filepath.Join(root, "audit.txt")
		if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}

		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		req := authedPut(workspaceWriteURL("audit.txt"), []byte("after"), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if len(auditor.calls) != 1 {
			t.Fatalf("EmitOperatorWrite calls = %d, want 1", len(auditor.calls))
		}
		if auditor.calls[0].sessionID != "ws1" || auditor.calls[0].relPath != "audit.txt" {
			t.Fatalf("calls = %+v", auditor.calls)
		}
	})

	t.Run("audit_emit_failed", func(t *testing.T) {
		auditor := &recordingOperatorAuditor{err: errors.New("append failed")}
		SetWorkspaceOperatorAuditor(auditor)
		t.Cleanup(func() { SetWorkspaceOperatorAuditor(nil) })

		d, fd := newDaemonPair(t)
		mux := NewMux(d, "tok")
		root := t.TempDir()
		target := filepath.Join(root, "fail-audit.txt")
		if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}

		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		req := authedPut(workspaceWriteURL("fail-audit.txt"), []byte("after"), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
		var resp workspaceWriteAuditFailed
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Error != "audit_emit_failed" {
			t.Fatalf("error = %q, want audit_emit_failed", resp.Error)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "after" {
			t.Fatalf("disk = %q, want after (write succeeded before audit failure)", data)
		}
	})
}

func TestWorkspaceWriteResourceBound(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bound.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	body := make([]byte, workspaceWriteBodyCap+512)
	req := authedPut(workspaceWriteURL("bound.txt"), body, nil)
	req.ContentLength = int64(len(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 before full body accepted", w.Code)
	}
	if got, err := io.ReadAll(w.Body); err == nil && len(got) == 0 {
		t.Fatal("expected JSON oversize body")
	}
}
