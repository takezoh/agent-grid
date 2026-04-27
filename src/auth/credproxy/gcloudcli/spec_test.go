package gcloudcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/config"
)

func TestSpecBuilder_emptyAccount_zeroSpec(t *testing.T) {
	b := NewSpecBuilder(context.Background(), t.TempDir(), t.TempDir())
	spec, err := b.ContainerSpec(context.Background(), "/proj", config.SandboxConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Env) != 0 || len(spec.Mounts) != 0 {
		t.Errorf("expected zero spec, got env=%v mounts=%v", spec.Env, spec.Mounts)
	}
}

func TestSpecBuilder_emptyProjects_zeroSpec(t *testing.T) {
	b := NewSpecBuilder(context.Background(), t.TempDir(), t.TempDir())
	sb := config.SandboxConfig{
		Proxy: config.ProxyConfig{GCP: config.GCPConfig{Account: "user@example.com"}},
	}
	spec, err := b.ContainerSpec(context.Background(), "/proj", sb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Env) != 0 || len(spec.Mounts) != 0 {
		t.Errorf("expected zero spec, got env=%v mounts=%v", spec.Env, spec.Mounts)
	}
}

func TestSpecBuilder_withConfig_injectsEnvAndFiles(t *testing.T) {
	stubGcloudForSpec(t, "gcp-test-token")

	gcpDir := t.TempDir()
	runBase := t.TempDir()
	b := NewSpecBuilder(context.Background(), gcpDir, runBase)

	sb := config.SandboxConfig{
		Proxy: config.ProxyConfig{
			GCP: config.GCPConfig{
				Account:  "user@example.com",
				Projects: []string{"proj-a", "proj-b"},
			},
		},
	}

	spec, err := b.ContainerSpec(context.Background(), "/myproject", sb)
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}

	if spec.Env[ConfigDirEnv] != containerConfigPath {
		t.Errorf("env[%s] = %q, want %q", ConfigDirEnv, spec.Env[ConfigDirEnv], containerConfigPath)
	}
	// Files are in the per-project run dir; the dir bind covers them — no per-file mounts.
	if len(spec.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d: %v", len(spec.Mounts), spec.Mounts)
	}

	// Verify gcloud-config dir was written under the per-project run dir.
	projectDir := filepath.Join(runBase, projectRunHash("/myproject"))
	if _, err := os.Stat(filepath.Join(projectDir, "gcloud-config")); err != nil {
		t.Errorf("gcloud-config dir not created in run dir: %v", err)
	}
}

func TestSpecBuilder_refresherDeduplication(t *testing.T) {
	stubGcloudForSpec(t, "tok")

	b := NewSpecBuilder(context.Background(), t.TempDir(), t.TempDir())
	sb := config.SandboxConfig{
		Proxy: config.ProxyConfig{
			GCP: config.GCPConfig{Account: "user@example.com", Projects: []string{"p"}},
		},
	}

	// Call twice — refresher goroutine should only start once.
	if _, err := b.ContainerSpec(context.Background(), "/p1", sb); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ContainerSpec(context.Background(), "/p2", sb); err != nil {
		t.Fatal(err)
	}

	b.mu.Lock()
	count := len(b.refreshers)
	b.mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 refresher, got %d", count)
	}
}

// stubGcloudForSpec writes a fake gcloud script and prepends its dir to PATH.
func stubGcloudForSpec(t *testing.T, token string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "gcloud")
	content := "#!/bin/sh\necho " + token + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub gcloud: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}
