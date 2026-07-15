package agentlaunch

import (
	"fmt"
	"os"
	"os/exec"
)

// DriverBinResolver resolves a driver's executable to an absolute path. It is
// the single source of truth for "where does driver <name>'s host binary
// live" — the missing owner identified as the root of the codex-host launch
// failure class (frame-exec and bridge shim both bare-name LookPath into an
// under-populated daemon PATH, with no diagnostic and no config override).
//
// Resolution order per Resolve(name):
//
//  1. Overrides[name] — an absolute path configured under [drivers.<name>]
//     bin in settings.toml. Verified to exist on disk; a stale path returns
//     an actionable error rather than silently falling through to LookPath.
//  2. exec.LookPath(name) — the daemon's own PATH. Success returns the
//     absolute path; failure returns an actionable error naming the config
//     key the operator can set to work around a missing PATH entry.
//
// Empty Overrides is legal (Resolve degrades to pure PATH lookup). Every
// return path either yields a non-empty absolute path or a non-nil error —
// callers do not need to double-check for the empty-string sentinel.
type DriverBinResolver struct {
	// Overrides maps driver name → absolute path. Values must be absolute;
	// relative paths are treated as unset (fall through to LookPath) because
	// resolving them against the daemon's cwd would recreate the same
	// implicit-context defect this resolver exists to close.
	Overrides map[string]string
}

// Resolve returns the absolute path to the driver's binary or a non-nil
// error whose message names the config key an operator can set to fix a
// missing-binary scenario.
func (r *DriverBinResolver) Resolve(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("driverbin: empty driver name")
	}
	if r != nil {
		if override, ok := r.Overrides[name]; ok && isAbsolutePath(override) {
			if _, err := os.Stat(override); err != nil {
				return "", fmt.Errorf("driverbin: [drivers.%s] bin=%q does not exist: %w", name, override, err)
			}
			return override, nil
		}
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf(
			"driverbin: %q not found in daemon PATH; set [drivers.%s] bin=\"/abs/path/to/%s\" in settings.toml, or add its directory to the daemon's PATH (systemd unit Environment=PATH=…): %w",
			name, name, name, err,
		)
	}
	return resolved, nil
}

// isAbsolutePath is a stdlib-independent absolute-path check that avoids
// pulling filepath into this pure helper. os.Stat rejects relative paths
// against the wrong cwd anyway, but explicit rejection produces a better
// error class ("unset" rather than "does not exist").
func isAbsolutePath(p string) bool {
	return len(p) > 0 && p[0] == '/'
}
