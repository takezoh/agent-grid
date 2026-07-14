package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/client/proto"
	stateview "github.com/takezoh/agent-grid/client/state/view"
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
	return proto.RespSessions{
		Sessions: []proto.SessionInfo{
			{
				ID:              "ws1",
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

	okReq := authedGet("/api/sessions/ws1/workspace/file?path=ok.txt&handle=0")
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
	badReq := authedGet("/api/sessions/ws1/workspace/file?path=escape/secret.txt&handle=0")
	badW := httptest.NewRecorder()
	mux.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusNotFound {
		t.Fatalf("escape: got %d, want 404", badW.Code)
	}
	if badW.Body.String() != "" && contains(badW.Body.String(), "secret") {
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
	treeReq := authedGet("/api/sessions/ws1/workspace/tree?handle=0&root=" + rootA)
	treeW := httptest.NewRecorder()
	mux.ServeHTTP(treeW, treeReq)
	if treeW.Code != http.StatusOK {
		t.Fatalf("pinned tree: %d %s", treeW.Code, treeW.Body.String())
	}

	sendFakeResponse(t, fd, workspaceSessionResp(rootB, 1))
	staleReq := authedGet("/api/sessions/ws1/workspace/tree?handle=0&root=" + rootA)
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

func TestDiffBase(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	nonGit := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonGit, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sendFakeResponse(t, fd, workspaceSessionResp(nonGit, 0))
	req := authedGet("/api/sessions/ws1/workspace/diff?path=f.txt&handle=0")
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
	req = authedGet("/api/sessions/ws1/workspace/diff?handle=0")
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

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
