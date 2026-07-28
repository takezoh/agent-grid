package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdmitRepositoryOperationMismatchNoSideEffects(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"", "protocol-1", "protocol-99", "not-a-version"} {
		var invoked []RepositorySideEffectKind
		res := AdmitRepositoryOperation(version, func(k RepositorySideEffectKind) {
			invoked = append(invoked, k)
		})
		if res.Admitted || res.Code != "incompatible_protocol" || len(invoked) != 0 {
			t.Fatalf("version %q: %+v invoked=%v", version, res, invoked)
		}
	}
	var invoked []RepositorySideEffectKind
	res := AdmitRepositoryOperation(ProtocolVersion, func(k RepositorySideEffectKind) {
		invoked = append(invoked, k)
	})
	if !res.Admitted || len(invoked) != 1 || invoked[0] != RepoSideEffectLease {
		t.Fatalf("equal version: %+v %v", res, invoked)
	}
}

func TestRepositoryCreateAndGateBeforeHandler(t *testing.T) {
	t.Parallel()
	if _, err := DecodeRepositoryCreateRequest([]byte(`{"intent_id":"i"}`)); err == nil {
		t.Fatal("incomplete")
	}
	raw, _ := json.Marshal(RepositoryCreateRequest{
		IntentID: "i", PreparationID: "p", ServerID: "s", RepositoryID: "1", Owner: "o", Name: "n",
	})
	if got, err := DecodeRepositoryCreateRequest(raw); err != nil || got.RepositoryID != "1" {
		t.Fatal(err)
	}

	var sideEffects int
	handler := ProtocolVersionGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		_ = AdmitRepositoryOperation(ProtocolVersion, func(RepositorySideEffectKind) { sideEffects++ })
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/repository/preparations", nil)
	req.Header.Set(ProtocolVersionHeader, "protocol-legacy")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUpgradeRequired || sideEffects != 0 {
		t.Fatalf("status=%d sideEffects=%d body=%s", rec.Code, sideEffects, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "incompatible_protocol") {
		t.Fatal(rec.Body.String())
	}
}
