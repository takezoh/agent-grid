package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// daemonFlagNames lists the long-form names of every flag the runtime
// coordinator accepts. classifyCommand consults this set to route
// `arc -data-dir ...` to commandKindRoost instead of the unknown-CLI fallback.
var daemonFlagNames = []string{"data-dir"}

// isCoordinatorFlag reports whether arg is one of the runtime coordinator's
// flags in any of Go's flag syntaxes: -name, --name, -name=value, --name=value.
func isCoordinatorFlag(arg string) bool {
	for _, name := range daemonFlagNames {
		for _, prefix := range []string{"-" + name, "--" + name} {
			if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
				return true
			}
		}
	}
	return false
}

// parseDaemonArgs extracts runtime coordinator flags from args. Called from
// runMain *before* logger init so the daemon log file lands in the
// flag-specified directory rather than the default ~/.agent-reactor.
//
// Returns the resolved -data-dir value (empty if not specified). When non-
// empty, the caller is expected to export ROOST_DATA_DIR=<value> so
// Config.ResolveDataDir (the only path that resolves the runtime's data
// directory) returns the flag value — that is how the flag overrides BOTH
// settings.toml data_dir AND any stray ROOST_DATA_DIR already in the
// process env. Without this hop systemd's `ExecStart=… -data-dir X` would
// silently lose to a developer's `export ROOST_DATA_DIR=…` in their shell rc.
func parseDaemonArgs(args []string) (dataDir string, err error) {
	fs := flag.NewFlagSet("agent-reactor runtime", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // surface parse errors via return value
	dd := fs.String("data-dir", "",
		"directory for runtime state (socket, sessions, pid). "+
			"Overrides settings.toml data_dir and any inherited "+
			"ROOST_DATA_DIR env value.")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", fmt.Errorf("agent-reactor runtime: unexpected positional argument %q", fs.Arg(0))
	}
	return *dd, nil
}
