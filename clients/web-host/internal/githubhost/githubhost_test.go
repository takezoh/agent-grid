package githubhost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type FakeGitHubAuth struct {
	mu sync.Mutex
	DeviceCodes []deviceCodeResponse
	TokenPolls []tokenResponse
	UserLogin string
	di, ti int
}
func (f *FakeGitHubAuth) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	switch {
	case strings.HasSuffix(req.URL.Path, "/login/device/code"):
		o := deviceCodeResponse{DeviceCode: "dev", UserCode: "ABCD-1234", VerificationURI: "https://github.com/login/device", ExpiresIn: 900, Interval: 1}
		if f.di < len(f.DeviceCodes) { o = f.DeviceCodes[f.di]; f.di++ }
		return jrt(200, o)
	case strings.HasSuffix(req.URL.Path, "/login/oauth/access_token"):
		o := tokenResponse{Error: "authorization_pending"}
		if f.ti < len(f.TokenPolls) { o = f.TokenPolls[f.ti]; f.ti++ }
		return jrt(200, o)
	case strings.HasSuffix(req.URL.Path, "/user"):
		return jrt(200, map[string]string{"login": f.UserLogin})
	default:
		return jrt(404, map[string]string{"m": "nf"})
	}
}
type FakeGitHubCatalog struct {
	mu sync.Mutex
	Pages [][]Repository
	FailAtPage, AuthStatus, pageCalls int
	ChangeAfterComplete bool
	ETag string
}
func (f *FakeGitHubCatalog) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if !strings.Contains(req.URL.Path, "/user/repos") { return jrt(404, map[string]string{"m": "nf"}) }
	if f.AuthStatus != 0 { return jrt(f.AuthStatus, map[string]string{"m": "bad"}) }
	f.pageCalls++
	pages := f.Pages
	if len(pages) == 0 { pages = [][]Repository{{}} }
	idx := f.pageCalls - 1
	if idx >= len(pages) {
		idx = 0
		if f.ChangeAfterComplete { return cpage([]Repository{{ID: "9", Owner: "x", Name: "y"}}, "", f.ETag+"-x") }
	}
	if f.FailAtPage > 0 && f.pageCalls == f.FailAtPage { return jrt(500, map[string]string{"m": "boom"}) }
	next := ""
	if f.pageCalls < len(pages) { next = req.URL.Scheme + "://" + req.URL.Host + req.URL.Path + "?page=" + strconv.Itoa(f.pageCalls+1) }
	return cpage(pages[idx], next, f.ETag)
}
type CombinedTransport struct{ Auth, Catalog http.RoundTripper }
func (c CombinedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/user/repos") { return c.Catalog.RoundTrip(req) }
	return c.Auth.RoundTrip(req)
}
func ClientFromRT(rt http.RoundTripper) *http.Client { return &http.Client{Transport: rt} }
func jrt(status int, v any) (*http.Response, error) {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(b))}, nil
}
func cpage(repos []Repository, next, etag string) (*http.Response, error) {
	raw := make([]map[string]any, 0, len(repos))
	for i, r := range repos {
		id, _ := strconv.Atoi(r.ID); if id == 0 { id = i + 1 }
		raw = append(raw, map[string]any{"id": id, "name": r.Name, "full_name": r.Owner + "/" + r.Name, "owner": map[string]string{"login": r.Owner}})
	}
	b, _ := json.Marshal(raw)
	h := http.Header{"Content-Type": []string{"application/json"}}
	if next != "" { h.Set("Link", `<`+next+`>; rel="next"`) }
	if etag != "" { h.Set("ETag", etag) }
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(b))}, nil
}

// TestGitHubAuthContract: success/denied/expired/cancelled/failure + no credential leak.
// rejects counterexample: SPA projection containing access/refresh tokens.
func TestGitHubAuthContract(t *testing.T) {
	a := newAcct(&FakeGitHubAuth{UserLogin: "alice", TokenPolls: []tokenResponse{{AccessToken: "gho_secret_token", RefreshToken: "ghr_r"}}})
	_, _ = a.StartDeviceFlow(context.Background())
	p, err := a.PollDeviceFlow(context.Background())
	if err != nil || p.State != StateAuthenticated || p.Login != "alice" { t.Fatalf("%+v %v", p, err) }
	noCred(t, p)
	if tok, ok := a.HostCredential(); !ok || tok == "" { t.Fatal("host credential") }

	for _, tc := range []struct{ err, want string }{{"access_denied", string(StateDenied)}, {"expired_token", string(StateExpired)}} {
		a = newAcct(&FakeGitHubAuth{TokenPolls: []tokenResponse{{Error: tc.err}}})
		_, _ = a.StartDeviceFlow(context.Background())
		if p, _ = a.PollDeviceFlow(context.Background()); string(p.State) != tc.want { t.Fatalf("%s -> %s", tc.err, p.State) }
	}
	a = newAcct(&FakeGitHubAuth{}); _, _ = a.StartDeviceFlow(context.Background())
	if a.CancelDeviceFlow().State != StateCancelled { t.Fatal("cancel") }
	a = NewAccount(ClientFromRT(&FakeGitHubAuth{}), NewManualClock(time.Unix(1, 0)), &MemoryCredentialStore{}, OAuthApp{})
	if p, err = a.StartDeviceFlow(context.Background()); err == nil || p.State != StateHostMisconfigured { t.Fatalf("%+v %v", p, err) }
	fake := &FakeGitHubAuth{UserLogin: "a", TokenPolls: []tokenResponse{{AccessToken: "gho_a", RefreshToken: "ghr_r"}}}
	a = newAcct(fake); _, _ = a.StartDeviceFlow(context.Background()); _, _ = a.PollDeviceFlow(context.Background())
	fake.TokenPolls, fake.ti = []tokenResponse{{Error: "bad"}}, 0
	if p, err = a.RefreshAccessToken(context.Background()); err == nil || p.State != StateRefreshFailed { t.Fatalf("%+v %v", p, err) }
}

// TestGitHubCatalogContract: singleflight, limits, partial non-publish, stale, source-local failure.
// rejects counterexample: clearing retained repos or publishing partial pages.
func TestGitHubCatalogContract(t *testing.T) {
	gate := make(chan struct{})
	fake := &FakeGitHubCatalog{Pages: [][]Repository{{{ID: "1", Owner: "o", Name: "a"}, {ID: "2", Owner: "o", Name: "b"}}}, ETag: `"g"`}
	cat := newCat(fake)
	blocked := &gatedRT{inner: fake, gate: gate, entered: make(chan struct{}, 1)}
	cat.Doer = ClientFromRT(CombinedTransport{Auth: &FakeGitHubAuth{}, Catalog: blocked})
	_ = cat.Account.Store.Save(CredentialRecord{AccessToken: "gho_a", Generation: 1})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ { wg.Add(1); go func() { defer wg.Done(); _, _ = cat.Refresh(context.Background()) }() }
	<-blocked.entered; time.Sleep(15 * time.Millisecond); close(gate); wg.Wait()
	p := cat.Projection()
	if p.Outcome != CatalogCompleteCurrent || p.Generation != 1 || len(p.Repositories) != 2 || fake.pageCalls != 2 {
		t.Fatalf("%+v calls=%d", p, fake.pageCalls)
	}
	noCred(t, p)

	cat = newCat(&FakeGitHubCatalog{Pages: [][]Repository{{{ID: "1", Owner: "o", Name: "kept"}}}, ETag: `"g1"`})
	_, _ = cat.Refresh(context.Background())
	cat.Doer = ClientFromRT(CombinedTransport{Auth: &FakeGitHubAuth{}, Catalog: &FakeGitHubCatalog{Pages: [][]Repository{{{ID: "9", Owner: "o", Name: "x"}}, {{ID: "10", Owner: "o", Name: "y"}}}, FailAtPage: 2, ETag: `"g2"`}})
	_ = cat.Account.Store.Save(CredentialRecord{AccessToken: "gho_a", Generation: 1})
	if p, _ = cat.Refresh(context.Background()); p.Outcome != CatalogSourceFailedRetained || !p.Stale || p.Generation != 1 || p.Repositories[0].Name != "kept" {
		t.Fatalf("partial published: %+v", p)
	}

	pages := make([][]Repository, MaxPages+1)
	for i := range pages { pages[i] = []Repository{{ID: strconv.Itoa(i + 1), Owner: "o", Name: "r"}} }
	cat = newCat(&FakeGitHubCatalog{Pages: pages, ETag: `"big"`})
	cat.proj = CatalogProjection{Outcome: CatalogCompleteCurrent, Generation: 3, Repositories: []Repository{{ID: "old", Owner: "o", Name: "old"}}}
	cat.generation = 3
	if p, _ = cat.Refresh(context.Background()); p.Outcome != CatalogTooLargeRetained || p.Generation != 3 || p.Repositories[0].Name != "old" {
		t.Fatalf("limit: %+v", p)
	}

	clock := NewManualClock(time.Unix(1_700_000_000, 0))
	adv := &deadlineRT{inner: &FakeGitHubCatalog{Pages: [][]Repository{{{ID: "1", Owner: "o", Name: "a"}}, {{ID: "2", Owner: "o", Name: "b"}}}, ETag: `"g"`}, clock: clock}
	httpC := ClientFromRT(CombinedTransport{Auth: &FakeGitHubAuth{}, Catalog: adv})
	acct := NewAccount(httpC, clock, &MemoryCredentialStore{}, FixedOAuthApp())
	_ = acct.Store.Save(CredentialRecord{AccessToken: "gho_a", Generation: 1})
	cache := &MemoryCacheStore{}
	_ = cache.Save(CatalogProjection{Outcome: CatalogCompleteCurrent, Generation: 1, Repositories: []Repository{{ID: "k", Owner: "o", Name: "kept"}}})
	if p, _ = NewCatalog(httpC, clock, cache, acct).Refresh(context.Background()); p.Outcome != CatalogRefreshDeadlineRetained || p.Repositories[0].Name != "kept" {
		t.Fatalf("deadline: %+v", p)
	}

	cat = newCat(&FakeGitHubCatalog{Pages: [][]Repository{{{ID: "1", Owner: "o", Name: "a"}}}, ETag: `"g"`})
	_, _ = cat.Refresh(context.Background())
	cat.Doer = ClientFromRT(CombinedTransport{Auth: &FakeGitHubAuth{}, Catalog: &FakeGitHubCatalog{AuthStatus: 500}})
	_ = cat.Account.Store.Save(CredentialRecord{AccessToken: "gho_a", Generation: 1})
	if p, _ = cat.Refresh(context.Background()); p.Outcome != CatalogSourceFailedRetained || !p.Stale || len(p.Repositories) != 1 {
		t.Fatalf("source: %+v", p)
	}
}

func TestSharedGitHubConfigFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "..", "config")
	raw, err := os.ReadFile(filepath.Join(root, "fixtures", "github-oauth-valid.json"))
	if err != nil { t.Fatal(err) }
	var app OAuthApp
	if err := json.Unmarshal(raw, &app); err != nil || app.ClientID != FixedClientID || len(app.Scopes) != 2 { t.Fatalf("%+v %v", app, err) }
	noCred(t, app)
	raw, err = os.ReadFile(filepath.Join(root, "fixtures", "github-projection-credential-free.json"))
	if err != nil { t.Fatal(err) }
	var proj struct {
		Account AccountProjection `json:"account"`
		Catalog CatalogProjection `json:"catalog"`
	}
	if err := json.Unmarshal(raw, &proj); err != nil || proj.Account.State != StateAuthenticated { t.Fatalf("%+v %v", proj, err) }
	noCred(t, proj)
	schema, _ := os.ReadFile(filepath.Join(root, "github.schema.json"))
	if strings.Contains(string(schema), `"client_secret"`) { t.Fatal("client_secret in schema") }
}

// TestFakeVsRealGitHubAuth compares fake and real device-code start shapes.
// Absent T3 authority is a typed unavailable outcome, never an opt-out pass.
func TestFakeVsRealGitHubAuth(t *testing.T) {
	auth := ResolveGitHubAuthAuthority()
	if auth.Outcome != AuthorityConfigured {
		if auth.Outcome != AuthorityUnavailable {
			t.Fatalf("expected typed unavailable, got %q", auth.Outcome)
		}
		if auth.Reason != "AG_E2E_GITHUB_OAUTH_CLIENT_ID unset" {
			t.Fatalf("unavailable reason: %q", auth.Reason)
		}
		return
	}
	clock := NewManualClock(time.Now())
	rp, err := NewAccount(http.DefaultClient, clock, &MemoryCredentialStore{}, OAuthApp{SchemaVersion: 1, ClientID: auth.ClientID, Scopes: []string{ScopeReadUser, ScopeRepo}}).StartDeviceFlow(context.Background())
	if err != nil || rp.State != StateAuthorizing || rp.UserCode == "" { t.Fatalf("%+v %v", rp, err) }
	fp, err := newAcct(&FakeGitHubAuth{DeviceCodes: []deviceCodeResponse{{DeviceCode: "d", UserCode: rp.UserCode, VerificationURI: rp.VerificationURI, ExpiresIn: 900}}}).StartDeviceFlow(context.Background())
	if err != nil || fp.State != rp.State { t.Fatalf("fake=%+v real=%+v", fp, rp) }
	noCred(t, rp)
}

// TestFakeVsRealGitHubCatalog compares fake and real complete catalog shapes.
// Absent T3 authority is a typed unavailable outcome, never an opt-out pass.
func TestFakeVsRealGitHubCatalog(t *testing.T) {
	auth := ResolveGitHubCatalogAuthority()
	if auth.Outcome != AuthorityConfigured {
		if auth.Outcome != AuthorityUnavailable {
			t.Fatalf("expected typed unavailable, got %q", auth.Outcome)
		}
		if auth.Reason != "AG_E2E_GITHUB_CATALOG_TOKEN unset" {
			t.Fatalf("unavailable reason: %q", auth.Reason)
		}
		return
	}
	clock := NewManualClock(time.Now())
	store := &MemoryCredentialStore{}; _ = store.Save(CredentialRecord{AccessToken: auth.Token, Generation: 1})
	rp, err := NewCatalog(http.DefaultClient, clock, &MemoryCacheStore{}, NewAccount(http.DefaultClient, clock, store, FixedOAuthApp())).Refresh(context.Background())
	if err != nil || rp.Outcome != CatalogCompleteCurrent { t.Fatalf("%+v %v", rp, err) }
	page := rp.Repositories; if len(page) > 3 { page = page[:3] }
	httpC := ClientFromRT(CombinedTransport{Auth: &FakeGitHubAuth{}, Catalog: &FakeGitHubCatalog{Pages: [][]Repository{page}, ETag: `"r"`}})
	fs := &MemoryCredentialStore{}; _ = fs.Save(CredentialRecord{AccessToken: "gho_f", Generation: 1})
	fp, err := NewCatalog(httpC, clock, &MemoryCacheStore{}, NewAccount(httpC, clock, fs, FixedOAuthApp())).Refresh(context.Background())
	if err != nil || len(fp.Repositories) != len(page) { t.Fatalf("%+v %v", fp, err) }
	noCred(t, rp); noCred(t, fp)
}

type deadlineRT struct{ inner *FakeGitHubCatalog; clock *ManualClock }
type gatedRT struct{ inner *FakeGitHubCatalog; gate <-chan struct{}; entered chan struct{}; once sync.Once }
func (d *deadlineRT) RoundTrip(req *http.Request) (*http.Response, error) {
	d.clock.Advance(RefreshDeadline); return d.inner.RoundTrip(req)
}
func (g *gatedRT) RoundTrip(req *http.Request) (*http.Response, error) {
	g.once.Do(func() { select { case g.entered <- struct{}{}: default: }; <-g.gate })
	return g.inner.RoundTrip(req)
}
func newAcct(f *FakeGitHubAuth) *Account {
	return NewAccount(ClientFromRT(f), NewManualClock(time.Unix(1_700_000_000, 0)), &MemoryCredentialStore{}, FixedOAuthApp())
}
func newCat(c *FakeGitHubCatalog) *Catalog {
	auth := &FakeGitHubAuth{UserLogin: "alice", TokenPolls: []tokenResponse{{AccessToken: "gho_secret", RefreshToken: "ghr_secret"}}}
	httpC := ClientFromRT(CombinedTransport{Auth: auth, Catalog: c})
	a := NewAccount(httpC, NewManualClock(time.Unix(1_700_000_000, 0)), &MemoryCredentialStore{}, FixedOAuthApp())
	_, _ = a.StartDeviceFlow(context.Background()); _, _ = a.PollDeviceFlow(context.Background())
	return NewCatalog(httpC, NewManualClock(time.Unix(1_700_000_000, 0)), &MemoryCacheStore{}, a)
}
func noCred(t *testing.T, v any) { t.Helper(); if ProjectionContainsCredential(v) { t.Fatalf("credential leak %#v", v) } }
