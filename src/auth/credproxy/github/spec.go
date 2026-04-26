package github

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// SpecBuilder implements credproxy.Provider for GitHub HTTPS authentication.
// It registers an HTTP route that shells out to "gh auth token" on the host,
// and mounts a git credential helper script and gitconfig snippet into containers.
type SpecBuilder struct {
	proxyAddr string
	token     string
	gitDir    string

	mu        sync.Mutex
	cachedPAT string
	patExpiry time.Time
}

// NewSpecBuilder creates a SpecBuilder. proxyAddr is "127.0.0.1:<port>"; gitDir
// is the directory where the helper script and gitconfig are written.
func NewSpecBuilder(proxyAddr, token, gitDir string) *SpecBuilder {
	return &SpecBuilder{proxyAddr: proxyAddr, token: token, gitDir: gitDir}
}

// ghPAT returns the current GitHub PAT from the host gh CLI, cached for tokenCacheTTL.
func (b *SpecBuilder) ghPAT(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cachedPAT != "" && time.Now().Before(b.patExpiry) {
		return b.cachedPAT, nil
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token: %w", err)
	}
	pat := strings.TrimSpace(string(out))
	if pat == "" {
		return "", fmt.Errorf("gh auth token returned empty output")
	}
	b.cachedPAT = pat
	b.patExpiry = time.Now().Add(tokenCacheTTL)
	return pat, nil
}

func (b *SpecBuilder) Name() string { return "github" }

// Init creates gitDir and writes the credential helper script and gitconfig snippet.
func (b *SpecBuilder) Init() error {
	if err := os.MkdirAll(b.gitDir, 0o755); err != nil {
		return fmt.Errorf("github: mkdir: %w", err)
	}
	if err := writeHelperScript(filepath.Join(b.gitDir, "git-credential-roost")); err != nil {
		return fmt.Errorf("github: write helper script: %w", err)
	}
	if err := writeGitconfig(filepath.Join(b.gitDir, "gitconfig")); err != nil {
		return fmt.Errorf("github: write gitconfig: %w", err)
	}
	return nil
}

// Routes returns the HTTP route that serves GitHub credentials to containers.
func (b *SpecBuilder) Routes() []credproxylib.Route {
	return []credproxylib.Route{
		{Path: RoutePath, Provider: newHTTPProvider()},
	}
}

// ContainerSpec implements credproxy.Provider.
// Returns zero Spec when proxy.github.enabled is false or gh is not installed on the host.
func (b *SpecBuilder) ContainerSpec(ctx context.Context, _ string, sb config.SandboxConfig) (credproxy.Spec, error) {
	if !sb.Proxy.GitHub.Enabled {
		return credproxy.Spec{}, nil
	}

	if _, err := exec.LookPath("gh"); err != nil {
		slog.Warn("github: proxy.github.enabled=true but gh not found on host PATH")
		return credproxy.Spec{}, nil
	}

	pat, err := b.ghPAT(ctx)
	if err != nil {
		slog.Warn("github: failed to get PAT from gh", "err", err)
		return credproxy.Spec{}, nil
	}

	helperPath := filepath.Join(b.gitDir, "git-credential-roost")
	gitconfigPath := filepath.Join(b.gitDir, "gitconfig")

	env := containerEnv(b.proxyAddr, b.token)
	env["GH_TOKEN"] = pat

	return credproxy.Spec{
		Env: env,
		Mounts: []string{
			helperPath + ":" + containerHelperPath + ":ro",
			gitconfigPath + ":" + containerGitconfigPath + ":ro",
		},
	}, nil
}
