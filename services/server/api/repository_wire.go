package api

import (
	"encoding/json"
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
// sideEffect is invoked only after equality; mismatch never runs any effect.
func AdmitRepositoryOperation(clientVersion string, sideEffect func(RepositorySideEffectKind)) RepositoryAdmissionResult {
	return AdmitRepositoryOperationAgainst(ProtocolVersion, clientVersion, sideEffect)
}

// AdmitRepositoryOperationAgainst models old/new pairs: serverExpected is the
// server's token, clientVersion is what the peer presented. Equality only.
func AdmitRepositoryOperationAgainst(serverExpected, clientVersion string, sideEffect func(RepositorySideEffectKind)) RepositoryAdmissionResult {
	actual := clientVersion
	if actual == "" {
		actual = "<missing>"
	}
	if clientVersion == "" || clientVersion != serverExpected {
		return RepositoryAdmissionResult{
			Admitted: false, Code: "incompatible_protocol",
			ExpectedVersion: serverExpected, ActualVersion: actual,
		}
	}
	if sideEffect != nil {
		// Lease is the first credential-adjacent effect; callers may chain others.
		sideEffect(RepoSideEffectLease)
	}
	return RepositoryAdmissionResult{
		Admitted: true, ExpectedVersion: serverExpected, ActualVersion: clientVersion,
	}
}

type RepositoryCreateRequest struct {
	IntentID        string         `json:"intent_id"`
	PreparationID   string         `json:"preparation_id"`
	ServerID        string         `json:"server_id"`
	RepositoryID    string         `json:"repository_id"`
	Owner           string         `json:"owner"`
	Name            string         `json:"name"`
	LeaseID         string         `json:"lease_id,omitempty"`
	Fingerprint     string         `json:"fingerprint,omitempty"`
	Version         int            `json:"version"`
	ProfileID       string         `json:"profile_id"`
	ProfileRevision string         `json:"profile_revision"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	Cols            int            `json:"cols,omitempty"`
	Rows            int            `json:"rows,omitempty"`
}

func DecodeRepositoryCreateRequest(raw []byte) (RepositoryCreateRequest, error) {
	var req RepositoryCreateRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return RepositoryCreateRequest{}, err
	}
	if req.IntentID == "" || req.PreparationID == "" || req.ServerID == "" ||
		req.RepositoryID == "" || req.Owner == "" || req.Name == "" ||
		req.Version != 2 || req.ProfileID == "" || req.ProfileRevision == "" {
		return RepositoryCreateRequest{}, errRepositoryInvalidIdentity
	}
	return req, nil
}

type repositoryIdentityError struct{}

func (repositoryIdentityError) Error() string {
	return "repository create requires fully-qualified identities"
}

var errRepositoryInvalidIdentity repositoryIdentityError
