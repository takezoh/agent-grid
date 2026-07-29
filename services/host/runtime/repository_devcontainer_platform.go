package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-grid/services/platform/sandbox/devcontainer"
)

// DevcontainerEnsureFunc is the inject seam for production Manager.EnsureInstance
// (and fakedocker-backed contract tests). Returns the running container ID.
type DevcontainerEnsureFunc func(ctx context.Context, projectPath string, opts sandbox.StartOptions) (containerID string, err error)

// PlatformRepositoryDevcontainer adapts platform/sandbox/devcontainer into the
// host runtime RepositoryDevcontainer seam. Repository identity (not clone path)
// keys the environment; sessions root is the only mount scope.
type PlatformRepositoryDevcontainer struct {
	EnsureFn DevcontainerEnsureFunc
}

// NewPlatformRepositoryDevcontainer wraps a live *devcontainer.Manager.
// mgr may be nil — Ensure then returns environment_authority_unavailable.
func NewPlatformRepositoryDevcontainer(mgr *sandboxdc.Manager) *PlatformRepositoryDevcontainer {
	if mgr == nil {
		return &PlatformRepositoryDevcontainer{}
	}
	return &PlatformRepositoryDevcontainer{
		EnsureFn: func(ctx context.Context, projectPath string, opts sandbox.StartOptions) (string, error) {
			inst, err := mgr.EnsureInstance(ctx, projectPath, "", opts)
			if err != nil {
				return "", err
			}
			if inst == nil || inst.Internal == nil {
				return "", fmt.Errorf("devcontainer: empty instance")
			}
			id := inst.Internal.ContainerID()
			if id == "" {
				return "", fmt.Errorf("devcontainer: empty container id")
			}
			return id, nil
		},
	}
}

// UnavailableRepositoryDevcontainer is the composition-root stand-in when no
// sandbox manager is available (direct mode). Always typed unavailable — never nil.
type UnavailableRepositoryDevcontainer struct{}

func (UnavailableRepositoryDevcontainer) Ensure(_ context.Context, _ RepositoryEnvironmentRequest) (RepositoryEnvironmentResult, error) {
	return RepositoryEnvironmentResult{Code: EnvCodeUnavailable, Diagnostic: "repository environment authority unavailable"}, nil
}

// Ensure forces per-project isolation, reuses repository-stable project identity,
// and reports mount scope as the sessions root with frame CWD = clone path.
func (p *PlatformRepositoryDevcontainer) Ensure(ctx context.Context, req RepositoryEnvironmentRequest) (RepositoryEnvironmentResult, error) {
	if p == nil || p.EnsureFn == nil {
		return RepositoryEnvironmentResult{Code: EnvCodeUnavailable, Diagnostic: "repository environment authority unavailable"}, nil
	}
	if req.SessionsRoot == "" {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: "sessions root required"}, nil
	}
	if req.ServerID == "" || !req.Repository.Valid() {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: "fully-qualified repository identity required"}, nil
	}
	plan := sandbox.RepositoryIsolationPlan()
	if err := sandbox.RejectSharedRepositoryIsolation(plan); err != nil {
		return RepositoryEnvironmentResult{Code: EnvCodeIsolationDenied, Diagnostic: err.Error()}, nil
	}
	// Stable project path for container key: never the per-session clone path.
	gen := req.ConfigGen
	if gen == "" {
		gen = "default"
	}
	envProject := RepositoryEnvironmentProjectPath(req.SessionsRoot, req.ServerID, req.Repository.ID, gen)
	if err := os.MkdirAll(envProject, 0o755); err != nil {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: err.Error()}, err
	}
	// Prefer clone-local .devcontainer when present; else Manager falls back to user base.
	if req.ClonePath != "" {
		candidate := filepath.Join(req.ClonePath, ".devcontainer")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			plan.DevcontainerDir = candidate
		}
	}
	cid, err := p.EnsureFn(ctx, envProject, sandbox.StartOptions{Isolation: plan})
	if err != nil {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: err.Error()}, err
	}
	envID := state.RepositoryEnvironmentID(fmt.Sprintf("env-%s-%s-%s", req.ServerID, req.Repository.ID, gen))
	return RepositoryEnvironmentResult{
		Code:          EnvCodeOK,
		EnvironmentID: envID,
		ContainerID:   cid,
		Reused:        false, // durable reuse is RepositoryEnvironmentManager.Store.FindReady
		MountScope:    req.SessionsRoot,
		FrameCWD:      req.ClonePath,
	}, nil
}

// RepositoryEnvironmentProjectPath is the durable host path that identities a
// repository environment for the sandbox manager (not a session clone).
func RepositoryEnvironmentProjectPath(sessionsRoot string, server state.ExecutionServerID, repoID state.GitHubRepositoryID, configGen string) string {
	return filepath.Join(sessionsRoot, "repository-env", string(server), string(repoID), configGen)
}
