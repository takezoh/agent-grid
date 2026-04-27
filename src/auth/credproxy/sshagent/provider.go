// Package sshagent implements a credproxy.Provider that injects an ephemeral
// SSH agent into containers. roost spawns an ssh-agent, loads only the listed
// keys, and forwards that socket. The container can sign but never sees private keys.
package sshagent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

const (
	// containerSocketPath is the in-container path to the SSH agent socket.
	// Exposed via the /opt/roost/run directory bind mount.
	containerSocketPath = "/opt/roost/run/agent.sock"
	agentReadyTimeout   = 5 * time.Second
	agentReadyPoll      = 50 * time.Millisecond
)

type ephemeralAgent struct {
	sockPath string
	cmd      *exec.Cmd
}

// SpecBuilder implements credproxy.Provider for SSH agent forwarding.
type SpecBuilder struct {
	ctx     context.Context
	runBase string // parent of per-project run dirs (e.g. <dataDir>/run)

	mu     sync.Mutex
	agents map[string]*ephemeralAgent // projectPath → agent
}

// NewSpecBuilder creates a SpecBuilder. ctx is used to kill ephemeral agents on shutdown.
// runBase is the parent of per-project run dirs bound into containers at /opt/roost/run.
func NewSpecBuilder(ctx context.Context, runBase string) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		runBase: runBase,
		agents:  map[string]*ephemeralAgent{},
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "sshagent" }

// Init creates runBase.
func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.runBase, 0o700)
}

// Routes returns nil; this provider uses sockets, not HTTP routes.
func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec implements credproxy.Provider.
// Spawns an ephemeral agent per project (cached for lifetime of roost) when Keys is non-empty.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string, sb config.SandboxConfig) (credproxy.Spec, error) {
	if len(sb.Proxy.SSHAgent.Keys) > 0 {
		return b.keysSpec(projectPath, sb.Proxy.SSHAgent.Keys)
	}
	return credproxy.Spec{}, nil
}

func (b *SpecBuilder) keysSpec(projectPath string, keys []string) (credproxy.Spec, error) {
	b.mu.Lock()
	_, ok := b.agents[projectPath]
	b.mu.Unlock()

	if !ok {
		a, err := b.spawnAgent(projectPath, keys)
		if err != nil {
			return credproxy.Spec{}, fmt.Errorf("sshagent: spawn agent: %w", err)
		}
		b.mu.Lock()
		b.agents[projectPath] = a
		b.mu.Unlock()
	}

	// No per-socket mount: the per-project run dir is bound at /opt/roost/run by
	// injectOverlay, so agent.sock inside it is accessible at containerSocketPath.
	return credproxy.Spec{
		Env: map[string]string{"SSH_AUTH_SOCK": containerSocketPath},
	}, nil
}

func (b *SpecBuilder) spawnAgent(projectPath string, keys []string) (*ephemeralAgent, error) {
	projectRunDir := filepath.Join(b.runBase, projectRunHash(projectPath))
	if err := os.MkdirAll(projectRunDir, 0o700); err != nil {
		return nil, fmt.Errorf("sshagent: mkdir run dir: %w", err)
	}
	sockPath := filepath.Join(projectRunDir, "agent.sock")
	_ = os.Remove(sockPath) // clean up stale socket from prior run

	// -D keeps ssh-agent in foreground so cmd.Process tracks the real agent PID.
	cmd := exec.CommandContext(b.ctx, "ssh-agent", "-D", "-a", sockPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ssh-agent: %w", err)
	}

	if err := waitForSocket(sockPath, agentReadyTimeout, agentReadyPoll); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("ssh-agent socket not ready: %w", err)
	}

	expandedKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		expandedKeys = append(expandedKeys, config.ExpandPath(k))
	}
	addKeys(sockPath, expandedKeys)

	return &ephemeralAgent{sockPath: sockPath, cmd: cmd}, nil
}

// waitForSocket polls until the Unix socket at path accepts connections or timeout elapses.
func waitForSocket(path string, timeout, poll time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("unix", path, poll)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(poll)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

// addKeys runs ssh-add for each key. Failures are logged and skipped.
func addKeys(sockPath string, keys []string) {
	for _, k := range keys {
		if _, err := os.Stat(k); err != nil {
			slog.Warn("sshagent: key not found, skipping", "path", k)
			continue
		}
		cmd := exec.Command("ssh-add", k)
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sockPath)
		cmd.Stdin = nil // non-interactive; passphrase-protected keys will fail
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Warn("sshagent: ssh-add failed (passphrase-protected?), skipping", "path", k, "out", string(out))
		}
	}
}

func (b *SpecBuilder) watchShutdown(ctx context.Context) {
	<-ctx.Done()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, a := range b.agents {
		if a.cmd.Process != nil {
			_ = a.cmd.Process.Kill()
		}
		_ = os.Remove(a.sockPath)
	}
}

// projectRunHash produces the per-project run dir name (6 bytes → 12 hex chars),
// matching the convention used by runtime.ProjectRunDir.
func projectRunHash(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:6])
}
