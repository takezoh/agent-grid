package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/server/session"
)

func TestMuxCreateListStop(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	mux := NewMux(svc, fstest.MapFS{})

	info := decodeInfo(t, doReq(t, mux, http.MethodPost, "/api/sessions",
		`{"command":"sleep 5"}`, http.StatusCreated))
	if info.ID == "" {
		t.Fatal("create returned empty id")
	}

	listBody := doReq(t, mux, http.MethodGet, "/api/sessions", "", http.StatusOK)
	var list []session.Info
	if err := json.Unmarshal([]byte(listBody), &list); err != nil || len(list) != 1 {
		t.Fatalf("list = %q (err %v)", listBody, err)
	}

	doReq(t, mux, http.MethodDelete, "/api/sessions/"+info.ID, "", http.StatusNoContent)
	doReq(t, mux, http.MethodDelete, "/api/sessions/"+info.ID, "", http.StatusNotFound)
}

func TestMuxCreateBadCommand(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	mux := NewMux(svc, fstest.MapFS{})
	doReq(t, mux, http.MethodPost, "/api/sessions", `{"command":""}`, http.StatusBadRequest)
}

func doReq(t *testing.T, h http.Handler, method, path, body string, want int) string {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: status = %d, want %d (body %q)", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.String()
}

func decodeInfo(t *testing.T, body string) session.Info {
	t.Helper()
	var info session.Info
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("decode info: %v (body %q)", err, body)
	}
	return info
}
