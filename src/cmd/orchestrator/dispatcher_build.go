package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/credproxy"
	sandboxdc "github.com/takezoh/agent-reactor/platform/sandbox/devcontainer"
)

// credproxyShutdownTimeout bounds how long graceful teardown waits for the
// credproxy server goroutine to exit before giving up.
const credproxyShutdownTimeout = 5 * time.Second

// buildDispatcher constructs the Dispatcher for the orchestrator.
// It loads the shared sandbox config from ~/.agent-reactor/settings.toml and enables
// the devcontainer backend when the workspace project is configured for it.
// Returns the dispatcher and a cleanup func that stops any background services.
func buildDispatcher(ctx context.Context, workspaceRoot string) (agentlaunch.Dispatcher, func(), error) {
	settings, err := platformconfig.LoadUserSettings()
	if err != nil {
		return nil, nil, fmt.Errorf("orchestrator: load user settings: %w", err)
	}

	resolver := platformconfig.NewSandboxResolver(settings.Sandbox)
	d := &agentlaunch.SandboxDispatcher{
		Resolver: resolver,
		Direct:   agentlaunch.DirectDispatcher{},
	}

	if resolver.Resolve(workspaceRoot).Mode != "devcontainer" {
		return d, func() {}, nil
	}

	devLauncher, cleanup, err := buildDevcontainerLauncher(ctx, resolver, settings.ResolveDataDir())
	if err != nil {
		return nil, nil, err
	}

	d.Devcontainer = devLauncher
	slog.Info("sandbox: devcontainer backend enabled")
	return d, cleanup, nil
}

func buildDevcontainerLauncher(
	ctx context.Context,
	resolver *platformconfig.SandboxResolver,
	dataDir string,
) (*agentlaunch.DevcontainerLauncher, func(), error) {
	if err := sandboxdc.CheckAvailable(); err != nil {
		return nil, nil, err
	}

	currentHost := os.Getenv("DOCKER_HOST")
	if host := platformconfig.ResolveDockerHost(
		currentHost,
		os.Getenv("XDG_RUNTIME_DIR"),
		func(p string) bool { _, err := os.Stat(p); return err == nil },
	); host != "" {
		_ = os.Setenv("DOCKER_HOST", host)
		slog.Info("sandbox: rootless docker detected", "DOCKER_HOST", host)
	} else if currentHost == "" {
		slog.Info("sandbox: using default docker socket (rootless not detected)")
	}

	// Empty ProjectsConfig: the orchestrator launches one workspace per frame
	// (non-shared isolation), so provider hooks only ever see real project paths
	// and never the shared-container fan-out. Kept aligned with the empty
	// ProjectsConfig passed to BuildContainerOverlay below.
	runner, err := credproxy.Start(ctx, dataDir, resolver.Resolve, agentlaunch.BuildProviderHooks(resolver.Resolve, platformconfig.ProjectsConfig{}), credproxy.Paths{
		RunDir:  agentlaunch.ContainerRunDir,
		BinPath: agentlaunch.ContainerBinaryPath,
		MCPSock: agentlaunch.ContainerMCPSockPath,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sandbox: start in-process credproxy: %w", err)
	}

	overlayFn := agentlaunch.BuildContainerOverlay(func(project string) platformconfig.SandboxConfig {
		return resolver.Resolve(project)
	}, platformconfig.ProjectsConfig{}, runner, dataDir, nil)

	mgr := sandboxdc.New(overlayFn)
	devLauncher := agentlaunch.NewDevcontainerLauncher(
		mgr,
		func(project string) platformconfig.SandboxConfig { return resolver.Resolve(project) },
		func(project string) *platformconfig.SandboxConfig { return resolver.ResolveProjectScope(project) },
		runner,
		dataDir,
		false, // orchestrator drives the agent over piped JSON-RPC stdio: no TTY
	)

	// credproxy is also bound to ctx, but Shutdown makes teardown deterministic:
	// it reaps provider-managed processes (ssh-agent) and waits for the server
	// goroutine to remove its socket before returning.
	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), credproxyShutdownTimeout)
		defer cancel()
		runner.Shutdown(shutdownCtx)
	}
	return devLauncher, cleanup, nil
}

// ensureProject warms up the dispatcher for a project path, supporting
// cold-start signalling if the dispatcher implements ColdStartAware.
func ensureProject(ctx context.Context, dispatcher agentlaunch.Dispatcher, projectPath string) error {
	cs, ok := dispatcher.(agentlaunch.ColdStartAware)
	if ok {
		cs.BeginColdStart()
	}
	if err := dispatcher.EnsureProject(ctx, projectPath); err != nil {
		return fmt.Errorf("orchestrator: ensure project: %w", err)
	}
	if ok {
		cs.EndColdStart()
	}
	return nil
}
