package api

import (
	"encoding/json"
	"net/http"
)

// Exact-version admission for repository operations. Lease/preparation/process/
// filesystem side effects must run only after AdmitRepositoryOperation succeeds.

type RepositorySideEffectKind string

const (
	RepoSideEffectLease       RepositorySideEffectKind = "lease"
	RepoSideEffectPreparation RepositorySideEffectKind = "preparation"
	RepoSideEffectProcess     RepositorySideEffectKind = "process"
	RepoSideEffectFilesystem  RepositorySideEffectKind = "filesystem"
)

type RepositoryAdmissionResult struct {
	Admitted        bool
	Code            string
	ExpectedVersion string
	ActualVersion   string
}

// AdmitRepositoryOperation enforces exact protocol equality before side effects.
func AdmitRepositoryOperation(clientVersion string, sideEffect func(RepositorySideEffectKind)) RepositoryAdmissionResult {
	actual := clientVersion
	if actual == "" {
		actual = "<missing>"
	}
	if clientVersion != ProtocolVersion {
		return RepositoryAdmissionResult{
			Admitted: false, Code: "incompatible_protocol",
			ExpectedVersion: ProtocolVersion, ActualVersion: actual,
		}
	}
	if sideEffect != nil {
		sideEffect(RepoSideEffectLease)
	}
	return RepositoryAdmissionResult{
		Admitted: true, ExpectedVersion: ProtocolVersion, ActualVersion: clientVersion,
	}
}

type RepositoryCreateRequest struct {
	IntentID      string `json:"intent_id"`
	PreparationID string `json:"preparation_id"`
	ServerID      string `json:"server_id"`
	RepositoryID  string `json:"repository_id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Fingerprint   string `json:"fingerprint,omitempty"`
}

func DecodeRepositoryCreateRequest(raw []byte) (RepositoryCreateRequest, error) {
	var req RepositoryCreateRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return RepositoryCreateRequest{}, err
	}
	if req.IntentID == "" || req.PreparationID == "" || req.ServerID == "" ||
		req.RepositoryID == "" || req.Owner == "" || req.Name == "" {
		return RepositoryCreateRequest{}, errRepositoryInvalidIdentity
	}
	return req, nil
}

type repositoryIdentityError struct{}

func (repositoryIdentityError) Error() string {
	return "repository create requires fully-qualified identities"
}

var errRepositoryInvalidIdentity repositoryIdentityError

func writeRepositoryProtocolMismatch(w http.ResponseWriter, actual string) {
	if actual == "" {
		actual = "<missing>"
	}
	writeProtocolVersionError(w, actual)
}
