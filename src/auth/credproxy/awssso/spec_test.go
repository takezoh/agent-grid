package awssso

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
)

func TestSpecBuilder_emptyProfiles_zeroSpec(t *testing.T) {
	runBase := t.TempDir()
	b := NewSpecBuilder(filepath.Join(runBase, "credproxy.sock"), runBase, func(string) (string, error) { return "tok", nil })
	spec, err := b.ContainerSpec(context.Background(), "/proj", config.SandboxConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Env) != 0 || len(spec.Mounts) != 0 {
		t.Errorf("expected zero spec, got env=%v mounts=%v", spec.Env, spec.Mounts)
	}
}

func TestSpecBuilder_withProfiles_returnsEnvAndFiles(t *testing.T) {
	withFakeAWS(t)
	runBase := t.TempDir()
	sockPath := filepath.Join(runBase, "credproxy.sock")
	b := NewSpecBuilder(sockPath, runBase, func(string) (string, error) { return "mytoken", nil })
	sb := config.SandboxConfig{
		Proxy: config.ProxyConfig{AWSProfiles: []string{"default", "prod"}},
	}

	spec, err := b.ContainerSpec(context.Background(), "/myproject", sb)
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}

	if spec.Env["ROOST_AWS_TOKEN"] != "mytoken" {
		t.Errorf("ROOST_AWS_TOKEN = %q, want %q", spec.Env["ROOST_AWS_TOKEN"], "mytoken")
	}
	if spec.Env["ROOST_PROXY_SOCK"] != ContainerSockPath {
		t.Errorf("ROOST_PROXY_SOCK = %q, want %q", spec.Env["ROOST_PROXY_SOCK"], ContainerSockPath)
	}
	if spec.Env["AWS_CONFIG_FILE"] != "/opt/roost/run/aws-config" {
		t.Errorf("AWS_CONFIG_FILE = %q, want /opt/roost/run/aws-config", spec.Env["AWS_CONFIG_FILE"])
	}
	// One file bind mount for the credproxy socket.
	wantMount := "type=bind,source=" + sockPath + ",target=" + ContainerSockPath
	if len(spec.Mounts) != 1 || spec.Mounts[0] != wantMount {
		t.Errorf("Mounts = %v, want [%q]", spec.Mounts, wantMount)
	}

	// Verify config file and helper script were written under the per-project run dir.
	projectDir := filepath.Join(runBase, credproxy.ProjectRunHash("/myproject"))
	if _, err := os.Stat(filepath.Join(projectDir, "aws-config")); err != nil {
		t.Errorf("aws-config not created in run dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "aws-creds.sh")); err != nil {
		t.Errorf("aws-creds.sh not created in run dir: %v", err)
	}

	// Profiles in the config must be accepted for this project's token_id.
	projectID := credproxy.ProjectRunHash("/myproject")
	provider := b.Routes()[0].Provider.(*Provider)
	_, err = provider.Get(context.Background(), req(projectID, "/prod"))
	if err != nil {
		t.Errorf("provider.Get(/prod): unexpected error: %v", err)
	}
	_, err = provider.Get(context.Background(), req(projectID, "/"))
	if err != nil {
		t.Errorf("provider.Get(/): unexpected error (default should be allowed): %v", err)
	}
	// Profile not in aws_profiles must be rejected.
	_, err = provider.Get(context.Background(), req(projectID, "/other"))
	if err == nil {
		t.Errorf("provider.Get(/other): expected error for unlisted profile, got nil")
	}
}

// TestSpecBuilder_NoProfiles_provider_remains_strict verifies that when ContainerSpec is never
// called with profiles, the provider rejects all requests.
func TestSpecBuilder_NoProfiles_provider_remains_strict(t *testing.T) {
	runBase := t.TempDir()
	b := NewSpecBuilder(filepath.Join(runBase, "credproxy.sock"), runBase, func(string) (string, error) { return "tok", nil })

	provider := b.Routes()[0].Provider.(*Provider)
	_, err := provider.Get(context.Background(), req("unknown-project", "/master"))
	if err == nil {
		t.Errorf("expected rejection for unlisted profile on empty allowlist, got nil")
	}
	_, err = provider.Get(context.Background(), req("unknown-project", "/"))
	if err == nil {
		t.Errorf("expected rejection for default profile on empty allowlist, got nil")
	}
}
