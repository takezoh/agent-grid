package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/takezoh/agent-grid/services/host/runtime"
)

// TestFakeVsRealRepositoryDevcontainer: opt-in Docker fixture fidelity.
// Absent AG_E2E_REPOSITORY_DEVCONTAINER is typed environment_authority_unavailable (no t.Skip).
func TestFakeVsRealRepositoryDevcontainer(t *testing.T) {
	req := runtime.RepositoryEnvironmentRequest{
		ServerID: "s", Repository: testRepo(), ConfigGen: "g", SessionsRoot: "/s", ClonePath: "/s/c",
	}
	if os.Getenv("AG_E2E_REPOSITORY_DEVCONTAINER") == "" {
		fake := runtime.NewFakeRepositoryDevcontainer()
		fake.FailCode = runtime.EnvCodeUnavailable
		res, err := fake.Ensure(context.Background(), req)
		if err != nil || res.Code != runtime.EnvCodeUnavailable {
			t.Fatalf("fake unavailable: %+v err=%v", res, err)
		}
		// Real path without injected authority: nil Devcontainer → environment_authority_unavailable.
		mgr := &runtime.RepositoryEnvironmentManager{
			Store: runtime.NewMemoryRepositoryEnvironmentStore(), Devcontainer: nil,
			ConfigGen: "g", SessionsRoot: "/s",
		}
		realRes, err := mgr.Ensure(context.Background(), "s", testRepo(), "/s/c")
		if err != nil || realRes.Code != runtime.EnvCodeUnavailable {
			t.Fatalf("real unavailable: %+v err=%v", realRes, err)
		}
		return
	}
	// Configured authority: fake success shape matches manager-mediated ensure.
	fake := runtime.NewFakeRepositoryDevcontainer()
	fr, err := fake.Ensure(context.Background(), req)
	if err != nil || fr.Code != runtime.EnvCodeOK || fr.EnvironmentID == "" {
		t.Fatalf("fake ensure: %+v err=%v", fr, err)
	}
	mgr := &runtime.RepositoryEnvironmentManager{
		Store: runtime.NewMemoryRepositoryEnvironmentStore(), Devcontainer: runtime.NewFakeRepositoryDevcontainer(),
		ConfigGen: "g", SessionsRoot: "/s",
	}
	rr, err := mgr.Ensure(context.Background(), "s", testRepo(), "/s/c")
	if err != nil || rr.Code != runtime.EnvCodeOK || rr.EnvironmentID == "" {
		t.Fatalf("manager ensure: %+v err=%v", rr, err)
	}
	if fr.Reused || rr.Reused {
		t.Fatal("first ensure must not be reused")
	}
}
