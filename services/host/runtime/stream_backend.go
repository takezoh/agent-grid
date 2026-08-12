package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-grid/services/host/runtime/subsystem/stream"
	"github.com/takezoh/agent-grid/services/host/state"
)

// resolveStreamListenPath returns the UDS path the per-session app-server
// binds. The bind path is host-absolute under the daemon's data dir and
// doubles as the daemon dial path. Each session gets a unique sock file so
// concurrent app-server processes do not collide.
func (r *Runtime) resolveStreamListenPath(sessionID state.SessionID, project string) (string, error) {
	_ = project // reserved for per-project sock naming if a future launch matrix needs it
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	sockName := cstream.SockPrefix + string(sessionID) + cstream.SockSuffix
	// App-server bind path and daemon dial path are identical. All session
	// sockets share one directory watched by the host bridge.
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
