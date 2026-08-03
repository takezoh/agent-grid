package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/takezoh/agent-grid/services/host/config"
	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

type leaseCreateRequest struct {
	HostID          string `json:"host_id"`
	RepositoryID    string `json:"repository_id"`
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	Token           string `json:"token"`
	TokenGeneration string `json:"token_generation"`
}
type leaseErrorBody struct {
	Code string `json:"code"`
}

func registerRepositoryRoutes(mux *http.ServeMux, deps muxDependencies) {
	leases := deps.leases
	if leases == nil {
		leases = newLeaseStore()
	}
	mux.HandleFunc("POST /api/repository/leases", handleCreateLease(deps.noAuth, leases))
	mux.HandleFunc("POST /api/repository-sessions", handleCreateRepositorySession(
		deps.daemon, leases, deps.repositoryAuthority, deps.sessionConfigLoader))
	mux.HandleFunc("GET /api/repository-preparations/{id}", handleGetRepositoryPreparation(deps.daemon))
	mux.HandleFunc("DELETE /api/repository/leases/{id}", handleRevokeLease(deps.noAuth, leases))
	mux.HandleFunc("POST /api/repository/leases/revoke-host", handleRevokeHostLeases(deps.noAuth, leases))
}

func handleGetRepositoryPreparation(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d == nil || !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			gatewayAPIError(w, r, http.StatusBadRequest, "invalid_identity", "preparation identity is required")
			return
		}
		payload, _ := json.Marshal(state.RepositoryPreparationParams{PreparationID: state.PreparationID(id)})
		ctx, cancel := rpcContext(r)
		defer cancel()
		response, err := d.SendCommand(ctx, proto.CmdEvent{Event: state.EventGetRepositoryPreparation, Payload: payload})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		prep, ok := response.(proto.RespRepositoryPreparation)
		if !ok {
			gatewayError(w, r, http.StatusBadGateway, "response_type_mismatch", "unexpected preparation response")
			return
		}
		writeJSON(w, http.StatusOK, prep)
	}
}

func handleCreateRepositorySession(
	d *DaemonClient,
	leases *leaseStore,
	authority RepositoryAuthoritySink,
	loadConfig func() (*config.Config, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d == nil || !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable", "daemon unavailable")
			return
		}
		var req RepositoryCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gatewayAPIError(w, r, http.StatusBadRequest, "invalid_identity", "invalid repository create request")
			return
		}
		if _, err := DecodeRepositoryCreateRequest(repositoryJSON(req)); err != nil {
			gatewayAPIError(w, r, http.StatusBadRequest, "invalid_identity", err.Error())
			return
		}
		if err := state.ValidateSizeHint(req.Cols, req.Rows); err != nil {
			gatewayAPIError(w, r, http.StatusBadRequest, "invalid_cols_rows", err.Error())
			return
		}
		cfg, err := loadConfig()
		if err != nil {
			gatewayError(w, r, http.StatusInternalServerError, "config_load_failed", "settings.toml load failed", "err", err)
			return
		}
		argv, err := config.MaterializeResolvedProfile(
			cfg.Session.ResolveBaseProfiles(), req.ProfileID, req.ProfileRevision, req.Parameters)
		if err != nil {
			status, code := http.StatusBadRequest, "invalid_profile_parameters"
			if strings.Contains(err.Error(), "stale profile revision") {
				status, code = http.StatusConflict, "stale_profile_revision"
			}
			gatewayAPIError(w, r, status, code, err.Error())
			return
		}
		command := joinShellCommand(argv)
		if req.LeaseID != "" {
			lease, failure := leases.Consume(req.LeaseID)
			if failure != nil {
				gatewayAPIError(w, r, http.StatusBadRequest, failure.Code, failure.Code)
				return
			}
			if lease.RepositoryID != req.RepositoryID || lease.Owner != req.Owner || lease.Name != req.Name {
				gatewayAPIError(w, r, http.StatusBadRequest, "lease_identity_mismatch", "lease repository identity mismatch")
				return
			}
			if authority == nil {
				gatewayAPIError(w, r, http.StatusServiceUnavailable, "authority_unavailable", "repository authority unavailable")
				return
			}
			if err := authority.Offer(req.ServerID, req.RepositoryID, req.Owner, req.Name, lease.Username, lease.Token); err != nil {
				gatewayAPIError(w, r, http.StatusServiceUnavailable, "authority_unavailable", "repository authority unavailable")
				return
			}
		}
		payload, _ := json.Marshal(state.RepositoryCreateParams{
			IntentID: state.CreationIntentID(req.IntentID), PreparationID: state.PreparationID(req.PreparationID),
			ServerID: state.ExecutionServerID(req.ServerID),
			Repository: state.GitHubRepositoryIdentity{
				ID: state.GitHubRepositoryID(req.RepositoryID), Owner: state.RepositoryOwner(req.Owner), Name: state.RepositoryName(req.Name),
			},
			Fingerprint: req.Fingerprint, Command: command,
		})
		ctx, cancel := rpcContext(r)
		defer cancel()
		if _, err := d.SendCommand(ctx, proto.CmdEvent{Event: state.EventCreateRepositorySession, Payload: payload}); err != nil {
			handleProtoError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"preparation_id": req.PreparationID, "intent_id": req.IntentID, "server_id": req.ServerID,
			"repository": map[string]string{"repository_id": req.RepositoryID, "owner": req.Owner, "name": req.Name},
			"stage":      "confirmed", "revision": 1,
		})
	}
}

func repositoryJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}
func handleCreateLease(noAuth bool, leases *leaseStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fail, st := admitRepositoryLease(r, noAuth); fail != nil {
			writeJSON(w, st, fail)
			return
		}
		var req leaseCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, leaseErrorBody{Code: "invalid_identity"})
			return
		}
		issue, fail := leases.Issue(req.HostID, req.RepositoryID, req.Owner, req.Name, req.Token, req.TokenGeneration)
		if fail != nil {
			st := http.StatusBadRequest
			if fail.Code == "lease_capacity_exceeded" {
				st = http.StatusTooManyRequests
			}
			writeJSON(w, st, leaseErrorBody{Code: fail.Code})
			return
		}
		writeJSON(w, http.StatusCreated, issue)
	}
}
func handleRevokeLease(noAuth bool, leases *leaseStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fail, st := admitRepositoryLease(r, noAuth); fail != nil {
			writeJSON(w, st, fail)
			return
		}
		leases.Revoke(r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	}
}
func handleRevokeHostLeases(noAuth bool, leases *leaseStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fail, st := admitRepositoryLease(r, noAuth); fail != nil {
			writeJSON(w, st, fail)
			return
		}
		var body struct {
			HostID          string `json:"host_id"`
			TokenGeneration string `json:"token_generation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.HostID == "" {
			writeJSON(w, http.StatusBadRequest, leaseErrorBody{Code: "invalid_identity"})
			return
		}
		if body.TokenGeneration != "" {
			leases.RevokeGeneration(body.HostID, body.TokenGeneration)
		} else {
			leases.RevokeHost(body.HostID)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// admitRepositoryLease enforces transport authenticity for lease routes.
// Exact protocol-version admission is owned by the global ProtocolVersionGate
// wrapping apiHandler in protocolAwareAPIHandler (see newMux). Handlers must
// not re-implement a second incompatible version gate.
func admitRepositoryLease(r *http.Request, noAuth bool) (*leaseErrorBody, int) {
	if noAuth || !isSecureRepositoryTransport(r) {
		return &leaseErrorBody{Code: "authenticated_transport_required"}, http.StatusForbidden
	}
	return nil, 0
}
func isSecureRepositoryTransport(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return isLoopbackHostPort(r.RemoteAddr)
}
func isLoopbackHostPort(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
