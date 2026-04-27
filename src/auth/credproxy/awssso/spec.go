package awssso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net"
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
	proxyAddr string // "host:port" from credproxylib.Server.Addr()
	token     string
	runBase   string // parent of per-project run dirs bound into containers at /opt/roost/run
}

// NewSpecBuilder creates a SpecBuilder.
// runBase is the parent of per-project run dirs (e.g. <dataDir>/run).
// proxyAddr may be empty at construction time; call SetProxyAddr once the
// listener port is known before invoking ContainerSpec.
func NewSpecBuilder(proxyAddr, token, runBase string) *SpecBuilder {
	return &SpecBuilder{proxyAddr: proxyAddr, token: token, runBase: runBase}
}

// SetProxyAddr updates the proxy address after the listener port is resolved.
func (b *SpecBuilder) SetProxyAddr(addr string) { b.proxyAddr = addr }

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
		{Path: RoutePath, Provider: New()},
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

	projectRunDir := filepath.Join(b.runBase, projectRunHash(projectPath))
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

	_, port, _ := net.SplitHostPort(b.proxyAddr)
	env := ContainerEnv("http://host.docker.internal:"+port, b.token)
	env["AWS_CONFIG_FILE"] = "/opt/roost/run/aws-config"

	return credproxy.Spec{Env: env}, nil
}

// projectRunHash produces the per-project run dir name (6 bytes → 12 hex chars),
// matching the convention used by runtime.ProjectRunDir.
func projectRunHash(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:6])
}
