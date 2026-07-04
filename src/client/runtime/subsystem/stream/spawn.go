package stream

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-reactor/platform/lib/codex"
)

// spawnServer wraps the app-server using the dispatcher, resolves the host dial
// path from the launch's bind mounts, and spawns the process.
// The returned *strings.Builder accumulates the app-server's stderr; read it
// only after the process is reaped (res.Wait) so the copier goroutine is done.
func (b *Backend) spawnServer(ctx context.Context) (agentlaunch.SpawnResult, *strings.Builder, error) {
	argv := libcodex.AppServerListenArgs(b.serverBin, b.listenSock, b.serverArgs, b.sandboxed)

	plan := agentlaunch.LaunchPlan{
		Command:  strings.Join(argv, " "),
		Argv:     argv,
		Project:  b.project,
		StartDir: "",
	}

	errBuf := &strings.Builder{}
	var wrapped agentlaunch.WrappedLaunch
	var err error
	if b.dispatcher != nil {
		wrapped, err = b.dispatcher.Wrap(ctx, string(b.subsystemID), plan)
		if err != nil {
			return agentlaunch.SpawnResult{}, errBuf, fmt.Errorf("stream backend: dispatch wrap: %w", err)
		}
		b.setSpawnCleanup(wrapped.Cleanup)
	} else {
		wrapped = agentlaunch.WrappedLaunch{Argv: argv}
	}

	b.dialSock = resolveDialSock(b.listenSock, wrapped)
	b.mounts = toPathmapMounts(wrapped.Mounts)
	// Clear any stale socket before the app-server binds it (e.g. a crashed
	// predecessor left the file behind). Removing the host path also clears the
	// bind-mounted container path.
	_ = os.Remove(b.dialSock)

	res, err := agentlaunch.Spawn(b.ctx, wrapped, agentlaunch.SpawnOptions{
		InheritEnv: true,
		Stderr:     newPrefixWriter(errBuf, 8192),
	})
	if err != nil {
		b.cleanupSpawn(context.Background())
	}
	return res, errBuf, err
}

// trackProcessGroups records the app-server pgid so a future boot's
// PruneOrphans reaps it if this daemon dies without a graceful Stop.
// No-op when tracker is nil.
func (b *Backend) trackProcessGroups() {
	if b.spawnRes.PID != 0 {
		b.tracker.Track(b.spawnRes.PID)
	}
}
