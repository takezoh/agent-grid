package gitclone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFakeVsRealGitClone: opt-in T3 fidelity; absent authority is typed unavailable (no t.Skip).
func TestFakeVsRealGitClone(t *testing.T) {
	repo, token := os.Getenv("AG_E2E_GIT_CLONE_PRIVATE_REPO"), os.Getenv("AG_E2E_GIT_CLONE_TOKEN")
	if repo == "" || token == "" {
		dest := filepath.Join(t.TempDir(), "r")
		fakeRes := (&FakeGitClone{}).Clone(context.Background(), CloneRequest{Owner: "acme", Name: "private-fixture", DestDir: dest, Cred: Credential{}})
		realRes := Service{}.Clone(context.Background(), CloneRequest{Owner: "acme", Name: "private-fixture", DestDir: dest, Cred: Credential{}})
		if fakeRes.Code != CodeAuthorityUnavailable || realRes.Code != CodeAuthorityUnavailable {
			t.Fatalf("absent authority: fake=%q real=%q", fakeRes.Code, realRes.Code)
		}
		return
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		t.Fatal(repo)
	}
	cred := Credential{Username: "x-access-token", Token: token}
	fr := (&FakeGitClone{}).Clone(context.Background(), CloneRequest{Owner: parts[0], Name: parts[1], DestDir: filepath.Join(t.TempDir(), "f"), Cred: cred})
	rr := Service{}.Clone(context.Background(), CloneRequest{Owner: parts[0], Name: parts[1], DestDir: filepath.Join(t.TempDir(), "r"), Cred: cred, Timeout: 2 * time.Minute})
	if fr.Code != CodeOK || rr.Code != CodeOK || fr.Remote != rr.Remote {
		t.Fatalf("%+v %+v", fr, rr)
	}
	if ContainsSentinel([]string{rr.Diagnostic, strings.Join(rr.Env, "\n"), strings.Join(rr.Argv, " ")}, token) {
		t.Fatal("leak")
	}
}
