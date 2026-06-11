package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-reactor/client/runtime/subsystem/stream"
	"github.com/takezoh/agent-reactor/client/state"
)

// resolveStreamListenPath returns the UDS path the per-session app-server binds.
// In host mode it is also the daemon's dial path; in container mode it is a
// container-absolute path under ContainerRunDir and the stream backend resolves
// the host dial path from the launch's bind mounts (the single source of truth
// for the host↔container mapping — this never recomputes the host run dir).
// Each session gets a unique sock file so concurrent app-server processes do not
// collide. project is passed by the caller (from the spawn effect) so this runs
// safely off r.state from a goroutine.
func (r *Runtime) resolveStreamListenPath(sessionID state.SessionID, project string) (string, error) {
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	sockName := cstream.SockPrefix + string(sessionID) + cstream.SockSuffix
	if launcher(r.cfg).IsContainer(project) {
		// The app-server binds this fixed container path; the in-container routing
		// sockbridge finds it by session ID and the backend maps it back to a host
		// path via the launch mounts.
		return ContainerRunDir + "/" + sockName, nil
	}
	// Host mode: app-server bind path and daemon dial path are identical. All
	// host-mode session sockets share one directory watched by the host bridge.
	runDir, err := ensureStreamRunDir(filepath.Join(dataDir, "run", cstream.RunDirName))
	if err != nil {
		return "", fmt.Errorf("stream backend: run dir: %w", err)
	}
	return filepath.Join(runDir, sockName), nil
}

// ensureStreamRunDir creates the stream run directory if it does not exist.
func ensureStreamRunDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
