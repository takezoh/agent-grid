package cli

import "github.com/takezoh/agent-reactor/client/event"

// resolveSocketPath returns the client daemon UDS path, preferring the
// ROOST_SOCKET env var when set. Inside a Docker sandbox container the env is
// set to the bind-mounted path (e.g. /tmp/arc.sock) so guest `arc` CLIs
// reach the same host daemon as local invocations.
func resolveSocketPath() (string, error) {
	return event.ResolveSocketPath()
}
