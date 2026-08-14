// Package app is the CLI entry point: it turns an argument vector into a
// failure class and writes everything the user sees. It owns verb dispatch and
// the exit-code assignment; every other package returns values to it.
package app

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Streams is the process's three standard streams plus one observation about
// stdout.
//
// OutIsTerminal exists to be ignored. FR-025 requires identical output content
// and identical exit codes whether or not stdout is a terminal, so nothing in
// this program may branch on it — no colouring, no width-dependent truncation,
// no confirmation prompt. It is carried anyway so that
// TestInteractiveAndPipedInvocationsAgree can vary it and assert that nothing
// changes, which a field that does not exist cannot do.
type Streams struct {
	In            io.Reader
	Out           io.Writer
	Err           io.Writer
	OutIsTerminal bool
}

// Deps are the external boundaries this program reaches through. Every field
// has a production default, so a caller that wants the real thing passes the
// zero value; a test substitutes only the boundary it cares about.
//
// The set is deliberately small and is exactly the two external dependencies
// the harness registry declares — the gateway and the local terminal — plus the
// configuration read, which is a file the CLI is forbidden to write.
type Deps struct {
	Doer             gateway.Doer
	Dialer           gateway.Dialer
	Terminal         tty.Opener
	LoadConfig       func(directory string) (clientconfig.Snapshot, error)
	DefaultConfigDir func() (string, error)
}

func (deps Deps) withDefaults() Deps {
	if deps.LoadConfig == nil {
		deps.LoadConfig = clientconfig.Load
	}
	if deps.DefaultConfigDir == nil {
		deps.DefaultConfigDir = clientconfig.DefaultDirectory
	}
	if deps.Terminal == nil {
		deps.Terminal = tty.Open
	}
	return deps
}

// command is one verb. Handlers receive the arguments after the verb.
type command struct {
	name    string
	summary string
	usage   string
	run     func(deps Deps, streams Streams, args []string) error
}

// Run dispatches argv (already stripped of the program name) against the real
// boundaries and returns the class the process should exit with.
func Run(streams Streams, argv []string) FailureClass {
	return RunWith(Deps{}, streams, argv)
}

// RunWith is Run with the external boundaries supplied.
//
// The first argument is always a verb (DP-ENTRY-DISPATCH-STYLE). A session
// identifier passed positionally is therefore an unknown verb and a usage
// error — never a target. That is the whole point: a selector that happens to
// spell a subcommand name cannot change what the invocation does.
func RunWith(deps Deps, streams Streams, argv []string) FailureClass {
	err := dispatch(deps.withDefaults(), streams, argv)
	if err == nil {
		return ClassSuccess
	}
	var failure *Failure
	if !asFailure(err, &failure) {
		// A command returned a bare error. That is this program's fault, not
		// the user's, and it is reported as one rather than being guessed into
		// a user-facing class.
		failure = &Failure{Class: ClassInternal, Reason: err.Error()}
	}
	_, _ = fmt.Fprintf(streams.Err, "agent-grid: %s\n", failure.Reason)
	return failure.Class
}

func asFailure(err error, target **Failure) bool {
	for current := err; current != nil; {
		if failure, ok := current.(*Failure); ok {
			*target = failure
			return true
		}
		unwrapper, ok := current.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		current = unwrapper.Unwrap()
	}
	return false
}

func dispatch(deps Deps, streams Streams, argv []string) error {
	if len(argv) == 0 {
		writeUsage(streams.Err)
		return failf(ClassUsage, "a verb is required")
	}
	verb := argv[0]
	switch verb {
	case "help", "-h", "--help":
		writeUsage(streams.Out)
		return nil
	case "version", "--version":
		_, _ = fmt.Fprintf(streams.Out, "agent-grid %s\n", gateway.ProtocolVersion)
		return nil
	}
	for _, candidate := range commands() {
		if candidate.name == verb {
			return candidate.run(deps, streams, argv[1:])
		}
	}
	writeUsage(streams.Err)
	return failf(ClassUsage,
		"unknown verb %q; the first argument is always a verb, never a session selector", verb)
}

// commands is the verb table. Sorted by name so help output is stable
// regardless of declaration order.
func commands() []command {
	list := []command{
		{
			name:    "ls",
			summary: "list sessions on every configured server",
			usage:   listUsage,
			run:     runList,
		},
		{
			name:    "attach",
			summary: "attach the local terminal to a session",
			usage:   attachUsage,
			run:     runAttach,
		},
		{
			name:    "new",
			summary: "create a session and attach to it",
			usage:   newUsage,
			run:     runNew,
		},
		{
			name:    "kill",
			summary: "terminate a session",
			usage:   killUsage,
			run:     runKill,
		},
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}

func writeUsage(out io.Writer) {
	var builder strings.Builder
	builder.WriteString("agent-grid — terminal client for Agent Grid sessions\n\n")
	builder.WriteString("usage: agent-grid <verb> [flags]\n\n")
	for _, candidate := range commands() {
		_, _ = fmt.Fprintf(&builder, "  %-8s %s\n", candidate.name, candidate.summary)
	}
	builder.WriteString("  help     show this message\n")
	builder.WriteString("  version  print the protocol version this build speaks\n")
	_, _ = fmt.Fprint(out, builder.String())
}
