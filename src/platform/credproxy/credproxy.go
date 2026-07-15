// Package credproxy provides an in-process credential proxy server that
// fans out to per-provider SpecBuilders (AWS SSO, GCP, SSH agent, hostexec,
// MCP proxy). Container paths are accepted as parameters so this package
// does not depend on platform/agentlaunch.
package credproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"sort"
	"time"

	"github.com/takezoh/agent-grid/platform/config"
	"github.com/takezoh/agent-grid/platform/hostexec"
	"github.com/takezoh/agent-grid/platform/mcpproxy"
	"github.com/takezoh/agent-grid/platform/secretenv"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
	"github.com/takezoh/credproxy/providers/awssso"
	"github.com/takezoh/credproxy/providers/gcloudcli"
	"github.com/takezoh/credproxy/providers/sshagent"
)

// Paths holds the container-side paths that providers need. Callers supply
// these from platform/agentlaunch constants so credproxy stays independent.
type Paths struct {
	RunDir  string
	BinPath string
	MCPSock string
}

// ProviderHooks carries the project→workspace resolution that the hostexec and
// MCP providers need but that belongs to the devcontainer orchestration layer
// (platform/agentlaunch), not to this neutral wiring package. credproxy knows
// nothing about shared containers, the project list, or how .mcp.json is placed;
// it only threads these injected functions into each provider's Config. Callers
// build them via agentlaunch.BuildProviderHooks.
type ProviderHooks struct {
	// HostExecWorkspaceFolder returns the container-side workspace path for a
	// project key (used to place hostexec overlay files).
	HostExecWorkspaceFolder func(projectKey string) string
	// MCPWorkspaceTargets returns the workspaces into which the .mcp.json overlay
	// is projected for a project key (one per bound project for shared containers).
	MCPWorkspaceTargets func(projectKey string) []mcpproxy.WorkspaceTarget
}

// ProjectReadiness is a per-project per-provider readiness view derived solely
// from the Runner's own Materialize outcomes. It is not sourced by peeking at
// provider-internal state; see adr-20260715-credproxy-runner-readonly-aggregation.
type ProjectReadiness struct {
	ProjectPath    string
	ProviderName   string
	Materialized   bool
	LastVerifiedAt time.Time
	LastError      string
}

// readinessKey is the aggregation-map key: (projectPath, providerName).
type readinessKey struct {
	project  string
	provider string
}

// Runner holds an in-process credential proxy server and provider SpecBuilders.
type Runner struct {
	srv       *credproxylib.Server
	providers []container.Provider

	// srvCancel cancels the server context; serverDone closes when the server
	// goroutine exits. Together they let Shutdown deterministically reap
	// provider-managed processes (e.g. ssh-agent) on graceful teardown.
	srvCancel  context.CancelFunc
	serverDone chan struct{}

	mu        sync.Mutex
	tokens    map[string]string                 // projectPath → bearer token
	readiness map[readinessKey]ProjectReadiness // caller-observed Materialize outcomes
}

// ProjectToken returns the bearer token for projectPath, generating and
// registering a new one if none exists. Safe for concurrent use.
func (r *Runner) ProjectToken(projectPath string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[projectPath]; ok {
		return t, nil
	}
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("credproxy: generate token for %s: %w", projectPath, err)
	}
	projectID := container.ProjectRunHash(projectPath)
	r.srv.AddAuthToken(token, projectID)
	r.tokens[projectPath] = token
	return token, nil
}

// Start starts an in-process credential proxy and registers all built-in
// providers. resolveSandbox provides per-project SandboxConfig; paths carries
// the container-side paths the providers need.
func Start(ctx context.Context, dataDir string, resolveSandbox func(string) config.SandboxConfig, hooks ProviderHooks, paths Paths) (*Runner, error) {
	runBase := dataDir + "/run"
	if err := os.MkdirAll(runBase, 0o700); err != nil {
		return nil, fmt.Errorf("credproxy: create run dir: %w", err)
	}
	sockPath := filepath.Join(runBase, "credproxy.sock")

	// Providers and the server share a child context so Shutdown (or daemon
	// ctx cancellation) tears down provider-managed processes such as ssh-agent.
	srvCtx, srvCancel := context.WithCancel(ctx)
	runner := &Runner{
		tokens:     make(map[string]string),
		readiness:  make(map[readinessKey]ProjectReadiness),
		srvCancel:  srvCancel,
		serverDone: make(chan struct{}),
	}
	providers := buildProviders(srvCtx, runBase, sockPath, resolveSandbox, runner.ProjectToken, hooks, paths)

	var routes []credproxylib.Route
	for _, p := range providers {
		routes = append(routes, p.Routes()...)
	}

	srv, err := credproxylib.New(credproxylib.ServerConfig{
		ListenUnix: sockPath,
		Routes:     routes,
	})
	if err != nil {
		return nil, fmt.Errorf("credproxy: create server: %w", err)
	}

	for _, p := range providers {
		if r, ok := p.(container.PeriodicRegistrar); ok {
			r.RegisterPeriodic(srv)
		}
	}
	for _, p := range providers {
		if err := p.Init(); err != nil {
			return nil, fmt.Errorf("credproxy: provider %s init: %w", p.Name(), err)
		}
	}

	runner.srv = srv
	runner.providers = providers

	go func() {
		defer close(runner.serverDone)
		_ = srv.Run(srvCtx)
		_ = os.Remove(sockPath)
	}()

	return runner, nil
}

// Shutdown cancels the credproxy server context — which reaps provider-managed
// processes such as the ssh-agent via their context watchers — waits for the
// server goroutine to exit (and remove its socket), then closes any provider
// implementing io.Closer. It is bounded by ctx so a stuck provider cannot block
// daemon shutdown indefinitely. Safe to call on a Runner whose Start failed.
func (r *Runner) Shutdown(ctx context.Context) {
	if r.srvCancel != nil {
		r.srvCancel()
	}
	if r.serverDone != nil {
		select {
		case <-r.serverDone:
		case <-ctx.Done():
		}
	}
	for _, p := range r.providers {
		if c, ok := p.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

func buildProviders(
	ctx context.Context,
	runBase, sockPath string,
	resolveSandbox func(string) config.SandboxConfig,
	tokenFor func(string) (string, error),
	hooks ProviderHooks,
	paths Paths,
) []container.Provider {
	cred := buildCredProviders(ctx, runBase, sockPath, resolveSandbox, tokenFor, paths)
	tool := buildToolProviders(ctx, runBase, resolveSandbox, hooks, paths)
	return append(cred, tool...)
}

func buildCredProviders(
	ctx context.Context,
	runBase, sockPath string,
	resolveSandbox func(string) config.SandboxConfig,
	tokenFor func(string) (string, error),
	paths Paths,
) []container.Provider {
	aws := awssso.NewSpecBuilder(
		awssso.Config{
			HostRunBase:       runBase,
			HostSockPath:      sockPath,
			ContainerRunDir:   paths.RunDir,
			ContainerSockPath: paths.RunDir + "/credproxy.sock",
		},
		func(p string) []string { return resolveSandbox(p).Proxy.AWSProfiles },
		tokenFor,
	)
	gcp := gcloudcli.NewSpecBuilder(
		ctx,
		gcloudcli.Config{RunBase: runBase, ContainerRunDir: paths.RunDir},
		func(p string) gcloudcli.GCPConfig {
			g := resolveSandbox(p).Proxy.GCP
			return gcloudcli.GCPConfig{Account: g.Account, ServiceAccount: g.ServiceAccount, Active: g.Active, Projects: g.Projects}
		},
	)
	ssh := sshagent.NewSpecBuilder(
		ctx,
		sshagent.Config{RunBase: runBase, ContainerRunDir: paths.RunDir},
		func(p string) []string { return resolveSandbox(p).Proxy.SSHAgent.Keys },
	)
	return []container.Provider{aws, gcp, ssh}
}

func buildToolProviders(
	ctx context.Context,
	runBase string,
	resolveSandbox func(string) config.SandboxConfig,
	hooks ProviderHooks,
	paths Paths,
) []container.Provider {
	he := hostexec.NewSpecBuilder(ctx,
		hostexec.Config{RunBase: runBase, ContainerRunDir: paths.RunDir, ContainerBinPath: paths.BinPath, WorkspaceFolderFor: hooks.HostExecWorkspaceFolder},
		func(p string) config.HostExecConfig { return resolveSandbox(p).Proxy.HostExec },
	)
	mcp := mcpproxy.NewSpecBuilder(ctx,
		mcpproxy.Config{
			RunBase:             runBase,
			ContainerSockPath:   paths.MCPSock,
			ContainerBinPath:    paths.BinPath,
			WorkspaceTargetsFor: hooks.MCPWorkspaceTargets,
		},
		func(p string) config.MCPProxyConfig { return resolveSandbox(p).Proxy.MCPProxy },
	)
	se := secretenv.NewSpecBuilder(ctx,
		secretenv.Config{
			RunBase:          runBase,
			ContainerRunDir:  paths.RunDir,
			ContainerBinPath: paths.BinPath,
			HostPathMountPrefixFor: func(p string) string {
				return resolveSandbox(p).Devcontainer.HostPathMountPrefix
			},
		},
		func(p string) config.SecretEnvConfig { return resolveSandbox(p).Proxy.SecretEnv },
	)
	return []container.Provider{he, mcp, se}
}

// ContainerSpec fans out to all providers and merges their Env, Mounts, and BridgeSpecs.
// Provider errors are logged as warnings and do not abort the other providers.
func (r *Runner) ContainerSpec(ctx context.Context, projectPath string) (container.Spec, error) {
	out := container.Spec{Env: map[string]string{}}
	for _, p := range r.providers {
		s, err := p.ContainerSpec(ctx, projectPath)
		if err != nil {
			slog.Warn("credproxy: provider failed", "provider", p.Name(), "project", projectPath, "err", err)
			continue
		}
		for k, v := range s.Env {
			out.Env[k] = v
		}
		out.Mounts = append(out.Mounts, s.Mounts...)
		out.BridgeSpecs = append(out.BridgeSpecs, s.BridgeSpecs...)
	}
	return out, nil
}

// Materialize fans out to every provider's Materialize method and aggregates the
// outcomes into the readiness map. The caller controls the retry envelope; this
// method does NOT retry internally (see adr-20260715-credproxy-retry-owner-caller-side).
// Providers whose Materialize returns nil for a project that is not configured
// for them (e.g. gcloudcli with no Proxy.GCP) do not appear in ReadinessSnapshot —
// silence = healthy per adr-20260715-credproxy-runner-readonly-aggregation.
//
// Returns the first non-nil provider error encountered so the caller can decide
// whether to retry; all outcomes are still recorded even when one provider fails.
func (r *Runner) Materialize(ctx context.Context, projectPath string) error {
	var firstErr error
	for _, p := range r.providers {
		err := p.Materialize(ctx, projectPath)
		if err != nil {
			// grep-distinguishable from the generic "credproxy: provider failed"
			// line emitted by ContainerSpec — this one signals the credential
			// surface is not usable inside the container.
			slog.Warn("credproxy: credential materialization unconfirmed",
				"provider", p.Name(), "project", projectPath, "err", err)
			r.recordReadiness(projectPath, p.Name(), false, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		r.recordReadiness(projectPath, p.Name(), true, nil)
	}
	return firstErr
}

// recordReadiness updates the aggregation map with a single outcome.
// The map only contains entries for (project, provider) pairs that this Runner
// has actually invoked Materialize on — no invented entries for opt-out providers.
func (r *Runner) recordReadiness(projectPath, providerName string, ok bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr := ProjectReadiness{
		ProjectPath:    projectPath,
		ProviderName:   providerName,
		Materialized:   ok,
		LastVerifiedAt: time.Now(),
	}
	if err != nil {
		pr.LastError = err.Error()
	}
	r.readiness[readinessKey{project: projectPath, provider: providerName}] = pr
}

// ReadinessSnapshot returns a defensive copy of every (project, provider) entry
// that this Runner has observed via Materialize. The slice is ordered by
// (ProjectPath, ProviderName) so callers can diff snapshots deterministically.
// The Runner's internal map is not exposed; mutating the returned slice has no
// effect on subsequent snapshots.
func (r *Runner) ReadinessSnapshot() []ProjectReadiness {
	r.mu.Lock()
	out := make([]ProjectReadiness, 0, len(r.readiness))
	for _, v := range r.readiness {
		out = append(out, v)
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProjectPath != out[j].ProjectPath {
			return out[i].ProjectPath < out[j].ProjectPath
		}
		return out[i].ProviderName < out[j].ProviderName
	})
	return out
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
