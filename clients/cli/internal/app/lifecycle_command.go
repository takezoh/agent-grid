package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/takezoh/agent-grid/clients/cli/internal/attach"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/lifecycle"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

// lifecycle_command.go is the script `new` and `kill` run.

func createSession(deps Deps, streams Streams, options newOptions) error {
	ctx := context.Background()
	snapshot, err := observeFleet(ctx, deps, options.config)
	if err != nil {
		return err
	}
	member, err := lifecycle.SelectCreateTarget(snapshot, options.server)
	if err != nil {
		if errors.Is(err, lifecycle.ErrTargetRequired) {
			return failf(ClassUsage,
				"--server is required; session creation never infers a target server")
		}
		return failf(ClassUsage, "%s", err)
	}
	client, err := clientFor(deps, member.Server)
	if err != nil {
		return err
	}
	if err := client.Admit(ctx); err != nil {
		return transportFailure(err)
	}

	// The terminal is opened before the creation when an attach will follow, so
	// a piped invocation fails without having created anything. Under
	// --no-attach there is nothing to open.
	var terminal tty.Terminal
	cols, rows := 0, 0
	if !options.noAttach {
		opened, openErr := attach.OpenTerminal(deps.Terminal)
		if openErr != nil {
			return &Failure{Class: ClassUsage, Reason: nonTerminalReason(openErr)}
		}
		terminal = opened
		defer func() { _ = terminal.Close() }()
		cols, rows, _ = terminal.Size()
	}

	result := lifecycle.Run(ctx, client, lifecycle.Params{
		Project:    options.cwd,
		Agent:      options.agent,
		Parameters: options.parameters.values,
		Cols:       cols,
		Rows:       rows,
		Worktree:   options.worktree,
		NoAttach:   options.noAttach,
		Attach: func(created lifecycle.Created) error {
			// The attach follows the identifier the creation returned, not a
			// re-resolution of the selector: re-resolving could land on a
			// different session that happens to match, which is the one thing
			// a create-then-attach must never do.
			ref := gateway.NewSessionRef(member.Server, gateway.SessionInfo{
				ID: created.ID, Project: created.Project, Command: created.Command,
			})
			return attachTo(ctx, deps, streams, terminal, member, ref, options.detachKey.value)
		},
	})
	return presentCreation(streams, member.DisplayName, options, result)
}

// presentCreation turns the four creation outcomes into four observations.
//
// The one that is easy to get wrong is the middle: a session was created and
// the attach then failed. Reporting that as a creation failure hides a live
// session the operator is now paying for and does not know about, so it exits
// non-zero *and* prints the identifier.
func presentCreation(
	streams Streams, server string, options newOptions, result lifecycle.Result,
) error {
	if result.Created == nil {
		if errors.Is(result.CreateErr, lifecycle.ErrUnknownAgent) {
			return failf(ClassUsage, "%s", result.CreateErr)
		}
		return transportFailure(result.CreateErr)
	}
	if result.AttachErr != nil {
		_, _ = fmt.Fprintf(streams.Out, "%s\n", result.Created.ID)
		_, _ = fmt.Fprintf(streams.Err,
			"created session %s on %s, but the attach failed; the session is running\n",
			result.Created.ID, server)
		attachFailure := transportFailure(result.AttachErr)
		var failure *Failure
		if asFailure(result.AttachErr, &failure) {
			attachFailure = failure
		}
		return attachFailure
	}
	if options.noAttach {
		// Nothing was attempted, so nothing failed. The identifier is the whole
		// point of the invocation.
		_, _ = fmt.Fprintf(streams.Out, "%s\n", result.Created.ID)
		_, _ = fmt.Fprintf(streams.Err, "created session %s on %s\n", result.Created.ID, server)
	}
	return nil
}

func terminateSession(deps Deps, streams Streams, options killOptions) error {
	ctx := context.Background()
	snapshot, err := observeFleet(ctx, deps, options.config)
	if err != nil {
		return err
	}
	member, ref, err := resolveTarget(streams, snapshot, options.target)
	if err != nil {
		return err
	}
	client, err := clientFor(deps, member.Server)
	if err != nil {
		return err
	}
	if err := client.Admit(ctx); err != nil {
		return transportFailure(err)
	}
	if err := lifecycle.Terminate(ctx, client, ref.SessionID()); err != nil {
		return transportFailure(err)
	}
	// The report claims exactly what was observed: the server acknowledged the
	// request. It does not say the session "is now stopped", because that is a
	// lifecycle state nobody read back.
	_, _ = fmt.Fprintf(streams.Err, "%s on %s: the server accepted the termination\n",
		ref.SessionID(), ref.ServerName())
	return nil
}
