package awssso

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// SpecBuilder implements credproxy.Provider for AWS SSO.
// It generates a synthetic AWS config and credential helper per project
// when aws_profiles are configured.
type SpecBuilder struct {
	sockHostPath string    // host-side Unix socket path (e.g. <dataDir>/run/credproxy.sock)
	token        string
	runBase      string    // parent of per-project run dirs bound into containers at /opt/roost/run
	provider     *Provider // shared Provider whose allowlist grows with each ContainerSpec call
}

// NewSpecBuilder creates a SpecBuilder.
// sockHostPath is the host-side Unix socket path for the credproxy listener
// (e.g. <dataDir>/run/credproxy.sock); it is bind-mounted per-project into
// the container at containerSockPath.
// runBase is the parent of per-project run dirs (e.g. <dataDir>/run).
func NewSpecBuilder(sockHostPath, token, runBase string) *SpecBuilder {
	return &SpecBuilder{
		sockHostPath: sockHostPath,
		token:        token,
		runBase:      runBase,
		provider:     New(nil),
	}
}

func (b *SpecBuilder) Name() string { return "awssso" }

// Init creates runBase.
func (b *SpecBuilder) Init() error {
	if err := os.MkdirAll(b.runBase, 0o700); err != nil {
		return fmt.Errorf("awssso: mkdir: %w", err)
	}
	return nil
}

// Routes returns the HTTP route that serves AWS credentials to containers.
func (b *SpecBuilder) Routes() []credproxylib.Route {
	return []credproxylib.Route{
		{Path: RoutePath, Provider: b.provider},
	}
}

// ContainerSpec implements credproxy.Provider.
// Returns zero Spec when sandbox.proxy.aws_profiles is empty.
// Files are written into the per-project run dir so the single dir bind at
// /opt/roost/run covers them without additional per-file mounts.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string, sb config.SandboxConfig) (credproxy.Spec, error) {
	profiles := sb.Proxy.AWSProfiles
	if len(profiles) == 0 {
		return credproxy.Spec{}, nil
	}
	b.provider.AllowProfiles(profiles)

	projectRunDir := filepath.Join(b.runBase, credproxy.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projectRunDir, 0o700); err != nil {
		return credproxy.Spec{}, fmt.Errorf("awssso: mkdir run dir: %w", err)
	}

	if err := WriteHelperScript(filepath.Join(projectRunDir, "aws-creds.sh")); err != nil {
		return credproxy.Spec{}, fmt.Errorf("awssso: write helper: %w", err)
	}

	var buf bytes.Buffer
	if err := RenderConfig(&buf, profiles, "/opt/roost/run/aws-creds.sh"); err != nil {
		return credproxy.Spec{}, fmt.Errorf("awssso: render config for %s: %w", projectPath, err)
	}
	if err := os.WriteFile(filepath.Join(projectRunDir, "aws-config"), buf.Bytes(), 0o644); err != nil {
		return credproxy.Spec{}, fmt.Errorf("awssso: write config for %s: %w", projectPath, err)
	}

	env := ContainerEnv(b.token)
	env["AWS_CONFIG_FILE"] = "/opt/roost/run/aws-config"

	mount := fmt.Sprintf("type=bind,source=%s,target=%s", b.sockHostPath, ContainerSockPath)
	return credproxy.Spec{Env: env, Mounts: []string{mount}}, nil
}

