package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/services/host/config"
)

// TestRegisteredLeaseRoutesProtocolMismatchNoSideEffects drives the production
// newMux path (TokenAuth → ProtocolVersionGate → registerRepositoryRoutes).
// Mismatch/missing protocol versions must reject Issue and Revoke before any
// lease mutation. Global ProtocolVersionGate is the single admission gate;
// handlers do not re-implement a second incompatible check.
func TestRegisteredLeaseRoutesProtocolMismatchNoSideEffects(t *testing.T) {
	t.Parallel()
	runRegisteredLeaseRoutesProtocolMismatchNoSideEffects(t)
}

func runRegisteredLeaseRoutesProtocolMismatchNoSideEffects(t *testing.T) {
	t.Helper()
	leases := newLeaseStore()
	mux := newMux(&DaemonClient{}, "secret", false, muxDependencies{
		leases:              leases,
		sessionConfigLoader: func() (*config.Config, error) { return nil, nil },
	})

	body := `{"host_id":"h-route","repository_id":"1","owner":"o","name":"n","token":"tok","token_generation":"g"}`
	postLease := func(ver string, setHeader bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/repository/leases", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		if setHeader {
			req.Header.Set(ProtocolVersionHeader, ver)
		}
		req.RemoteAddr = "127.0.0.1:9"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	// Seed one lease under exact version for revoke-path mismatch.
	recOK := postLease(ProtocolVersion, true)
	if recOK.Code != http.StatusCreated {
		t.Fatalf("seed create status=%d body=%s", recOK.Code, recOK.Body.String())
	}
	var issue LeaseIssue
	if err := json.Unmarshal(recOK.Body.Bytes(), &issue); err != nil || issue.LeaseID == "" {
		t.Fatalf("seed issue: %v %s", err, recOK.Body.String())
	}
	before := leases.UnconsumedCount("h-route")
	if before != 1 {
		t.Fatalf("seed count=%d", before)
	}

	cases := []struct {
		name      string
		ver       string
		setHeader bool
	}{
		{name: "old-client", ver: "protocol-1", setHeader: true},
		{name: "garbage", ver: "not-a-version", setHeader: true},
		{name: "missing", ver: "", setHeader: false},
		{name: "empty-header", ver: "", setHeader: true},
	}
	for _, tc := range cases {
		t.Run("issue/"+tc.name, func(t *testing.T) {
			rec := postLease(tc.ver, tc.setHeader)
			if rec.Code != http.StatusUpgradeRequired {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "incompatible_protocol") {
				t.Fatalf("body=%s", rec.Body.String())
			}
			if leases.UnconsumedCount("h-route") != before {
				t.Fatalf("lease side effect on mismatch: count=%d", leases.UnconsumedCount("h-route"))
			}
		})
		t.Run("revoke/"+tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/repository/leases/"+issue.LeaseID, nil)
			req.Header.Set("Authorization", "Bearer secret")
			if tc.setHeader {
				req.Header.Set(ProtocolVersionHeader, tc.ver)
			}
			req.RemoteAddr = "127.0.0.1:9"
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusUpgradeRequired {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			// Lease still consumable → revoke did not run.
			if leases.UnconsumedCount("h-route") != before {
				t.Fatalf("revoke side effect on mismatch")
			}
		})
		t.Run("revoke-host/"+tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/repository/leases/revoke-host",
				bytes.NewBufferString(`{"host_id":"h-route","token_generation":"g"}`))
			req.Header.Set("Authorization", "Bearer secret")
			if tc.setHeader {
				req.Header.Set(ProtocolVersionHeader, tc.ver)
			}
			req.RemoteAddr = "127.0.0.1:9"
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusUpgradeRequired {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if leases.UnconsumedCount("h-route") != before {
				t.Fatalf("revoke-host side effect on mismatch")
			}
		})
	}

	// Equal version still admits through the same registered route.
	rec := postLease(ProtocolVersion, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("equal version create: %d %s", rec.Code, rec.Body.String())
	}
	if leases.UnconsumedCount("h-route") != before+1 {
		t.Fatalf("want +1 lease after admit")
	}

	// TLS non-loopback also works when protocol matches.
	req := httptest.NewRequest(http.MethodPost, "/api/repository/leases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	req.RemoteAddr = "8.8.8.8:9"
	req.TLS = &tls.ConnectionState{}
	recTLS := httptest.NewRecorder()
	mux.ServeHTTP(recTLS, req)
	if recTLS.Code != http.StatusCreated {
		t.Fatalf("tls admit: %d", recTLS.Code)
	}
}
