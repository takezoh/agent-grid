package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/config"
)

const leaseSentinel = "SENTINEL_LEASE_TOKEN_7f3a9c"

func TestLeaseStoreOneUseTTLCapacityRevoke(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	s := newLeaseStore()
	s.now = func() time.Time { return fixed }
	for i := 0; i < maxLeasesPerHost; i++ {
		if _, f := s.Issue("host-a", "r1", "o", "n", leaseSentinel, "g1"); f != nil {
			t.Fatal(i, f)
		}
	}
	if _, f := s.Issue("host-a", "r1", "o", "n", leaseSentinel, "g1"); f == nil || f.Code != "lease_capacity_exceeded" {
		t.Fatal(f)
	}
	if _, f := s.Issue("host-b", "r1", "o", "n", leaseSentinel, "g1"); f != nil {
		t.Fatal(f)
	}
	s2 := newLeaseStore()
	s2.now = func() time.Time { return fixed }
	issue, _ := s2.Issue("h", "rid", "acme", "widgets", leaseSentinel, "gen-1")
	c1, f := s2.Consume(issue.LeaseID)
	if f != nil || c1.Token != leaseSentinel {
		t.Fatal(c1, f)
	}
	if _, f := s2.Consume(issue.LeaseID); f == nil || f.Code != "lease_replay" {
		t.Fatal(f)
	}
	issue2, _ := s2.Issue("h", "rid", "acme", "widgets", leaseSentinel, "gen-1")
	s2.now = func() time.Time { return fixed.Add(leaseTTL + time.Second) }
	if _, f := s2.Consume(issue2.LeaseID); f == nil || f.Code != "lease_expired" {
		t.Fatal(f)
	}
	s3 := newLeaseStore()
	iss, _ := s3.Issue("h", "rid", "o", "n", leaseSentinel, "g")
	s3.Revoke(iss.LeaseID)
	if _, f := s3.Consume(iss.LeaseID); f == nil {
		t.Fatal("revoked")
	}
	s3.Clear()
	if s3.UnconsumedCount("h") != 0 {
		t.Fatal("clear")
	}
	s4 := newLeaseStore()
	_, _ = s4.Issue("h", "rid", "o", "n", leaseSentinel, "old")
	s4.RevokeGeneration("h", "new")
	if s4.UnconsumedCount("h") != 0 {
		t.Fatal("gen")
	}
}

// TestEphemeralCredentialLeaseContract: gates before lease side effects.
// rejects counterexample: no-auth / insecure / version mismatch minting a lease
func TestEphemeralCredentialLeaseContract(t *testing.T) {
	t.Parallel()
	leases := newLeaseStore()
	deps := muxDependencies{leases: leases, sessionConfigLoader: func() (*config.Config, error) { return nil, nil }}
	inner := http.NewServeMux()
	registerRepositoryRoutes(inner, deps)
	h := ProtocolVersionGate(inner)
	body := `{"host_id":"h1","repository_id":"1","owner":"o","name":"n","token":"` + leaseSentinel + `","token_generation":"g"}`
	post := func(handler http.Handler, ver, addr string, tlsOn bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/repository/leases", strings.NewReader(body))
		req.Header.Set(ProtocolVersionHeader, ver)
		req.RemoteAddr = addr
		if tlsOn {
			req.TLS = &tls.ConnectionState{}
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	depsNo := muxDependencies{leases: newLeaseStore(), noAuth: true}
	innerNo := http.NewServeMux()
	registerRepositoryRoutes(innerNo, depsNo)
	if rec := post(ProtocolVersionGate(innerNo), ProtocolVersion, "127.0.0.1:9", false); rec.Code != http.StatusForbidden || depsNo.leases.UnconsumedCount("h1") != 0 {
		t.Fatalf("no-auth %d", rec.Code)
	}
	if rec := post(h, ProtocolVersion, "8.8.8.8:9", false); rec.Code != http.StatusForbidden || leases.UnconsumedCount("h1") != 0 {
		t.Fatalf("insecure %d", rec.Code)
	}
	if rec := post(h, "protocol-legacy", "127.0.0.1:9", false); rec.Code != http.StatusUpgradeRequired || leases.UnconsumedCount("h1") != 0 {
		t.Fatalf("mismatch %d", rec.Code)
	}
	rec := post(h, ProtocolVersion, "127.0.0.1:9", false)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), leaseSentinel) {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var issue LeaseIssue
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil || issue.LeaseID == "" {
		t.Fatal(err)
	}
	if rec := post(h, ProtocolVersion, "8.8.8.8:9", true); rec.Code != http.StatusCreated {
		t.Fatalf("tls %d", rec.Code)
	}
	leases2 := newLeaseStore()
	mux := newMux(&DaemonClient{}, "secret", false, muxDependencies{leases: leases2, sessionConfigLoader: func() (*config.Config, error) { return nil, nil }})
	req := httptest.NewRequest(http.MethodPost, "/api/repository/leases", bytes.NewBufferString(body))
	req.Header.Set(ProtocolVersionHeader, ProtocolVersion)
	req.RemoteAddr = "127.0.0.1:1"
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || leases2.UnconsumedCount("h1") != 0 {
		t.Fatalf("unauth %d", rec.Code)
	}
}
