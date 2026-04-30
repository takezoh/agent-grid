package winexec

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/lib/wsl"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

// Config holds path configuration for the winexec spec builder.
type Config struct {
	// RunBase is the parent of per-project run directories on the host.
	RunBase string
	// ContainerRunDir is the mount target inside the container (e.g. /opt/roost/run).
	ContainerRunDir string
	// ContainerBinPath is the in-container path to the roost binary that shims exec into.
	ContainerBinPath string
}

// SpecBuilder implements container.Provider for the WSL2 Windows exe broker.
// On non-WSL2 hosts, ContainerSpec always returns an empty Spec.
type SpecBuilder struct {
	ctx       context.Context
	cfg       Config
	configFor func(projectPath string) config.WinExecConfig

	mu      sync.Mutex
	brokers map[string]*broker // projectPath → broker
}

// NewSpecBuilder creates a SpecBuilder. ctx cancellation shuts down all brokers.
// configFor returns the WinExecConfig for a given project path.
func NewSpecBuilder(ctx context.Context, cfg Config, configFor func(string) config.WinExecConfig) *SpecBuilder {
	b := &SpecBuilder{
		ctx:       ctx,
		cfg:       cfg,
		configFor: configFor,
		brokers:   map[string]*broker{},
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "winexec" }

// Init creates RunBase. Idempotent.
func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.cfg.RunBase, 0o700)
}

// Routes returns nil; this provider uses sockets, not HTTP routes.
func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project broker, writes shims, and
// injects the PATH entry for the shims directory.
// Returns an empty Spec on non-WSL2 hosts or when no exes are configured.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string) (container.Spec, error) {
	if !wsl.IsWSL() {
		return container.Spec{}, nil
	}
	winCfg := b.configFor(projectPath)
	if len(winCfg.AllowedExes) == 0 {
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("winexec: mkdir run dir: %w", err)
	}

	if err := b.ensureBroker(projectPath, projRunDir, winCfg); err != nil {
		return container.Spec{}, err
	}

	if _, err := writeShims(projRunDir, b.cfg.ContainerBinPath, winCfg.AllowedExes); err != nil {
		return container.Spec{}, fmt.Errorf("winexec: write shims: %w", err)
	}

	// $PATH placeholder is expanded by ResolveContainerEnvPlaceholders against the image baseline.
	shimsDir := b.cfg.ContainerRunDir + "/" + ShimDirName
	return container.Spec{
		Env: map[string]string{"PATH": shimsDir + ":$PATH"},
	}, nil
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, winCfg config.WinExecConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.brokers[projectPath]; ok {
		existing.cfg.Store(&winCfg)
		return nil
	}

	sockPath := filepath.Join(projRunDir, "winexec.sock")
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("winexec: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:     b.ctx,
		sock:    sockPath,
		ln:      ln,
		project: projectPath,
		onStop: func() {
			b.mu.Lock()
			delete(b.brokers, projectPath)
			b.mu.Unlock()
		},
	}
	br.cfg.Store(&winCfg)
	b.brokers[projectPath] = br
	go br.serve()
	slog.Info("winexec: broker started", "project", projectPath, "sock", sockPath)
	return nil
}

func (b *SpecBuilder) watchShutdown(ctx context.Context) {
	<-ctx.Done()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, br := range b.brokers {
		br.ln.Close()
	}
}
