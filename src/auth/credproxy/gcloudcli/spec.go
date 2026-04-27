package gcloudcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// SpecBuilder implements credproxy.Provider for the gcloud CLI.
// It manages per-account token refresh goroutines and writes synthetic
// CLOUDSDK_CONFIG directories so containers receive only short-lived access tokens.
type SpecBuilder struct {
	rootCtx context.Context
	gcpDir  string // base directory for per-account token storage
	runBase string // base for per-project run dirs (e.g. <dataDir>/run)

	mu         sync.Mutex
	refreshers map[string]*Refresher // keyed by principalKey(account, sa)
}

// NewSpecBuilder creates a SpecBuilder.
// gcpDir stores per-account token files refreshed by background goroutines.
// runBase is the parent of per-project run dirs bound into containers at /opt/roost/run.
// rootCtx controls the lifetime of token refresh goroutines.
func NewSpecBuilder(rootCtx context.Context, gcpDir, runBase string) *SpecBuilder {
	return &SpecBuilder{
		rootCtx:    rootCtx,
		gcpDir:     gcpDir,
		runBase:    runBase,
		refreshers: make(map[string]*Refresher),
	}
}

func (b *SpecBuilder) Name() string { return "gcloudcli" }

// Init creates gcpDir and runBase.
func (b *SpecBuilder) Init() error {
	if err := os.MkdirAll(b.gcpDir, 0o755); err != nil {
		return fmt.Errorf("gcloudcli: mkdir: %w", err)
	}
	if err := os.MkdirAll(b.runBase, 0o700); err != nil {
		return fmt.Errorf("gcloudcli: mkdir runBase: %w", err)
	}
	return nil
}

// Routes returns nil; gcloudcli uses bind-mounted files, not an HTTP route.
func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec implements credproxy.Provider.
// Returns zero Spec when service_account or projects are absent.
// Returns an error when account-only is set without service_account (full-scope tokens unsupported).
// Files are written into the per-project run dir so the single dir bind at
// /opt/roost/run covers them without additional per-file mounts.
func (b *SpecBuilder) ContainerSpec(ctx context.Context, projectPath string, sb config.SandboxConfig) (credproxy.Spec, error) {
	account := sb.Proxy.GCP.Account
	sa := sb.Proxy.GCP.ServiceAccount
	projects := sb.Proxy.GCP.Projects

	if sa == "" && account == "" && len(projects) == 0 {
		return credproxy.Spec{}, nil
	}
	if sa == "" || len(projects) == 0 {
		return credproxy.Spec{}, fmt.Errorf("gcloudcli: service_account and projects must both be set; full-scope account tokens are not supported")
	}

	tokenSrc, err := b.ensureRefresher(ctx, account, sa)
	if err != nil {
		return credproxy.Spec{}, err
	}

	projectRunDir := filepath.Join(b.runBase, projectRunHash(projectPath))
	if err := os.MkdirAll(projectRunDir, 0o700); err != nil {
		return credproxy.Spec{}, fmt.Errorf("gcloudcli: mkdir run dir: %w", err)
	}

	if err := linkToken(tokenSrc, filepath.Join(projectRunDir, "gcloud-token")); err != nil {
		slog.Warn("gcloudcli: token link failed, skipping", "err", err)
	}

	configDir := filepath.Join(projectRunDir, "gcloud-config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return credproxy.Spec{}, fmt.Errorf("gcloudcli: mkdir config dir: %w", err)
	}
	if err := WriteConfigDir(configDir, sa, projects); err != nil {
		return credproxy.Spec{}, fmt.Errorf("gcloudcli: write config dir: %w", err)
	}

	// Do not inject GOOGLE_OAUTH_ACCESS_TOKEN: that env var is static and becomes stale after
	// 1h. Instead, gcloud reads containerTokenPath via auth/access_token_file on every
	// invocation (gcloud 394+). The host Refresher writes to the same inode via os.WriteFile,
	// so the bind-mounted file always reflects the latest token without a container restart.
	return credproxy.Spec{Env: ContainerEnv("")}, nil
}

// ensureRefresher starts a token refresh goroutine for (account, sa) if not already running.
// Returns the host path of the token file (under gcpDir, NOT the run dir).
func (b *SpecBuilder) ensureRefresher(ctx context.Context, account, sa string) (string, error) {
	key := principalKey(account, sa)
	principalDir := filepath.Join(b.gcpDir, principalHash(key))
	if err := os.MkdirAll(principalDir, 0o755); err != nil {
		return "", fmt.Errorf("gcloudcli: mkdir principal dir: %w", err)
	}
	tokenPath := filepath.Join(principalDir, "access-token")

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, running := b.refreshers[key]; !running {
		ref := NewRefresher(account, sa, tokenPath)
		if err := ref.Prime(ctx); err != nil {
			slog.Warn("gcloudcli: initial token fetch failed", "account", account, "sa", sa, "err", err)
		}
		b.refreshers[key] = ref
		go ref.Run(b.rootCtx)
	}

	return tokenPath, nil
}

// linkToken hardlinks tokenSrc into dst so the container sees the same file that
// Refresher writes to (os.WriteFile preserves the inode, so hardlinks track updates).
// Silently skips if tokenSrc does not exist yet (gcloud not configured).
func linkToken(tokenSrc, dst string) error {
	if _, err := os.Stat(tokenSrc); err != nil {
		return nil // token not yet written (gcloud unavailable)
	}
	_ = os.Remove(dst)
	return os.Link(tokenSrc, dst)
}

// principalKey combines the host gcloud account and service account into a unique key
// so that different (account, SA) pairs get isolated token files and refreshers.
func principalKey(account, sa string) string { return account + "|" + sa }

func principalHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:4])
}

// projectRunHash produces the per-project run dir name (6 bytes → 12 hex chars),
// matching the convention used by runtime.ProjectRunDir.
func projectRunHash(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:6])
}
