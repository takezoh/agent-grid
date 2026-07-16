package secretenv

import (
	"context"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/platform/appid"
	"github.com/takezoh/agent-grid/platform/config"
)

// TestProvider_DoesNotContributePATH pins the case-D regression guard: the
// ContainerSpec return value must not set Env["PATH"]. framelaunch owns the
// runtime PATH invariant via appid.RuntimeAuthoritativePathList()
// (adr-20260716-framelaunch-runtime-path-owner).
func TestProvider_DoesNotContributePATH(t *testing.T) {
	b := NewSpecBuilder(context.Background(),
		Config{
			RunBase:          t.TempDir(),
			ContainerRunDir:  appid.ContainerRunDir,
			ContainerBinPath: appid.ContainerBinaryPath,
		},
		func(string) config.SecretEnvConfig { return config.SecretEnvConfig{} })
	spec, err := b.ContainerSpec(context.Background(), "/host/proj")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if _, ok := spec.Env["PATH"]; ok {
		t.Fatalf("case-D invariant: spec.Env[PATH] = %q, want absent", spec.Env["PATH"])
	}
}

// TestSpecBuilder_ContainerRunDirSSOT_Panic pins
// adr-20260716-provider-shim-root-appid-ssot for secretenv: mismatched
// container run dir MUST panic at construction.
func TestSpecBuilder_ContainerRunDirSSOT_Panic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on mismatched ContainerRunDir; got no panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not string: %T", r)
		}
		if !strings.Contains(msg, "adr-20260716-provider-shim-root-appid-ssot") {
			t.Errorf("panic message %q does not name the ADR", msg)
		}
		if !strings.Contains(msg, "/opt/wrong/run") {
			t.Errorf("panic message %q does not name the offending value", msg)
		}
	}()
	_ = NewSpecBuilder(context.Background(),
		Config{
			RunBase:          t.TempDir(),
			ContainerRunDir:  "/opt/wrong/run",
			ContainerBinPath: "/opt/wrong/run/bridge",
		},
		func(string) config.SecretEnvConfig { return config.SecretEnvConfig{} })
}

// TestSpecBuilder_ContainerRunDirSSOT_Matched confirms the matched case is
// no-op: construction succeeds when cfg.ContainerRunDir == appid.ContainerRunDir.
func TestSpecBuilder_ContainerRunDirSSOT_Matched(t *testing.T) {
	b := NewSpecBuilder(context.Background(),
		Config{
			RunBase:          t.TempDir(),
			ContainerRunDir:  appid.ContainerRunDir,
			ContainerBinPath: appid.ContainerBinaryPath,
		},
		func(string) config.SecretEnvConfig { return config.SecretEnvConfig{} })
	if b == nil {
		t.Fatal("NewSpecBuilder returned nil")
	}
}

// TestShimDirName_MatchesAppidSSOT pins the alias between secretenv's local
// const and appid.SecretEnvShimsDir.
func TestShimDirName_MatchesAppidSSOT(t *testing.T) {
	if shimDirName != appid.SecretEnvShimsDir {
		t.Fatalf("secretenv.shimDirName = %q, want %q (appid.SecretEnvShimsDir SSOT)", shimDirName, appid.SecretEnvShimsDir)
	}
}
