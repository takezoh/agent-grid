package webhost
import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
	"github.com/takezoh/agent-grid/clients/web-host/internal/githubhost"
)
const (
	leaseFailAuth     = "authenticated_transport_required"
	leaseFailProtocol = "incompatible_protocol"
	leaseFailNoCred   = "authentication_required"
	leaseFailXport    = "transport_failure"
	leaseFailInvalid  = "invalid_identity"
)
type HostSource struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Status    string `json:"status,omitempty"`
}
type HostProjectTarget struct {
	ServerID string `json:"server_id"`
	Project  string `json:"project"`
}
type HostRepositoryIdentity struct {
	ID    string `json:"repository_id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}
type HostTarget struct {
	Source     string                  `json:"source"`
	Project    *HostProjectTarget      `json:"project,omitempty"`
	Repository *HostRepositoryIdentity `json:"repository,omitempty"`
}
type HostPreparation struct {
	PreparationID          string                 `json:"preparation_id"`
	IntentID               string                 `json:"intent_id"`
	ServerID               string                 `json:"server_id"`
	Repository             HostRepositoryIdentity `json:"repository"`
	Stage                  string                 `json:"stage"`
	FailureCode            string                 `json:"failure_code,omitempty"`
	RetryOrigin            string                 `json:"retry_origin,omitempty"`
	EnvironmentDisposition string                 `json:"environment_disposition,omitempty"`
	Revision               uint64                 `json:"revision"`
	SessionID              string                 `json:"session_id,omitempty"`
	ClonePath              string                 `json:"clone_path,omitempty"`
	EnvironmentID          string                 `json:"environment_id,omitempty"`
}
type HostRepositoryProjection struct {
	Sources      []HostSource                 `json:"sources"`
	Account      githubhost.AccountProjection `json:"account"`
	Catalog      githubhost.CatalogProjection `json:"catalog"`
	Targets      []HostTarget                 `json:"targets"`
	Preparations []HostPreparation            `json:"preparations"`
}
// BuildHostRepositoryProjection: Projects source is never reduced by GitHub failure.
func BuildHostRepositoryProjection(
	projects []HostProjectTarget, account githubhost.AccountProjection,
	catalog githubhost.CatalogProjection, preps []HostPreparation,
) HostRepositoryProjection {
	if projects == nil {
		projects = []HostProjectTarget{}
	}
	if catalog.Repositories == nil {
		catalog.Repositories = []githubhost.Repository{}
	}
	if preps == nil {
		preps = []HostPreparation{}
	}
	ghOK := catalog.Outcome == githubhost.CatalogCompleteCurrent ||
		catalog.Outcome == githubhost.CatalogRefreshingRetained ||
		(catalog.Stale && len(catalog.Repositories) > 0) ||
		account.State == githubhost.StateAuthenticated || account.State == githubhost.StateAuthorizing
	st := string(catalog.Outcome)
	if st == "" || st == string(githubhost.CatalogEmpty) {
		st = string(account.State)
	}
	sources := []HostSource{{ID: "projects", Available: true}, {ID: "github", Available: ghOK, Status: st}}
	targets := make([]HostTarget, 0, len(projects)+len(catalog.Repositories))
	for _, p := range projects {
		if p.ServerID == "" || p.Project == "" {
			continue
		}
		cp := p
		targets = append(targets, HostTarget{Source: "project", Project: &cp})
	}
	for _, r := range catalog.Repositories {
		if r.ID == "" || r.Owner == "" || r.Name == "" {
			continue
		}
		id := HostRepositoryIdentity{r.ID, r.Owner, r.Name}
		targets = append(targets, HostTarget{Source: "repository", Repository: &id})
	}
	return HostRepositoryProjection{sources, account, catalog, targets, preps}
}
func PreflightRepositoryLease(authMode, observedProtocol, serverURL string) *struct{ Code string } {
	if authMode == "" || authMode == "none" {
		return &struct{ Code string }{leaseFailAuth}
	}
	if observedProtocol != protocolVersion {
		return &struct{ Code string }{leaseFailProtocol}
	}
	if !isSecureServerURL(serverURL) {
		return &struct{ Code string }{leaseFailAuth}
	}
	return nil
}
func isSecureServerURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Scheme, "https") {
		return true
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
type LeaseIssue struct {
	LeaseID   string    `json:"lease_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
type LeaseRequest struct {
	HostID, RepositoryID, Owner, Name, ServerID string
}
type LeaseResult struct {
	Issue *LeaseIssue `json:"issue,omitempty"`
	Code  string      `json:"code,omitempty"`
}
type LeaseHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type CredentialAuthority interface {
	HostCredential() (string, bool)
	CredentialGeneration() (string, bool)
}
type RepositoryLeaseMediator struct {
	Auth CredentialAuthority
	Doer LeaseHTTPDoer
	HostID, Protocol string
}
// RequestLease: preflight then host-only credential transfer.
// rejects counterexample: credential read before preflight; sentinel in response.
func (m *RepositoryLeaseMediator) RequestLease(
	ctx context.Context, server clientconfig.Server, observedProtocol string, req LeaseRequest,
) LeaseResult {
	if fail := PreflightRepositoryLease(server.Auth.Mode, observedProtocol, server.URL); fail != nil {
		return LeaseResult{Code: fail.Code}
	}
	if req.RepositoryID == "" || req.Owner == "" || req.Name == "" {
		return LeaseResult{Code: leaseFailInvalid}
	}
	token, ok := m.Auth.HostCredential()
	if !ok || token == "" {
		return LeaseResult{Code: leaseFailNoCred}
	}
	gen, gok := m.Auth.CredentialGeneration()
	if !gok {
		gen = "0"
	}
	hostID := req.HostID
	if hostID == "" {
		hostID = m.HostID
	}
	if hostID == "" {
		hostID = "web-host"
	}
	body, _ := json.Marshal(map[string]string{
		"host_id": hostID, "repository_id": req.RepositoryID, "owner": req.Owner, "name": req.Name,
		"token": token, "token_generation": gen,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(server.URL, "/")+"/api/repository/leases", bytes.NewReader(body))
	if err != nil {
		return LeaseResult{Code: leaseFailXport}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	proto := m.Protocol
	if proto == "" {
		proto = protocolVersion
	}
	httpReq.Header.Set(protocolVersionHeader, proto)
	if server.Auth.Mode == "bearer" && server.Auth.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+server.Auth.Token)
	}
	doer := m.Doer
	if doer == nil {
		doer = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := doer.Do(httpReq)
	if err != nil {
		return LeaseResult{Code: leaseFailXport}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var eb struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &eb)
		if eb.Code == "" {
			eb.Code = leaseFailXport
		}
		return LeaseResult{Code: eb.Code}
	}
	var issue LeaseIssue
	if err := json.Unmarshal(raw, &issue); err != nil || issue.LeaseID == "" {
		return LeaseResult{Code: leaseFailXport}
	}
	return LeaseResult{Issue: &issue}
}
