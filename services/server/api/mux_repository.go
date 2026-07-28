package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
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
	mux.HandleFunc("DELETE /api/repository/leases/{id}", handleRevokeLease(deps.noAuth, leases))
	mux.HandleFunc("POST /api/repository/leases/revoke-host", handleRevokeHostLeases(deps.noAuth, leases))
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
