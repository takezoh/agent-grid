package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// TokenAuth wraps h, requiring a bearer token equal to want. The token may be
// supplied as "Authorization: Bearer <token>" or a "token" query parameter (the
// latter for WebSocket connections, which cannot set headers from a browser).
// An empty want rejects every request, so a token must always be configured.
func TokenAuth(want string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want == "" || !tokenOK(r, want) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func tokenOK(r *http.Request, want string) bool {
	got := r.URL.Query().Get("token")
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		got = strings.TrimPrefix(h, "Bearer ")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
