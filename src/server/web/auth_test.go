package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestTokenAuth(t *testing.T) {
	h := TokenAuth("secret", okHandler())

	cases := []struct {
		name   string
		setup  func(*http.Request)
		status int
	}{
		{"no token", func(*http.Request) {}, http.StatusUnauthorized},
		{"wrong token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }, http.StatusUnauthorized},
		{"bearer header", func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }, http.StatusOK},
		{"query param", func(r *http.Request) { r.URL.RawQuery = "token=secret" }, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setup(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
		})
	}
}

func TestTokenAuthEmptyWantRejects(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?token=anything", nil)
	w := httptest.NewRecorder()
	TokenAuth("", okHandler()).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("empty want should reject, got %d", w.Code)
	}
}
