package webhost
import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
	"github.com/takezoh/agent-grid/clients/web-host/internal/githubhost"
)
const leaseSentinel = "gho_SENTINEL_WEB_HOST_7f3a9c"
type staticCred struct{ token, gen string }
func (s staticCred) HostCredential() (string, bool) {
	if s.token == "" {
		return "", false
	}
	return s.token, true
}
func (s staticCred) CredentialGeneration() (string, bool) {
	if s.gen == "" {
		return "", false
	}
	return s.gen, true
}
type roundTripFn func(*http.Request) (*http.Response, error)
func (f roundTripFn) Do(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResp(status int, body any) *http.Response {
	raw, _ := json.Marshal(body)
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(raw)),
		Header: http.Header{"Content-Type": []string{"application/json"}}}
}
// rejects counterexample: GitHub failure dropping Projects source
func TestHostRepositoryProjectionParity(t *testing.T) {
	t.Parallel()
	projects := []HostProjectTarget{{"srv-a", "/repo/demo"}}
	got := BuildHostRepositoryProjection(projects,
		githubhost.AccountProjection{State: githubhost.StateAuthenticated, Login: "example-user"},
		githubhost.CatalogProjection{
			Outcome: githubhost.CatalogSourceFailedRetained, Generation: 1, Stale: true,
			FailureCode: githubhost.FailSourceFailed,
			Repositories: []githubhost.Repository{{ID: "1", Owner: "example-user", Name: "demo"}},
		},
		[]HostPreparation{{
			PreparationID: "prep-1", IntentID: "intent-1", ServerID: "srv-a",
			Repository: HostRepositoryIdentity{"1", "example-user", "demo"},
			Stage: "clone-in-progress", Revision: 2, EnvironmentDisposition: "none",
		}})
	if githubhost.ProjectionContainsCredential(got) || len(got.Sources) != 2 || !got.Sources[0].Available ||
		got.Sources[0].ID != "projects" || !got.Sources[1].Available || len(got.Targets) != 2 {
		t.Fatalf("%#v", got)
	}
	failed := BuildHostRepositoryProjection(projects,
		githubhost.AccountProjection{State: githubhost.StateRefreshFailed},
		githubhost.CatalogProjection{Outcome: githubhost.CatalogAuthenticationRequiredRetained}, nil)
	if !failed.Sources[0].Available || len(failed.Targets) != 1 || failed.Targets[0].Source != "project" {
		t.Fatalf("projects reduced: %#v", failed)
	}
	raw, _ := json.Marshal(got)
	var again HostRepositoryProjection
	_ = json.Unmarshal(raw, &again)
	raw2, _ := json.Marshal(again)
	if !bytes.Equal(raw, raw2) {
		t.Fatal("round-trip drift")
	}
}
// rejects counterexample: auth.mode=none / protocol mismatch / insecure minting a lease
func TestRepositoryLeasePreflightAndNoTransfer(t *testing.T) {
	t.Parallel()
	serverOK := clientconfig.Server{ID: "s", URL: "http://127.0.0.1:9", Auth: clientconfig.Auth{Mode: "bearer", Token: "x"}}
	var transferred bool
	m := &RepositoryLeaseMediator{Auth: staticCred{leaseSentinel, "3"}, HostID: "host-1",
		Doer: roundTripFn(func(r *http.Request) (*http.Response, error) {
			transferred = true
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), leaseSentinel) {
				t.Fatal("token missing upstream")
			}
			return jsonResp(http.StatusCreated, map[string]any{"lease_id": "lease-1", "expires_at": time.Now().UTC()}), nil
		})}
	req := LeaseRequest{RepositoryID: "1", Owner: "o", Name: "n"}
	for _, tc := range []struct {
		s    clientconfig.Server
		p, c string
	}{
		{clientconfig.Server{ID: "s", URL: "http://127.0.0.1:9", Auth: clientconfig.Auth{Mode: "none"}}, protocolVersion, leaseFailAuth},
		{serverOK, "protocol-legacy", leaseFailProtocol},
		{clientconfig.Server{ID: "s", URL: "http://example.com", Auth: clientconfig.Auth{Mode: "bearer", Token: "x"}}, protocolVersion, leaseFailAuth},
	} {
		transferred = false
		if got := m.RequestLease(context.Background(), tc.s, tc.p, req); got.Code != tc.c || got.Issue != nil || transferred {
			t.Fatalf("%s: %#v xfer=%v", tc.c, got, transferred)
		}
	}
	if got := (&RepositoryLeaseMediator{Auth: staticCred{}, Doer: m.Doer}).
		RequestLease(context.Background(), serverOK, protocolVersion, req); got.Code != leaseFailNoCred {
		t.Fatalf("no cred %#v", got)
	}
	ok := m.RequestLease(context.Background(), serverOK, protocolVersion, req)
	if ok.Issue == nil || ok.Issue.LeaseID != "lease-1" || githubhost.ProjectionContainsCredential(ok) {
		t.Fatalf("success %#v", ok)
	}
	if b, _ := json.Marshal(ok); strings.Contains(string(b), leaseSentinel) {
		t.Fatal("sentinel leaked")
	}
	if PreflightRepositoryLease("bearer", protocolVersion, "https://git.example") != nil {
		t.Fatal("https")
	}
}
