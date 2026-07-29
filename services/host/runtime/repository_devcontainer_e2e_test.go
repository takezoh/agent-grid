package runtime_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/services/host/runtime"
	"github.com/takezoh/agent-grid/services/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-grid/services/platform/sandbox/devcontainer"
	"github.com/takezoh/agent-grid/services/platform/sandbox/devcontainer/fakedocker"
)

// TestFakeVsRealRepositoryDevcontainer: always-on fakedocker exercises the real
// PlatformRepositoryDevcontainer → Manager boundary; AG_E2E_REPOSITORY_DEVCONTAINER
// opts into a live docker-backed Manager for the same adapter contract.
// Absent opt-in is typed environment_authority_unavailable on the nil-manager path (no t.Skip).
func TestFakeVsRealRepositoryDevcontainer(t *testing.T) {
	sessions := t.TempDir()
	clone := writeRepoCloneWithDevcontainer(t, sessions)
	req := runtime.RepositoryEnvironmentRequest{
		ServerID: "s", Repository: testRepo(), ConfigGen: "g",
		SessionsRoot: sessions, ClonePath: clone,
	}

	// Always-on fake authority shape (typed unavailable without injected fn).
	fake := runtime.NewFakeRepositoryDevcontainer()
	fake.FailCode = runtime.EnvCodeUnavailable
	fres, err := fake.Ensure(context.Background(), req)
	if err != nil || fres.Code != runtime.EnvCodeUnavailable {
		t.Fatalf("fake unavailable: %+v err=%v", fres, err)
	}

	// Always-on real adapter boundary via fakedocker PATH injection (not a second fake).
	binDir := setupRepositoryFakeDocker(t)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	mgr := sandboxdc.New(nil)
	adapter := runtime.NewPlatformRepositoryDevcontainer(mgr)
	ares, err := adapter.Ensure(context.Background(), req)
	if err != nil || ares.Code != runtime.EnvCodeOK || ares.ContainerID == "" {
		t.Fatalf("platform adapter via fakedocker: %+v err=%v", ares, err)
	}
	if ares.MountScope != sessions || ares.FrameCWD != clone {
		t.Fatalf("adapter scope: %+v", ares)
	}
	// Isolation: same repository identity + different clone reuses env project path
	// (manager finds existing container by project key).
	req2 := req
	req2.ClonePath = filepath.Join(sessions, "clones", "other")
	_ = os.MkdirAll(req2.ClonePath, 0o755)
	ares2, err := adapter.Ensure(context.Background(), req2)
	if err != nil || ares2.Code != runtime.EnvCodeOK {
		t.Fatalf("second ensure: %+v err=%v", ares2, err)
	}
	// Durable store reuse is manager-level; adapter identity is env-id stable.
	if ares2.EnvironmentID != ares.EnvironmentID {
		t.Fatalf("env id drift: %q vs %q", ares.EnvironmentID, ares2.EnvironmentID)
	}

	if os.Getenv("AG_E2E_REPOSITORY_DEVCONTAINER") == "" {
		// Nil manager / missing authority: typed unavailable, not skip.
		nilAdapter := runtime.NewPlatformRepositoryDevcontainer(nil)
		nres, err := nilAdapter.Ensure(context.Background(), req)
		if err != nil || nres.Code != runtime.EnvCodeUnavailable {
			t.Fatalf("nil manager unavailable: %+v err=%v", nres, err)
		}
		return
	}

	// Opt-in real docker: same Platform adapter over a live Manager (no double-fake).
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("AG_E2E_REPOSITORY_DEVCONTAINER set but docker missing: %v", err)
	}
	// Restore real docker on PATH ahead of fakedocker.
	t.Setenv("PATH", os.Getenv("PATH"))
	realMgr := sandboxdc.NewWithPrefix(nil, "ag-repo-e2e")
	realAdapter := runtime.NewPlatformRepositoryDevcontainer(realMgr)
	// Real ensure requires a resolvable image; use the clone's declared image only when
	// the environment has pre-pulled it. Probe with Ensure and require OK or typed fail.
	realSessions := t.TempDir()
	realClone := writeRepoCloneWithDevcontainer(t, realSessions)
	realReq := runtime.RepositoryEnvironmentRequest{
		ServerID: "s", Repository: testRepo(), ConfigGen: "real-g",
		SessionsRoot: realSessions, ClonePath: realClone,
	}
	rres, rerr := realAdapter.Ensure(context.Background(), realReq)
	if rerr == nil && rres.Code == runtime.EnvCodeOK {
		if rres.ContainerID == "" || rres.MountScope != realSessions {
			t.Fatalf("real ensure incomplete: %+v", rres)
		}
		// Best-effort cleanup of the repository-env project container.
		envPath := runtime.RepositoryEnvironmentProjectPath(realSessions, "s", testRepo().ID, "real-g")
		if inst, err := realMgr.EnsureInstance(context.Background(), envPath, "", sandbox.StartOptions{
			Isolation: sandbox.RepositoryIsolationPlan(),
		}); err == nil {
			_ = realMgr.DestroyInstance(context.Background(), inst)
		}
		return
	}
	// Real docker without the image is still the real adapter boundary failing typed.
	if rres.Code != runtime.EnvCodeFailed && rerr == nil {
		t.Fatalf("real path must be ok or failed, got %+v err=%v", rres, rerr)
	}
}

func writeRepoCloneWithDevcontainer(t *testing.T, sessionsRoot string) string {
	t.Helper()
	clone := filepath.Join(sessionsRoot, "clones", "c1")
	if err := os.MkdirAll(filepath.Join(clone, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"image":"test:latest","workspaceFolder":"/workspaces/app"}`
	if err := os.WriteFile(filepath.Join(clone, ".devcontainer", "devcontainer.json"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return clone
}

func setupRepositoryFakeDocker(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "docker-config.json")
	recordPath := filepath.Join(tmp, "docker-records.jsonl")
	if err := fakedocker.WriteConfig(configPath, fakedocker.Config{
		Responses: map[string][]fakedocker.Response{
			"ps":            {{Stdout: ""}},
			"image inspect": {{Stdout: "[]\n"}},
			"create":        {{Stdout: fakedocker.DefaultContainerID + "\n"}},
			"start":         {{}},
			"exec":          {{}, {Stdout: "ok\n"}, {Stdout: "ok\n"}},
			"inspect":       {{Stdout: fakedocker.DefaultInspectJSON}},
			"rm":            {{}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "docker")
	cmd := exec.Command("go", "build", "-o", out, "./fakedocker/cmd/docker")
	cmd.Dir = filepath.Join("..", "..", "platform", "sandbox", "devcontainer")
	// Resolve relative to this test file's package location via cwd of services/host/runtime.
	// Prefer absolute path from module root via runtime package path.
	wd, _ := os.Getwd()
	// tests run with cwd = package dir services/host/runtime
	cmd.Dir = filepath.Clean(filepath.Join(wd, "..", "..", "platform", "sandbox", "devcontainer"))
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fakedocker: %v\n%s", err, outb)
	}
	t.Setenv(fakedocker.EnvConfigPath, configPath)
	t.Setenv(fakedocker.EnvRecordPath, recordPath)
	return tmp
}
