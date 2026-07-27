package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceRootDisappearance(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")
	root := t.TempDir()
	target := filepath.Join(root, "gone.txt")
	if err := os.WriteFile(target, []byte("dirty-buffer"), 0o600); err != nil {
		t.Fatal(err)
	}

	sendFakeResponse(t, fd, workspaceSessionResp(root, 0))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	req := authedPut(workspaceWriteURL("gone.txt", root), []byte("operator-work"), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("save after root disappearance must not succeed silently, got 200")
	}
	if w.Code < 400 {
		t.Fatalf("status = %d, want typed 4xx/5xx", w.Code)
	}
}
