// Package githubhost owns fixed OAuth device-flow credentials and a bounded
// complete-generation catalog. SPA projections never carry credentials.
package githubhost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	RefreshDeadline                                           = 120 * time.Second
	MaxPages, MaxRepositories, MaxResponseBytes, MaxCacheBytes = 100, 10_000, 32 << 20, 16 << 20
	deviceCodeURL                                             = "https://github.com/login/device/code"
	tokenURL                                                  = "https://github.com/login/oauth/access_token"
	userURL                                                   = "https://api.github.com/user"
	defaultReposURL                                           = "https://api.github.com/user/repos?per_page=100&sort=full_name"
)

type AccountState string

const (
	StateUnauthenticated, StateAuthorizing, StateAuthenticated                 AccountState = "unauthenticated", "authorizing", "authenticated"
	StateDenied, StateExpired, StateCancelled, StateRefreshFailed, StateHostMisconfigured AccountState = "denied", "expired", "cancelled", "refresh_failed", "host_misconfigured"
)

type CatalogOutcome string

const (
	CatalogEmpty, CatalogCompleteCurrent, CatalogRefreshingRetained                    CatalogOutcome = "empty", "complete-current", "refreshing-retained"
	CatalogAuthenticationRequiredRetained, CatalogSourceFailedRetained                 CatalogOutcome = "authentication-required-retained", "source-failed-retained"
	CatalogTooLargeRetained, CatalogRefreshDeadlineRetained                            CatalogOutcome = "catalog_too_large-retained", "refresh_deadline-retained"
	FailCatalogTooLarge, FailRefreshDeadline, FailSourceFailed                         = "catalog_too_large", "refresh_deadline", "source_failed"
	FailAuthRequired, FailTransport, FailRefreshToken                                  = "authentication_required", "transport_failure", "refresh_failure"
)

type HTTPDoer interface{ Do(*http.Request) (*http.Response, error) }
type Clock interface{ Now() time.Time }
type ManualClock struct {
	mu sync.Mutex
	t  time.Time
}
type CredentialStore interface {
	Load() (CredentialRecord, bool, error)
	Save(CredentialRecord) error
}
type CacheStore interface {
	Load() (CatalogProjection, bool, error)
	Save(CatalogProjection) error
}
type OAuthApp struct {
	SchemaVersion int      `json:"schema_version"`
	ClientID      string   `json:"client_id"`
	Scopes        []string `json:"scopes"`
}
type Repository struct {
	ID    string `json:"repository_id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}
type AccountProjection struct {
	State           AccountState `json:"state"`
	Login           string       `json:"login,omitempty"`
	UserCode        string       `json:"user_code,omitempty"`
	VerificationURI string       `json:"verification_uri,omitempty"`
	FailureCode     string       `json:"failure_code,omitempty"`
}
type CatalogProjection struct {
	Outcome      CatalogOutcome `json:"outcome"`
	Generation   uint64         `json:"generation"`
	Stale        bool           `json:"stale"`
	FailureCode  string         `json:"failure_code,omitempty"`
	Repositories []Repository   `json:"repositories"`
}
type CredentialRecord struct {
	AccessToken, RefreshToken, TokenType, Scope, Login string
	ExpiresAt                                          time.Time
	Generation                                         uint64
}
type MemoryCredentialStore struct {
	mu sync.Mutex
	r  CredentialRecord
	ok bool
}
type MemoryCacheStore struct {
	mu sync.Mutex
	p  CatalogProjection
	ok bool
}

func NewManualClock(t time.Time) *ManualClock             { return &ManualClock{t: t} }
func (c *ManualClock) Now() time.Time                     { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *ManualClock) Advance(d time.Duration)            { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }
func FixedOAuthApp() OAuthApp                             { return OAuthApp{1, FixedClientID, []string{ScopeReadUser, ScopeRepo}} }
func (s *MemoryCredentialStore) Load() (CredentialRecord, bool, error) {
	s.mu.Lock(); defer s.mu.Unlock(); return s.r, s.ok, nil
}
func (s *MemoryCredentialStore) Save(r CredentialRecord) error {
	s.mu.Lock(); s.r, s.ok = r, true; s.mu.Unlock(); return nil
}
func (s *MemoryCacheStore) Load() (CatalogProjection, bool, error) {
	s.mu.Lock(); defer s.mu.Unlock(); return s.p, s.ok, nil
}
func (s *MemoryCacheStore) Save(p CatalogProjection) error {
	s.mu.Lock(); s.p, s.ok = p, true; s.mu.Unlock(); return nil
}

var credentialSentinels = []string{"access_token", "refresh_token", "client_secret", `"authorization"`, "bearer ", "gho_", "ghr_", "ghu_", "ghs_"}

func ProjectionContainsCredential(v any) bool {
	b, err := json.Marshal(v)
	if err != nil {
		return true
	}
	low := strings.ToLower(string(b))
	for _, s := range credentialSentinels {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

type Account struct {
	Doer HTTPDoer; Clock Clock; Store CredentialStore; App OAuthApp
	mu sync.Mutex
	state AccountState
	login, userCode, verificationURI, deviceCode, failureCode string
	interval time.Duration; expiresAt time.Time; credGen uint64; pollFlight chan struct{}
}
type deviceCodeResponse struct {
	DeviceCode string `json:"device_code"`; UserCode string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`; VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn int `json:"expires_in"`; Interval int `json:"interval"`
}
type tokenResponse struct {
	AccessToken string `json:"access_token"`; RefreshToken string `json:"refresh_token"`
	TokenType string `json:"token_type"`; Scope string `json:"scope"`; ExpiresIn int `json:"expires_in"`; Error string `json:"error"`
}

func NewAccount(d HTTPDoer, c Clock, s CredentialStore, app OAuthApp) *Account {
	return &Account{Doer: d, Clock: c, Store: s, App: app, state: StateUnauthenticated, interval: 5 * time.Second}
}
func (a *Account) Projection() AccountProjection { a.mu.Lock(); defer a.mu.Unlock(); return a.snap() }
func (a *Account) HostCredential() (string, bool) {
	r, ok, err := a.Store.Load(); if err != nil || !ok || r.AccessToken == "" { return "", false }; return r.AccessToken, true
}
func (a *Account) StartDeviceFlow(ctx context.Context) (AccountProjection, error) {
	if a.App.ClientID == "" { return a.set(StateHostMisconfigured, "host_misconfigured"), fmt.Errorf("githubhost: missing client_id") }
	var out deviceCodeResponse
	if err := a.post(ctx, deviceCodeURL, url.Values{"client_id": {a.App.ClientID}, "scope": {strings.Join(a.App.Scopes, " ")}}, &out); err != nil {
		return a.set(StateHostMisconfigured, FailTransport), err
	}
	a.mu.Lock(); a.state, a.userCode, a.deviceCode, a.failureCode = StateAuthorizing, out.UserCode, out.DeviceCode, ""
	a.verificationURI = out.VerificationURI; if a.verificationURI == "" { a.verificationURI = out.VerificationURIComplete }
	if out.Interval > 0 { a.interval = time.Duration(out.Interval) * time.Second }
	a.expiresAt = a.Clock.Now().Add(time.Duration(out.ExpiresIn) * time.Second); p := a.snap(); a.mu.Unlock(); return p, nil
}
func (a *Account) CancelDeviceFlow() AccountProjection {
	a.mu.Lock(); defer a.mu.Unlock()
	if a.state == StateAuthorizing { a.state, a.failureCode = StateCancelled, "cancelled"; a.userCode, a.verificationURI, a.deviceCode = "", "", "" }
	return a.snap()
}
func (a *Account) PollDeviceFlow(ctx context.Context) (AccountProjection, error) {
	a.mu.Lock()
	if a.state != StateAuthorizing { p := a.snap(); a.mu.Unlock(); return p, nil }
	if a.pollFlight != nil { ch := a.pollFlight; a.mu.Unlock(); select { case <-ctx.Done(): case <-ch: }; return a.Projection(), nil }
	done := make(chan struct{}); a.pollFlight = done; code, cid, exp := a.deviceCode, a.App.ClientID, a.expiresAt; a.mu.Unlock()
	defer func() { a.mu.Lock(); close(done); a.pollFlight = nil; a.mu.Unlock() }()
	if !a.Clock.Now().Before(exp) { return a.term(StateExpired, "expired"), nil }
	var out tokenResponse
	if err := a.post(ctx, tokenURL, url.Values{"client_id": {cid}, "device_code": {code}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &out); err != nil {
		return a.set(StateHostMisconfigured, FailTransport), err
	}
	return a.applyToken(ctx, out)
}
func (a *Account) applyToken(ctx context.Context, out tokenResponse) (AccountProjection, error) {
	if out.Error != "" {
		switch out.Error {
		case "authorization_pending", "slow_down": return a.Projection(), nil
		case "access_denied": return a.term(StateDenied, "denied"), nil
		case "expired_token": return a.term(StateExpired, "expired"), nil
		default: return a.term(StateHostMisconfigured, FailTransport), nil
		}
	}
	if out.AccessToken == "" { return a.set(StateHostMisconfigured, FailTransport), fmt.Errorf("githubhost: empty token") }
	login := ""
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil); err == nil {
		req.Header.Set("Authorization", "Bearer "+out.AccessToken); req.Header.Set("Accept", "application/vnd.github+json")
		if res, err := a.Doer.Do(req); err == nil {
			var body struct{ Login string `json:"login"` }
			_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); res.Body.Close(); login = body.Login
		}
	}
	a.mu.Lock(); a.credGen++; rec := CredentialRecord{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, TokenType: out.TokenType, Scope: out.Scope, Login: login, Generation: a.credGen}; a.mu.Unlock()
	if err := a.Store.Save(rec); err != nil { return a.set(StateHostMisconfigured, FailTransport), err }
	a.mu.Lock(); a.state, a.login, a.failureCode = StateAuthenticated, login, ""; a.userCode, a.verificationURI, a.deviceCode = "", "", ""; p := a.snap(); a.mu.Unlock(); return p, nil
}
func (a *Account) RefreshAccessToken(ctx context.Context) (AccountProjection, error) {
	rec, ok, err := a.Store.Load()
	if err != nil || !ok || rec.RefreshToken == "" { return a.set(StateRefreshFailed, FailRefreshToken), fmt.Errorf("githubhost: refresh unavailable") }
	var out tokenResponse
	if err := a.post(ctx, tokenURL, url.Values{"client_id": {a.App.ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {rec.RefreshToken}}, &out); err != nil || out.Error != "" || out.AccessToken == "" {
		return a.set(StateRefreshFailed, FailRefreshToken), fmt.Errorf("githubhost: refresh failed")
	}
	rec.AccessToken = out.AccessToken; if out.RefreshToken != "" { rec.RefreshToken = out.RefreshToken }
	a.mu.Lock(); a.credGen++; rec.Generation = a.credGen; a.mu.Unlock()
	if err := a.Store.Save(rec); err != nil { return a.Projection(), err }
	return a.set(StateAuthenticated, ""), nil
}
func (a *Account) set(st AccountState, code string) AccountProjection {
	a.mu.Lock(); defer a.mu.Unlock(); a.state, a.failureCode = st, code; return a.snap()
}
func (a *Account) term(st AccountState, code string) AccountProjection {
	a.mu.Lock(); defer a.mu.Unlock(); a.state, a.failureCode = st, code; a.userCode, a.verificationURI, a.deviceCode = "", "", ""; return a.snap()
}
func (a *Account) snap() AccountProjection {
	return AccountProjection{a.state, a.login, a.userCode, a.verificationURI, a.failureCode}
}
func (a *Account) post(ctx context.Context, ep string, form url.Values, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, strings.NewReader(form.Encode()))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded"); req.Header.Set("Accept", "application/json")
	res, err := a.Doer.Do(req); if err != nil { return err }; defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20)); if err != nil { return err }
	if res.StatusCode < 200 || res.StatusCode >= 300 { return fmt.Errorf("githubhost: status %d", res.StatusCode) }
	return json.Unmarshal(b, dest)
}

type Catalog struct {
	Doer HTTPDoer; Clock Clock; Cache CacheStore; Account *Account; ReposURL string
	mu sync.Mutex; proj CatalogProjection; flight *struct{ done chan struct{}; proj CatalogProjection; err error }; generation uint64
}

func NewCatalog(d HTTPDoer, c Clock, cache CacheStore, acct *Account) *Catalog {
	cat := &Catalog{Doer: d, Clock: c, Cache: cache, Account: acct, ReposURL: defaultReposURL, proj: CatalogProjection{Outcome: CatalogEmpty, Repositories: []Repository{}}}
	if p, ok, err := cache.Load(); err == nil && ok {
		cat.proj, cat.generation = p, p.Generation
		if cat.proj.Repositories == nil { cat.proj.Repositories = []Repository{} }
	}
	return cat
}
func (c *Catalog) Projection() CatalogProjection { c.mu.Lock(); defer c.mu.Unlock(); return clone(c.proj) }
func (c *Catalog) Refresh(ctx context.Context) (CatalogProjection, error) {
	c.mu.Lock()
	if c.flight != nil {
		f := c.flight; c.mu.Unlock()
		select { case <-ctx.Done(): return c.Projection(), ctx.Err(); case <-f.done: return clone(f.proj), f.err }
	}
	f := &struct{ done chan struct{}; proj CatalogProjection; err error }{done: make(chan struct{})}; c.flight = f
	if len(c.proj.Repositories) > 0 || c.proj.Generation > 0 { c.proj.Outcome, c.proj.Stale = CatalogRefreshingRetained, true }
	c.mu.Unlock()
	proj, err := c.once(ctx)
	c.mu.Lock(); f.proj, f.err, c.flight = proj, err, nil; close(f.done); c.mu.Unlock(); return clone(proj), err
}
func (c *Catalog) once(ctx context.Context) (CatalogProjection, error) {
	token, ok := c.Account.HostCredential()
	if !ok { return c.fail(CatalogAuthenticationRequiredRetained, FailAuthRequired), nil }
	deadline := c.Clock.Now().Add(RefreshDeadline)
	repos, bytes, pages, first, err := c.gen(ctx, token, deadline)
	if err != nil {
		switch err {
		case errDeadline: return c.fail(CatalogRefreshDeadlineRetained, FailRefreshDeadline), nil
		case errTooLarge: return c.fail(CatalogTooLargeRetained, FailCatalogTooLarge), nil
		case errAuth: return c.fail(CatalogAuthenticationRequiredRetained, FailAuthRequired), nil
		default: return c.fail(CatalogSourceFailedRetained, FailSourceFailed), nil
		}
	}
	sig, err := c.sig1(ctx, token)
	if err != nil || sig != first { return c.fail(CatalogSourceFailedRetained, FailSourceFailed), nil }
	if pages > MaxPages || len(repos) > MaxRepositories || bytes > MaxResponseBytes { return c.fail(CatalogTooLargeRetained, FailCatalogTooLarge), nil }
	if raw, _ := json.Marshal(repos); len(raw) > MaxCacheBytes { return c.fail(CatalogTooLargeRetained, FailCatalogTooLarge), nil }
	c.mu.Lock(); c.generation++; next := CatalogProjection{CatalogCompleteCurrent, c.generation, false, "", repos}; c.proj = next; c.mu.Unlock()
	_ = c.Cache.Save(next); return clone(next), nil
}
func (c *Catalog) fail(o CatalogOutcome, code string) CatalogProjection {
	c.mu.Lock(); defer c.mu.Unlock()
	c.proj.Outcome, c.proj.FailureCode = o, code
	c.proj.Stale = len(c.proj.Repositories) > 0 || c.proj.Generation > 0
	return clone(c.proj)
}
var errDeadline, errTooLarge, errAuth = fmt.Errorf("refresh_deadline"), fmt.Errorf("catalog_too_large"), fmt.Errorf("authentication_required")
func (c *Catalog) gen(ctx context.Context, token string, deadline time.Time) ([]Repository, int, int, string, error) {
	u, repos, bytes, pages, first := c.ReposURL, []Repository(nil), 0, 0, ""
	for u != "" {
		if !c.Clock.Now().Before(deadline) { return nil, bytes, pages, first, errDeadline }
		if pages >= MaxPages { return nil, bytes, pages, first, errTooLarge }
		page, next, n, etag, err := c.page(ctx, token, u)
		if err != nil { return nil, bytes, pages, first, err }
		bytes += n; pages++
		if pages == 1 { first = pageSig(page, etag) }
		repos = append(repos, page...)
		if len(repos) > MaxRepositories || bytes > MaxResponseBytes { return nil, bytes, pages, first, errTooLarge }
		u = next
	}
	return repos, bytes, pages, first, nil
}
func (c *Catalog) sig1(ctx context.Context, token string) (string, error) {
	repos, _, _, etag, err := c.page(ctx, token, c.ReposURL); if err != nil { return "", err }; return pageSig(repos, etag), nil
}
func pageSig(repos []Repository, etag string) string {
	if etag != "" { return "etag:" + etag }
	var b strings.Builder
	for _, r := range repos { b.WriteString(r.ID); b.WriteByte('|') }
	return b.String()
}
func (c *Catalog) page(ctx context.Context, token, pageURL string) ([]Repository, string, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil); if err != nil { return nil, "", 0, "", err }
	req.Header.Set("Authorization", "Bearer "+token); req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.Doer.Do(req); if err != nil { return nil, "", 0, "", err }; defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, MaxResponseBytes+1)); if err != nil { return nil, "", 0, "", err }
	if len(body) > MaxResponseBytes { return nil, "", len(body), "", errTooLarge }
	if res.StatusCode == 401 || res.StatusCode == 403 { return nil, "", len(body), "", errAuth }
	if res.StatusCode != 200 { return nil, "", len(body), "", fmt.Errorf("status %d", res.StatusCode) }
	var items []struct {
		ID int64 `json:"id"`; Name string `json:"name"`; FullName string `json:"full_name"`
		Owner struct{ Login string `json:"login"` } `json:"owner"`
	}
	if err := json.Unmarshal(body, &items); err != nil { return nil, "", len(body), "", err }
	repos := make([]Repository, 0, len(items))
	for _, it := range items {
		o, n := it.Owner.Login, it.Name
		if o == "" || n == "" { if p := strings.SplitN(it.FullName, "/", 2); len(p) == 2 { o, n = p[0], p[1] } }
		if o != "" && n != "" { repos = append(repos, Repository{strconv.FormatInt(it.ID, 10), o, n}) }
	}
	return repos, nextLink(res.Header.Get("Link")), len(body), res.Header.Get("ETag"), nil
}
func nextLink(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) { continue }
		s, e := strings.Index(part, "<"), strings.Index(part, ">")
		if s >= 0 && e > s { return part[s+1 : e] }
	}
	return ""
}
func clone(p CatalogProjection) CatalogProjection {
	out := p; out.Repositories = append([]Repository(nil), p.Repositories...)
	if out.Repositories == nil { out.Repositories = []Repository{} }
	return out
}

// RealAuthorityOutcome is the typed T3 fidelity admission result.
// Absent configuration is AuthorityUnavailable — never a silent pass or skip.
type RealAuthorityOutcome string

const (
	AuthorityConfigured  RealAuthorityOutcome = "configured"
	AuthorityUnavailable RealAuthorityOutcome = "unavailable"
)

// RealAuthAuthority is the opt-in GitHub OAuth App authority for FakeVsRealGitHubAuth.
type RealAuthAuthority struct {
	Outcome  RealAuthorityOutcome
	ClientID string
	Reason   string
}

// RealCatalogAuthority is the opt-in catalog token authority for FakeVsRealGitHubCatalog.
type RealCatalogAuthority struct {
	Outcome RealAuthorityOutcome
	Token   string
	Reason  string
}

// ResolveGitHubAuthAuthority reports whether real OAuth device-flow authority is configured.
func ResolveGitHubAuthAuthority() RealAuthAuthority {
	id := strings.TrimSpace(os.Getenv("AG_E2E_GITHUB_OAUTH_CLIENT_ID"))
	if id == "" {
		return RealAuthAuthority{Outcome: AuthorityUnavailable, Reason: "AG_E2E_GITHUB_OAUTH_CLIENT_ID unset"}
	}
	return RealAuthAuthority{Outcome: AuthorityConfigured, ClientID: id}
}

// ResolveGitHubCatalogAuthority reports whether real catalog API authority is configured.
func ResolveGitHubCatalogAuthority() RealCatalogAuthority {
	token := strings.TrimSpace(os.Getenv("AG_E2E_GITHUB_CATALOG_TOKEN"))
	if token == "" {
		return RealCatalogAuthority{Outcome: AuthorityUnavailable, Reason: "AG_E2E_GITHUB_CATALOG_TOKEN unset"}
	}
	return RealCatalogAuthority{Outcome: AuthorityConfigured, Token: token}
}
