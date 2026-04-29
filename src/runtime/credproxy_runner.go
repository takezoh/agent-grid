package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/auth/credproxy/awssso"
	"github.com/takezoh/agent-roost/auth/credproxy/gcloudcli"
	"github.com/takezoh/agent-roost/auth/credproxy/sshagent"
	"github.com/takezoh/agent-roost/auth/credproxy/winexec"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// CredProxyRunner holds an in-process credential proxy server and a set of
// provider-specific SpecBuilders. Each provider encapsulates all knowledge of
// its credential system; this runner fans out ContainerSpec calls and merges results.
type CredProxyRunner struct {
	srv       *credproxylib.Server
	providers []credproxy.Provider
}

// StartCredProxy starts an in-process credential proxy and registers all built-in
// providers. The returned runner's ContainerSpec method is the only entry point
// for docker_launcher — it contains no provider-specific logic.
func StartCredProxy(ctx context.Context, dataDir string) (*CredProxyRunner, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("credproxy: generate token: %w", err)
	}

	runBase := dataDir + "/run"
	sockPath := filepath.Join(runBase, "credproxy.sock")

	awsSpec := awssso.NewSpecBuilder(sockPath, token, runBase)
	gcpSpec := gcloudcli.NewSpecBuilder(ctx, dataDir+"/gcp", runBase)
	sshSpec := sshagent.NewSpecBuilder(ctx, runBase)
	winExecSpec := winexec.NewSpecBuilder(ctx, runBase)
	providers := []credproxy.Provider{awsSpec, gcpSpec, sshSpec, winExecSpec}

	var routes []credproxylib.Route
	for _, p := range providers {
		routes = append(routes, p.Routes()...)
	}

	srv, err := credproxylib.New(credproxylib.ServerConfig{
		ListenUnix: sockPath,
		AuthTokens: []string{token},
		Routes:     routes,
	})
	if err != nil {
		return nil, fmt.Errorf("credproxy: create server: %w", err)
	}

	for _, p := range providers {
		if err := p.Init(); err != nil {
			return nil, fmt.Errorf("credproxy: provider %s init: %w", p.Name(), err)
		}
	}

	go func() {
		_ = srv.Run(ctx)
		_ = os.Remove(sockPath)
	}()

	return &CredProxyRunner{srv: srv, providers: providers}, nil
}

// ContainerSpec fans out to all providers and merges their Env and Mounts.
// Provider errors are logged as warnings and do not abort the other providers.
func (r *CredProxyRunner) ContainerSpec(ctx context.Context, projectPath string, sb config.SandboxConfig) (credproxy.Spec, error) {
	out := credproxy.Spec{Env: map[string]string{}}
	for _, p := range r.providers {
		s, err := p.ContainerSpec(ctx, projectPath, sb)
		if err != nil {
			slog.Warn("credproxy: provider failed", "provider", p.Name(), "project", projectPath, "err", err)
			continue
		}
		for k, v := range s.Env {
			out.Env[k] = v
		}
		out.Mounts = append(out.Mounts, s.Mounts...)
	}
	return out, nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
