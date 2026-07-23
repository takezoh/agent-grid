package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/host/runtime"
)

func TestWorkspaceWriteAuditToolLogTail(t *testing.T) {
	dataDir := t.TempDir()
	project := "/home/dev/myproj"
	log := runtime.NewFileToolLog(dataDir)
	SetWorkspaceOperatorAuditor(NewFileToolLogOperatorAuditorForSession(log, "claude", "", project))
	t.Cleanup(func() { SetWorkspaceOperatorAuditor(nil) })

	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	target := filepath.Join(root, "audit-tail.txt")
	if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionRespProject(root, 0, project))
	req := authedPut(workspaceWriteURL("audit-tail.txt", root), []byte("after"), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	slug := toolLogProjectSlug(project)
	logPath := filepath.Join(dataDir, "claude", "tool-logs", slug+".jsonl")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tool log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("tool log lines = %d, want 1", len(lines))
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["actor"] != runtime.ToolLogActorOperator {
		t.Fatalf("actor = %v, want operator", rec["actor"])
	}
	if rec["workspace_relative_path"] != "audit-tail.txt" {
		t.Fatalf("path = %v", rec["workspace_relative_path"])
	}
}

func TestToolLogSlugHelpers(t *testing.T) {
	t.Parallel()
	if got := toolLogNamespaceFromCommand("codex resume", ""); got != "codex" {
		t.Fatalf("namespace = %q, want codex", got)
	}
	if got := toolLogNamespaceFromCommand("claude", "gemini"); got != "gemini" {
		t.Fatalf("root_driver wins: got %q", got)
	}
	if got := toolLogProjectSlug("/home/dev/proj"); got != "-home-dev-proj" {
		t.Fatalf("project slug = %q", got)
	}
}
