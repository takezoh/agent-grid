package app

import (
	"context"
	"errors"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/view"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// session_commands.go holds the four operational verbs. Each one is a short
// script over the packages that do the work: read the configured fleet, observe
// it, resolve the target, then act. Nothing here decides an exit code — it
// returns a Failure and exit.go owns the number.

// observeFleet reads the configuration and probes every usable entry.
//
// The configuration read is a file-level stop or nothing: the loader admits a
// broken entry as configuration-invalid and only refuses the whole document
// when no entry can be named at all. That case is a usage failure — the
// operator has a file to fix — and it is the only one where the CLI declines to
// show a fleet.
func observeFleet(
	ctx context.Context, deps Deps, config configFlags,
) (fleet.Snapshot, error) {
	directory := config.directory
	if directory == "" {
		resolved, err := deps.DefaultConfigDir()
		if err != nil {
			return fleet.Snapshot{}, failf(ClassUsage,
				"cannot locate the client configuration directory: %s", err)
		}
		directory = resolved
	}
	configured, err := deps.LoadConfig(directory)
	if err != nil {
		return fleet.Snapshot{}, failf(ClassUsage, "%s", err)
	}
	return fleet.Observe(ctx, configured, fleet.NewProber(deps.Doer, deps.Dialer)), nil
}

func listSessions(deps Deps, streams Streams, options listOptions) error {
	snapshot, err := observeFleet(context.Background(), deps, options.config)
	if err != nil {
		return err
	}
	if options.json {
		if err := view.WriteJSON(streams.Out, snapshot); err != nil {
			return failf(ClassInternal, "cannot write the listing: %s", err)
		}
		return nil
	}
	view.WriteTable(streams.Out, snapshot)
	return nil
}

// transportFailure turns a gateway failure into the class the exit-code
// assignment expects.
//
// The classification is read off the reason the plane published, never guessed
// from the shape of the error. A departure that published nothing is a
// transport failure of unknown cause, which is what it is — not a not-found and
// not a clean end.
func transportFailure(err error) *Failure {
	var transport *gateway.TransportError
	if !errors.As(err, &transport) {
		return &Failure{Class: ClassTransport, Reason: err.Error()}
	}
	class := ClassTransport
	switch {
	case errors.Is(transport, gateway.ErrIncompatibleProtocol),
		errors.Is(transport, gateway.ErrUnauthenticated):
		class = ClassAuth
	case errors.Is(transport, gateway.ErrNotFound):
		class = ClassNotFound
	case errors.Is(transport, gateway.ErrInvalidRequest):
		class = ClassUsage
	case errors.Is(transport, gateway.ErrInternal):
		class = ClassInternal
	}
	return &Failure{Class: class, Reason: transport.Error()}
}

// clientFor builds the transport for one observed member.
func clientFor(deps Deps, server clientconfig.Server) (*gateway.Client, error) {
	client, err := gateway.New(server, deps.Doer, deps.Dialer)
	if err != nil {
		return nil, transportFailure(err)
	}
	return client, nil
}
