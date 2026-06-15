package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/termvt"
	"github.com/takezoh/agent-reactor/server/session"
)

// Sessions is the session-service surface the HTTP API needs (satisfied by
// *session.Service; a fake is used in tests).
type Sessions interface {
	Create(ctx context.Context, spec session.Spec) (session.Info, error)
	List() []session.Info
	Stop(ctx context.Context, id string) error
	Session(id string) (*termvt.Session, bool)
}

// NewMux builds the HTTP handler: the static web client, the session REST API,
// and the per-session WebSocket attach endpoint. Wrap it with TokenAuth.
func NewMux(svc Sessions, assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.FS(assets)))

	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.List())
	})
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var spec session.Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		info, err := svc.Create(r.Context(), spec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, info)
	})
	mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Stop(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		serveAttach(svc, w, r)
	})
	return mux
}

func serveAttach(svc Sessions, w http.ResponseWriter, r *http.Request) {
	sess, ok := svc.Session(r.URL.Query().Get("session"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	_ = AttachWS(r.Context(), sess, c)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
