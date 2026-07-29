package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExactVersionMismatchMatrixNoSideEffects freezes the old/new matrix:
// old-client/new-server and new-client/old-server reject before credential,
// lease, preparation, process, or filesystem side effects.
func TestExactVersionMismatchMatrixNoSideEffects(t *testing.T) {
	t.Parallel()

	type pair struct {
		name           string
		serverExpected string
		clientVersion  string
	}
	pairs := []pair{
		{name: "old-client/new-server", serverExpected: ProtocolVersion, clientVersion: "protocol-1"},
		{name: "new-client/old-server", serverExpected: "protocol-1", clientVersion: ProtocolVersion},
		{name: "missing-client/new-server", serverExpected: ProtocolVersion, clientVersion: ""},
		{name: "garbage-client/new-server", serverExpected: ProtocolVersion, clientVersion: "not-a-version"},
	}

	sideEffectKinds := []RepositorySideEffectKind{
		RepoSideEffectLease,
		RepoSideEffectPreparation,
		RepoSideEffectProcess,
		RepoSideEffectFilesystem,
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			// Counters for every forbidden effect class (credential is modeled as
			// lease issuance; no separate store mutation is permitted).
			counts := map[RepositorySideEffectKind]int{}
			credentialIssued := 0
			res := AdmitRepositoryOperationAgainst(p.serverExpected, p.clientVersion, func(k RepositorySideEffectKind) {
				counts[k]++
				if k == RepoSideEffectLease {
					credentialIssued++
				}
			})
			if res.Admitted {
				t.Fatalf("admitted on mismatch: %+v", res)
			}
			if res.Code != "incompatible_protocol" {
				t.Fatalf("code=%q want incompatible_protocol", res.Code)
			}
			if credentialIssued != 0 {
				t.Fatalf("credential side effect issued=%d", credentialIssued)
			}
			for _, k := range sideEffectKinds {
				if counts[k] != 0 {
					t.Fatalf("side effect %s invoked %d times", k, counts[k])
				}
			}
			// HTTP gate: handler body (which would start prep/process/fs) never runs.
			var handlerCalls int
			h := ProtocolVersionGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				handlerCalls++
				_ = AdmitRepositoryOperation(ProtocolVersion, func(RepositorySideEffectKind) {
					t.Fatal("inner side effect after mismatch gate")
				})
			}))
			req := httptest.NewRequest(http.MethodPost, "/api/repository/leases", strings.NewReader(`{}`))
			if p.clientVersion != "" {
				// For new-client/old-server the gate sees current ProtocolVersion
				// and would admit; the Against helper already covered that pair.
				// Here we only assert the gate rejects non-current tokens.
				if p.clientVersion != ProtocolVersion {
					req.Header.Set(ProtocolVersionHeader, p.clientVersion)
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, req)
					if rec.Code != http.StatusUpgradeRequired || handlerCalls != 0 {
						t.Fatalf("gate status=%d calls=%d body=%s", rec.Code, handlerCalls, rec.Body.String())
					}
					if !strings.Contains(rec.Body.String(), "incompatible_protocol") {
						t.Fatalf("body=%s", rec.Body.String())
					}
				}
			} else {
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusUpgradeRequired || handlerCalls != 0 {
					t.Fatalf("missing version status=%d calls=%d", rec.Code, handlerCalls)
				}
			}
		})
	}

	// Equal versions admit and may run the first side effect exactly once.
	var equalInvoked []RepositorySideEffectKind
	ok := AdmitRepositoryOperationAgainst(ProtocolVersion, ProtocolVersion, func(k RepositorySideEffectKind) {
		equalInvoked = append(equalInvoked, k)
	})
	if !ok.Admitted || len(equalInvoked) != 1 || equalInvoked[0] != RepoSideEffectLease {
		t.Fatalf("equal admit: %+v invoked=%v", ok, equalInvoked)
	}
}

// TestRegisteredRouteMismatchMatrixNoSideEffects is the route-level matrix:
// production newMux rejects old/new/missing/garbage clients before lease
// mutation. Complements AdmitRepositoryOperationAgainst unit coverage.
func TestRegisteredRouteMismatchMatrixNoSideEffects(t *testing.T) {
	t.Parallel()
	// Present here so the mismatch matrix file cannot be green while only unit
	// admission helpers are exercised (production path lives in repository_route_mismatch_test.go).
	runRegisteredLeaseRoutesProtocolMismatchNoSideEffects(t)
}
