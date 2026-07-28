package sandbox

import "fmt"

// RepositoryEnvironmentKey is the durable identity for a repository-scoped
// devcontainer: execution server + GitHub repository ID + config generation.
// It is intentionally not a clone path — session clones share one environment.
type RepositoryEnvironmentKey struct {
	ServerID     string
	RepositoryID string
	ConfigGen    string
}

// Validate reports whether the key is fully qualified.
func (k RepositoryEnvironmentKey) Validate() error {
	if k.ServerID == "" || k.RepositoryID == "" || k.ConfigGen == "" {
		return fmt.Errorf("sandbox: repository environment key requires server, repository, and config generation")
	}
	return nil
}

// String returns a stable map key for caches and durable records.
func (k RepositoryEnvironmentKey) String() string {
	return k.ServerID + "|" + k.RepositoryID + "|" + k.ConfigGen
}

// RepositoryIsolationPlan returns the only isolation plan permitted for
// repository environments: per-project (never shared, never per-session clone).
func RepositoryIsolationPlan() IsolationPlan {
	return IsolationPlan{Kind: IsolationProject}
}

// RejectSharedRepositoryIsolation fails closed when a shared plan is proposed
// for a repository environment (contract-repository-devcontainer-scope).
func RejectSharedRepositoryIsolation(p IsolationPlan) error {
	if p.IsShared() {
		return fmt.Errorf("sandbox: shared isolation is forbidden for repository environments")
	}
	return nil
}
