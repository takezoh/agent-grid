package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/host/proto"
	stateview "github.com/takezoh/agent-grid/host/state/view"
)

func workspaceTestRoutes(sessionID string) []string {
	routes := make([]string, len(workspaceReadOnlyRoutes))
	for i, tmpl := range workspaceReadOnlyRoutes {
		route := strings.ReplaceAll(tmpl, "{id}", sessionID)
		if strings.HasSuffix(route, "/file") || strings.HasSuffix(route, "/diff") {
			route += "?path=foo.txt&handle=0"
		}
		routes[i] = route
	}
	return routes
}

func workspaceSessionResp(root string, gen int) proto.RespSessions {
	return workspaceSessionRespProject(root, gen, "")
}

// pumpWorkspaceSessions answers every daemon ListSessions RPC with the same
// workspace session fixture until the pipe closes (test cleanup).
func pumpWorkspaceSessions(t *testing.T, fd *fakeDaemon, root string, gen int) {
	t.Helper()
	go fd.pumpResponses(func(reqID string) {
		fd.sendResp(reqID, workspaceSessionResp(root, gen))
	})
}

func workspaceSessionRespProject(root string, gen int, project string) proto.RespSessions {
	return proto.RespSessions{
		Sessions: []proto.SessionInfo{
			{
				ID:              "ws1",
				Project:         project,
				Command:         "claude",
				WorkspaceRoot:   root,
				FrameGeneration: gen,
				View:            stateview.View{},
			},
		},
	}
}

func TestWorkspaceReadOnlyVerbs(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")
	routes := workspaceTestRoutes("ws1")
	for _, route := range routes {
		for _, verb := range workspaceMutationVerbs {
			t.Run(verb+" "+route, func(t *testing.T) {
				// PUT /workspace/file is the editor write endpoint (supersedes
				// viewer no-write for that single verb); all other mutations stay rejected.
				if verb == http.MethodPut && strings.Contains(route, "/workspace/file") {
					return
				}
				req := httptest.NewRequest(verb, route, nil)
				req.Header.Set("Authorization", "Bearer tok")
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)
				if w.Code == http.StatusOK || w.Code == http.StatusCreated {
					t.Fatalf("mutation verb %s accepted on %s: %d", verb, route, w.Code)
				}
			})
		}
	}
}

func TestWorkspaceReadOnlyVerbs_StrictReadRoutes(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")
	strictRoutes := []string{
		"/api/sessions/ws1/workspace/root-handle",
		"/api/sessions/ws1/workspace/tree",
		"/api/sessions/ws1/workspace/diff?path=foo.txt&handle=0",
	}
	for _, route := range strictRoutes {
		for _, verb := range workspaceMutationVerbs {
			t.Run(verb+" "+route, func(t *testing.T) {
				req := httptest.NewRequest(verb, route, nil)
				req.Header.Set("Authorization", "Bearer tok")
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)
				if w.Code == http.StatusOK || w.Code == http.StatusCreated {
					t.Fatalf("mutation verb %s accepted on %s: %d", verb, route, w.Code)
				}
			})
		}
	}
}

func TestWorkspacePathGuardHTTP(t *testing.T) {
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

	okReq := authedGet("/api/sessions/ws1/workspace/file?path=ok.txt&handle=0&handle_session=ws1&root=" + url.QueryEscape(root))
	okW := httptest.NewRecorder()
	mux.ServeHTTP(okW, okReq)
	if okW.Code != http.StatusOK {
		t.Fatalf("in-root: got %d %s", okW.Code, okW.Body.String())
	}
	var fileResp workspaceFileResponse
	if err := json.Unmarshal(okW.Body.Bytes(), &fileResp); err != nil {
		t.Fatal(err)
	}
	if fileResp.Content != "ok" {
		t.Fatalf("content = %q", fileResp.Content)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	badReq := authedGet("/api/sessions/ws1/workspace/file?path=escape/secret.txt&handle=0&handle_session=ws1&root=" + url.QueryEscape(root))
	badW := httptest.NewRecorder()
	mux.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusNotFound {
		t.Fatalf("escape: got %d, want 404", badW.Code)
	}
	if badW.Body.String() != "" && strings.Contains(badW.Body.String(), "secret") {
		t.Fatal("response leaked outside bytes")
	}
}

func TestWorkspaceRootHandlePinning(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	rootA := t.TempDir()
	rootB := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "marker-a.txt"), []byte("A"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "marker-b.txt"), []byte("B"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(rootA, 0))
	handleReq := authedGet("/api/sessions/ws1/workspace/root-handle")
	handleW := httptest.NewRecorder()
	mux.ServeHTTP(handleW, handleReq)
	if handleW.Code != http.StatusOK {
		t.Fatalf("root-handle: %d %s", handleW.Code, handleW.Body.String())
	}
	var handle workspaceRootHandle
	if err := json.Unmarshal(handleW.Body.Bytes(), &handle); err != nil {
		t.Fatal(err)
	}
	if handle.ResolvedRootPath != rootA || handle.FrameGeneration != 0 {
		t.Fatalf("handle = %+v, want rootA gen 0", handle)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(rootA, 0))
	treeReq := authedGet("/api/sessions/ws1/workspace/tree?handle=0&handle_session=ws1&root=" + url.QueryEscape(rootA))
	treeW := httptest.NewRecorder()
	mux.ServeHTTP(treeW, treeReq)
	if treeW.Code != http.StatusOK {
		t.Fatalf("pinned tree: %d %s", treeW.Code, treeW.Body.String())
	}

	sendFakeResponse(t, fd, workspaceSessionResp(rootA, 1))
	staleReq := authedGet("/api/sessions/ws1/workspace/tree?handle=0&handle_session=ws1&root=" + url.QueryEscape(rootA))
	staleW := httptest.NewRecorder()
	mux.ServeHTTP(staleW, staleReq)
	if staleW.Code != http.StatusConflict {
		t.Fatalf("stale handle: got %d, want 409", staleW.Code)
	}
	var stale workspaceHandleStaleError
	if err := json.Unmarshal(staleW.Body.Bytes(), &stale); err != nil {
		t.Fatal(err)
	}
	if stale.Error != "handle_stale" || stale.ResolvedRootPath != rootA {
		t.Fatalf("stale body = %+v", stale)
	}
}

// TestWorkspaceHandleSessionMismatch covers
// adr-20260714-workspace-handle-session-binding: a handle_session query value
// that differs from the URL session must be rejected as invalid_handle
// before any filesystem access, even though the URL id resolves fine on its
// own and its generation/root would otherwise validate.
func TestWorkspaceHandleSessionMismatch(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	req := authedGet(
		"/api/sessions/ws1/workspace/file?path=secret.txt&handle=0&handle_session=other-session&root=" + url.QueryEscape(root),
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("handle_session mismatch: got %d %s, want 400", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "invalid_handle" {
		t.Fatalf("error = %q, want invalid_handle", body.Error)
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("response leaked file contents on handle_session mismatch")
	}
}

// TestWorkspaceHandleRootMismatch covers the same ADR: a root query value
// that disagrees with the server-resolved current root — even though the
// generation still matches — must be rejected as invalid_handle rather than
// silently trusting the client-supplied root.
func TestWorkspaceHandleRootMismatch(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	actualRoot := t.TempDir()
	otherRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(otherRoot, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(actualRoot, 0))
	req := authedGet(
		"/api/sessions/ws1/workspace/file?path=secret.txt&handle=0&handle_session=ws1&root=" + url.QueryEscape(otherRoot),
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("root mismatch: got %d %s, want 400", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "invalid_handle" {
		t.Fatalf("error = %q, want invalid_handle", body.Error)
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("response leaked file contents on root mismatch")
	}
}

func TestWorkspaceHandleRequiresCompleteBindingTuple(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []string{
		"/api/sessions/ws1/workspace/file?path=secret.txt&handle=0&root=" + url.QueryEscape(root),
		"/api/sessions/ws1/workspace/file?path=secret.txt&handle=0&handle_session=ws1",
	}
	for _, route := range tests {
		sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, authedGet(route))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("incomplete handle %q: got %d %s, want 400", route, w.Code, w.Body.String())
		}
		var body workspaceInvalidHandleError
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error != "invalid_handle" || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("incomplete handle response = %s", w.Body.String())
		}
	}
}

func TestDiffBase(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	nonGit := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonGit, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sendFakeResponse(t, fd, workspaceSessionResp(nonGit, 0))
	req := authedGet("/api/sessions/ws1/workspace/diff?path=f.txt&handle=0&handle_session=ws1&root=" + url.QueryEscape(nonGit))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("non-git diff: %d %s", w.Code, w.Body.String())
	}
	var resp workspaceDiffResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "not_a_repo" {
		t.Fatalf("outcome = %q, want not_a_repo", resp.Outcome)
	}

	corruptDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(corruptDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptDir, ".git", "HEAD"), []byte("not-git"), 0o644); err != nil {
		t.Fatal(err)
	}
	sendFakeResponse(t, fd, workspaceSessionResp(corruptDir, 0))
	req = authedGet("/api/sessions/ws1/workspace/diff?handle=0&handle_session=ws1&root=" + url.QueryEscape(corruptDir))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("corrupt diff: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "git_metadata_corrupted" {
		t.Fatalf("outcome = %q, want git_metadata_corrupted", resp.Outcome)
	}
}

func TestWorkspaceRootHandleResolutionPriority(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	worktree := filepath.Join(project, "worktree")
	startDir := filepath.Join(project, "start")
	for _, dir := range []string{worktree, startDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name string
		root string
	}{
		{name: "project", root: project},
		{name: "start_dir", root: startDir},
		{name: "worktree", root: worktree},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, fd := newDaemonPair(t)
			mux := NewMux(d, "tok")
			sendFakeResponse(t, fd, workspaceSessionResp(tc.root, 0))
			req := authedGet("/api/sessions/ws1/workspace/root-handle")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d", w.Code)
			}
			var handle workspaceRootHandle
			if err := json.Unmarshal(w.Body.Bytes(), &handle); err != nil {
				t.Fatal(err)
			}
			if handle.ResolvedRootPath != tc.root {
				t.Fatalf("resolved_root_path = %q, want %q", handle.ResolvedRootPath, tc.root)
			}
		})
	}
}
