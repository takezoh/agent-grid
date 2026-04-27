package credproxy

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// Spec is the per-container contribution from a single credential provider.
// Env keys are roost-internal names; Mounts are docker-style "host:guest[:mode]" specs.
type Spec struct {
	Env    map[string]string
	Mounts []string
}

// ProjectRunHash returns the per-project run directory name (6 bytes → 12 hex chars).
// Matches the convention used by runtime.ProjectRunDir.
func ProjectRunHash(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:6])
}

// Provider is implemented by each credential backend (awssso, gcloudcli, sshagent, github, ...).
// The lifecycle is: Init once at startup, Routes to register HTTP handlers, then ContainerSpec
// per container launch.
type Provider interface {
	// Name returns a short identifier used in logs.
	Name() string
	// Init creates any host-side state (directories, helper scripts) the provider needs.
	// Called once by StartCredProxy before any container is launched. Must be idempotent.
	Init() error
	// Routes returns HTTP route registrations for this provider.
	// Providers that need no HTTP route return nil.
	Routes() []credproxylib.Route
	// ContainerSpec returns this provider's Env/Mounts contribution for projectPath,
	// or a zero Spec when the provider is not configured for that project.
	ContainerSpec(ctx context.Context, projectPath string, sb config.SandboxConfig) (Spec, error)
}
