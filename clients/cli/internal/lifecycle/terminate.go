package lifecycle

import (
	"context"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
)

// terminate.go holds the only site in this module that asks a server to end a
// session.
//
// Being the only one is the requirement, not a consequence of it. FR-001 lists
// detach, Ctrl-C, terminal close, stdin end, signals, an uncatchable kill and a
// WebSocket close, and forbids a termination on every one of them — and the way
// that stays true is that no code on any of those paths has the call available
// to make. TestTerminateRequestHasExactlyOneEmissionSite fails when a second
// site appears anywhere in the module.

// Terminate asks the server to end one session and reports whether it said it
// did.
//
// Success means an acknowledgement and nothing more. This function deliberately
// does not read the session back afterwards and the caller must not claim a
// resulting lifecycle state: the server acknowledges that it applied the
// request, and describing what the session "now is" would be asserting an
// observation nobody made.
func Terminate(ctx context.Context, client *gateway.Client, sessionID string) error {
	return client.Delete(ctx, "/api/sessions/"+sessionID)
}
