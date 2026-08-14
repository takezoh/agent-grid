package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func protocolRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequestWithContext(testsContext(), method, target, body)
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	return req
}

func testsContext() context.Context { return context.Background() }

func TestProtocolVersionGateRejectsMissingAndMismatchBeforeHandler(t *testing.T) {
	t.Parallel()
	calls := 0
	handler := ProtocolVersionGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))

	for _, tc := range []struct {
		name    string
		version string
	}{
		{name: "missing"},
		{name: "mismatch", version: "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
			if tc.version != "" {
				req.Header.Set(ProtocolVersionHeader, tc.version)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUpgradeRequired {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
			}
			if !strings.Contains(rec.Body.String(), `"code":"incompatible_protocol"`) {
				t.Fatalf("body = %q", rec.Body.String())
			}
		})
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d, want 0", calls)
	}
}

func TestProtocolVersionGateAdmitsExactVersion(t *testing.T) {
	t.Parallel()
	handler := ProtocolVersionGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, protocolRequest(http.MethodGet, "/api/sessions", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestProtocolVersionGateRunsAfterAuthentication(t *testing.T) {
	t.Parallel()
	handler := TokenAuth("secret", ProtocolVersionGate(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
	)))

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if strings.Contains(rec.Body.String(), ProtocolVersion) {
		t.Fatal("unauthenticated response disclosed protocol version")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("authenticated mismatch status = %d", rec.Code)
	}
}

func TestNoAuthWSProtocolVersionRejectsDuplicateQueryValues(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(
		http.MethodGet,
		"/ws?protocolVersion="+ProtocolVersion+"&protocolVersion="+ProtocolVersion,
		nil,
	)
	actual, supplied, ok := wsProtocolVersion(req)
	if ok {
		t.Fatal("duplicate protocolVersion unexpectedly admitted")
	}
	if !supplied {
		t.Fatal("duplicate protocolVersion must count as a declaration, not as silence")
	}
	if actual != ProtocolVersion+","+ProtocolVersion {
		t.Fatalf("actual = %q", actual)
	}
}
