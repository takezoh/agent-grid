package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/takezoh/agent-grid/clients/cli/internal/attach"
	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/selector"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

// attach_command.go is the script `attach` runs. The order of its first two
// steps is a requirement rather than a preference: the terminal is checked
// before anything is subscribed, so an attach with a piped standard input
// leaves both the invoking terminal and the target session exactly as they
// were.

func attachSession(deps Deps, streams Streams, options attachOptions) error {
	terminal, err := attach.OpenTerminal(deps.Terminal)
	if err != nil {
		return &Failure{Class: ClassUsage, Reason: nonTerminalReason(err)}
	}
	defer func() { _ = terminal.Close() }()

	ctx := context.Background()
	snapshot, err := observeFleet(ctx, deps, options.config)
	if err != nil {
		return err
	}
	member, ref, err := resolveTarget(streams, snapshot, options.target)
	if err != nil {
		return err
	}
	return attachTo(ctx, deps, streams, terminal, member, ref, options.detachKey.value)
}

// attachTo dials, attaches, and presents the departure.
func attachTo(
	ctx context.Context,
	deps Deps,
	streams Streams,
	terminal tty.Terminal,
	member fleet.Member,
	ref gateway.SessionRef,
	detachByte byte,
) error {
	client, err := clientFor(deps, member.Server)
	if err != nil {
		return err
	}
	if err := client.Admit(ctx); err != nil {
		return transportFailure(err)
	}
	socket, instance, err := client.Dial(ctx)
	if err != nil {
		return transportFailure(err)
	}
	defer func() { _ = socket.Close() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, attachSignals()...)
	defer signal.Stop(signals)

	result, err := attach.Run(ctx, attach.Options{
		Ref:              ref,
		Socket:           socket,
		ClientInstanceID: instance,
		Terminal:         terminal,
		Out:              streams.Out,
		DetachByte:       detachByte,
		Signals:          signals,
	})
	if err != nil {
		return transportFailure(err)
	}
	// The terminal is already restored by the time Run returns, so the reason
	// below is printed to a shell that echoes again.
	return presentDeparture(streams, ref, result)
}

// presentDeparture writes why the attach ended and returns the failure, if any.
//
// The three endings the experience contract requires to be distinguishable get
// three different sentences, and a transport loss quotes the reason the plane
// published rather than a description this client invented.
func presentDeparture(streams Streams, ref gateway.SessionRef, result attach.Result) error {
	switch result.Departure {
	case attach.DepartedByDetach:
		_, _ = fmt.Fprintf(streams.Err, "detached from %s on %s; the session is still running\n",
			ref.SessionID(), ref.ServerName())
		return nil
	case attach.DepartedByInputEnd:
		_, _ = fmt.Fprintf(streams.Err, "terminal input ended; detached from %s on %s, "+
			"and the session is still running\n", ref.SessionID(), ref.ServerName())
		return nil
	case attach.DepartedBySignal:
		_, _ = fmt.Fprintf(streams.Err, "interrupted; detached from %s on %s, "+
			"and the session is still running\n", ref.SessionID(), ref.ServerName())
		return nil
	case attach.DepartedBySessionEnd:
		_, _ = fmt.Fprintf(streams.Err, "session %s on %s ended\n", ref.SessionID(), ref.ServerName())
		return nil
	case attach.DepartedByTransportLoss:
		failure := transportFailure(result.Failure())
		if result.Reason == gateway.ReasonUnattributed {
			// The connection ended without saying why. Saying so is the honest
			// report; naming one of the known causes would be a guess the
			// operator could not tell from an observation.
			failure.Reason = fmt.Sprintf(
				"lost the connection to %s; the server did not say why, "+
					"and the session may still be running", ref.ServerName())
		} else {
			failure.Reason = fmt.Sprintf("lost the connection to %s: %s",
				ref.ServerName(), result.Reason)
		}
		return failure
	}
	return failf(ClassInternal, "the attach ended in an unrecognised way")
}

// nonTerminalReason explains a refusal to attach without a terminal.
func nonTerminalReason(err error) string {
	if err == tty.ErrNotATerminal {
		return "attach needs a terminal on standard input; " +
			"nothing was subscribed and the terminal was not changed"
	}
	return fmt.Sprintf("cannot use the terminal: %s", err)
}

// resolveTarget turns the selector flags into one session, or into the failure
// that resolution concluded.
//
// Every branch below is a distinct exit code, because automation branches on
// them: "there is no such session" and "one of your servers is down" lead to
// completely different next actions and must never share a code.
func resolveTarget(
	streams Streams, snapshot fleet.Snapshot, target selectorFlags,
) (fleet.Member, gateway.SessionRef, error) {
	kind := selector.KindID
	value := target.id
	if target.prefix != "" {
		kind, value = selector.KindPrefix, target.prefix
	}
	outcome := selector.Resolve(snapshot, selector.Query{
		Kind: kind, Value: value, Server: target.server,
	})
	switch outcome.Kind {
	case selector.Resolved:
		return outcome.Member, outcome.Ref, nil
	case selector.NotFound:
		return fleet.Member{}, gateway.SessionRef{},
			failf(ClassNotFound, "no session matches %q", value)
	case selector.Unreachable:
		return fleet.Member{}, gateway.SessionRef{}, failf(ClassTransport,
			"no session matches %q, but at least one configured server could not be read, "+
				"so it may exist there", value)
	case selector.Ambiguous:
		writeCandidates(streams, outcome)
		return fleet.Member{}, gateway.SessionRef{},
			failf(ClassAmbiguous, "%q matches %d sessions; name one of them exactly",
				value, len(outcome.Candidates))
	case selector.QualifierUnknown:
		return fleet.Member{}, gateway.SessionRef{},
			failf(ClassUsage, "no configured server is named %q", target.server)
	case selector.QualifierUnusable:
		writeUnusableEntries(streams, outcome)
		return fleet.Member{}, gateway.SessionRef{}, failf(ClassUsage,
			"every configured entry named %q is unusable; fix the entries listed above",
			target.server)
	case selector.QualifierAmbiguous:
		writeAmbiguousServers(streams, outcome)
		return fleet.Member{}, gateway.SessionRef{},
			failf(ClassAmbiguous, "%q names %d usable servers; use the stable identifier instead",
				target.server, len(outcome.Entries))
	}
	return fleet.Member{}, gateway.SessionRef{},
		failf(ClassInternal, "resolution ended in an unrecognised way")
}

// writeCandidates lists an ambiguity's matches. Each line carries the values
// that uniquely specify the next invocation's target, so the operator can copy
// one rather than go back to the listing.
func writeCandidates(streams Streams, outcome selector.Outcome) {
	_, _ = fmt.Fprintf(streams.Err, "%q matches more than one session:\n", outcome.Value)
	for _, candidate := range outcome.Candidates {
		_, _ = fmt.Fprintf(streams.Err, "  --id %s --server %s   %s   %s\n",
			candidate.Ref.SessionID(), candidate.Ref.ServerID(),
			candidate.Ref.DisplayName(), candidate.Cwd)
	}
}

// writeUnusableEntries names each matching entry by its position, its display
// name as written, and the rule it violates — the three facts FR-031 requires
// and the three a loader that renamed or dropped the entry would have
// destroyed.
func writeUnusableEntries(streams Streams, outcome selector.Outcome) {
	_, _ = fmt.Fprintf(streams.Err, "%q names only unusable configured entries:\n", outcome.Server)
	for _, member := range outcome.Entries {
		_, _ = fmt.Fprintf(streams.Err, "  entry %d  %q  rule %s\n",
			member.Index, member.DisplayName, member.Observation.Rule)
	}
}

func writeAmbiguousServers(streams Streams, outcome selector.Outcome) {
	_, _ = fmt.Fprintf(streams.Err, "%q names more than one usable server:\n", outcome.Server)
	for _, member := range outcome.Entries {
		_, _ = fmt.Fprintf(streams.Err, "  entry %d  %q  --server %s\n",
			member.Index, member.DisplayName, member.ServerID)
	}
}
