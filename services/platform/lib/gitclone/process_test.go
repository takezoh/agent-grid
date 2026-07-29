package gitclone

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cloneSentinel = "SENTINEL_CLONE_TOKEN_b91e2d"

func TestMain(m *testing.M) {
	if len(os.Args) >= 2 && os.Args[1] == "internal-git-askpass" {
		os.Exit(RunAskpassClient(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// TestGitCloneContract: remote derivation + sentinel-free surfaces.
// rejects counterexample: token in remote URL or env values
func TestGitCloneContract(t *testing.T) {
	t.Parallel()
	if r, err := CanonicalHTTPSRemote("acme", "widgets"); err != nil || r != "https://github.com/acme/widgets.git" {
		t.Fatalf("%q %v", r, err)
	}
	if _, err := CanonicalHTTPSRemote("../x", "y"); err == nil {
		t.Fatal("invalid")
	}
	fake := &FakeGitClone{}
	dest := filepath.Join(t.TempDir(), "repo")
	res := fake.Clone(context.Background(), CloneRequest{Owner: "acme", Name: "widgets", DestDir: dest, Cred: Credential{Username: "x-access-token", Token: cloneSentinel}})
	if res.Code != CodeOK || ContainsSentinel(fake.Captured.Surfaces, cloneSentinel) {
		t.Fatalf("%+v", res)
	}
	script := &ScriptedRunner{Code: 0}
	res = Service{}.Clone(context.Background(), CloneRequest{Owner: "acme", Name: "widgets", DestDir: dest, Cred: Credential{Username: "x-access-token", Token: cloneSentinel}, Runner: script, Timeout: time.Second})
	joined := strings.Join(script.Argv, " ") + "\n" + strings.Join(script.Env, "\n")
	if res.Code != CodeOK || strings.Contains(joined, cloneSentinel) {
		t.Fatalf("%+v %s", res, joined)
	}
}

// TestCloneCredentialProcessBoundaryContract: one-use peer/nonce broker.
// rejects counterexample: replay / peer mismatch receive no credential
func TestCloneCredentialProcessBoundaryContract(t *testing.T) {
	t.Parallel()
	cred := Credential{Username: "x-access-token", Token: cloneSentinel}
	broker, prep, err := StartBroker(cred)
	if err != nil {
		t.Fatal(err)
	}
	defer prep.Cleanup()
	if fi, _ := os.Stat(prep.Dir); fi.Mode().Perm() != 0o700 {
		t.Fatal("dir mode")
	}
	for _, e := range prep.Env {
		if strings.Contains(e, cloneSentinel) {
			t.Fatal(e)
		}
	}
	broker.BindGit(os.Getpid(), os.Getpid())
	user, pass, err := dialAskpass(prep.SockPath, prep.ExtraFiles[0])
	if err != nil || user != "x-access-token" || pass != cloneSentinel || broker.LastCode() != CodeOK {
		t.Fatalf("%q %q %s %v", user, pass, broker.LastCode(), err)
	}
	if c, err := net.Dial("unix", prep.SockPath); err == nil {
		_, _ = c.Write([]byte("00\n"))
		_ = c.Close()
		time.Sleep(20 * time.Millisecond)
		if code := broker.LastCode(); code != CodeReplay && code != "nonce_mismatch" {
			t.Fatal(code)
		}
	}
	b2, p2, err := StartBroker(cred)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Cleanup()
	b2.BindGit(1, 1)
	c, err := net.Dial("unix", p2.SockPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write([]byte("deadbeef\n"))
	_ = c.Close()
	time.Sleep(20 * time.Millisecond)
	if b2.LastCode() != CodePeerMismatch {
		t.Fatal(b2.LastCode())
	}
}

// TestGitCloneAuthorityAndFailureContract: empty credential and non-zero exit are typed.
// rejects counterexample: silent success / raw secret in diagnostics
func TestGitCloneAuthorityAndFailureContract(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "r")
	fakeRes := (&FakeGitClone{}).Clone(context.Background(), CloneRequest{Owner: "acme", Name: "widgets", DestDir: dest, Cred: Credential{}})
	if fakeRes.Code != CodeAuthorityUnavailable {
		t.Fatalf("fake absent authority: %q", fakeRes.Code)
	}
	svcRes := Service{}.Clone(context.Background(), CloneRequest{Owner: "acme", Name: "widgets", DestDir: dest, Cred: Credential{}})
	if svcRes.Code != CodeAuthorityUnavailable {
		t.Fatalf("service absent authority: %q", svcRes.Code)
	}
	fail := Service{}.Clone(context.Background(), CloneRequest{
		Owner: "acme", Name: "widgets", DestDir: dest,
		Cred:   Credential{Username: "x-access-token", Token: "tok"},
		Runner: &ScriptedRunner{Code: 1}, Timeout: time.Second,
	})
	if fail.Code != CodeCloneFailed {
		t.Fatalf("nonzero exit: %+v", fail)
	}
	if s := SanitizeDiagnostic("err token=secret-x", "secret-x"); strings.Contains(s, "secret-x") {
		t.Fatal("sanitize must redact secrets")
	}
}

// TestCloneResultOmitsArgvEnvOnSentinelLeak: credential surface leak must not
// return unredacted Argv/Env that embed the token.
func TestCloneResultOmitsArgvEnvOnSentinelLeak(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "r")
	leaky := &ScriptedRunner{
		Code: 0,
		Out:  []byte("cloned with " + cloneSentinel),
	}
	res := Service{}.Clone(context.Background(), CloneRequest{
		Owner: "acme", Name: "widgets", DestDir: dest,
		Cred:   Credential{Username: "x-access-token", Token: cloneSentinel},
		Runner: leaky, Timeout: time.Second,
	})
	if res.Code != CodeCloneFailed || res.Diagnostic != CodeCredentialSurfaceLeak {
		t.Fatalf("want surface leak: %+v", res)
	}
	if len(res.Argv) != 0 || len(res.Env) != 0 {
		t.Fatalf("Argv/Env must be omitted on leak: argv=%v env=%v", res.Argv, res.Env)
	}
	if strings.Contains(res.Diagnostic, cloneSentinel) {
		t.Fatal("diagnostic must not embed token")
	}
}

// TestBrokerCloseRacesCredentialTake: Close concurrent with handle must not
// race on cred (run with -race).
func TestBrokerCloseRacesCredentialTake(t *testing.T) {
	t.Parallel()
	cred := Credential{Username: "x-access-token", Token: cloneSentinel}
	for i := 0; i < 32; i++ {
		b, prep, err := StartBroker(cred)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = b.takeCredential()
		}()
		b.Close()
		<-done
		prep.Cleanup()
	}
}

func dialAskpass(sock string, nonceFile *os.File) (string, string, error) {
	nonceLine, err := bufio.NewReader(nonceFile).ReadString('\n')
	if err != nil {
		return "", "", err
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		return "", "", err
	}
	defer c.Close()
	if _, err := c.Write([]byte(nonceLine)); err != nil {
		return "", "", err
	}
	br := bufio.NewReader(c)
	user, err := br.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	pass, err := br.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(user), strings.TrimSpace(pass), nil
}
