package main

import (
	"flag"
	"fmt"
	"io"
)

// daemonFlagSet collects every flag the merged daemon+gateway binary accepts
// in daemon mode (no subcommand). The struct is the parsed form; the loose
// flag.FlagSet wires from CLI args.
type daemonFlagSet struct {
	dataDir            string
	addr               string
	token              string
	tokenFile          string
	certFile           string
	keyFile            string
	insecure           bool
	noAuth             bool
	allowNoAuthNonLoop bool
}

// defaultDaemonFlags returns the flag values used when daemon mode runs with
// no arguments — the same defaults that flag.Parse would have installed.
func defaultDaemonFlags() *daemonFlagSet {
	return &daemonFlagSet{addr: ":8443"}
}

// parseDaemonArgs extracts daemon-mode flags from args. Called from runMain
// *before* logger init so the daemon log file lands in the flag-specified
// directory rather than the default ~/.agent-grid.
//
// Returns the parsed flag set. When -data-dir is non-empty the caller is
// expected to export AG_DATA_DIR=<value> so config.ResolveDataDir (the
// only path that resolves the runtime's data directory) returns the flag
// value — that is how the flag overrides BOTH settings.toml data_dir AND any
// stray AG_DATA_DIR already in the process env. Without this hop systemd's
// `ExecStart=… -data-dir X` would silently lose to a developer's `export
// AG_DATA_DIR=…` in their shell rc.
func parseDaemonArgs(args []string) (*daemonFlagSet, error) {
	fs := flag.NewFlagSet("agent-grid", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // surface parse errors via return value
	out := defaultDaemonFlags()
	fs.StringVar(&out.dataDir, "data-dir", "",
		"directory for runtime state (socket, sessions, pid). "+
			"Overrides settings.toml data_dir and any inherited AG_DATA_DIR env value.")
	fs.StringVar(&out.addr, "addr", out.addr, "gateway listen address")
	fs.StringVar(&out.token, "token", "",
		"bearer token (generated and printed if empty); ignored with -no-auth")
	fs.StringVar(&out.tokenFile, "token-file", "",
		"path to a file holding the bearer token; if the file does not exist "+
			"it is created (mode 0600) with a freshly generated token, so the "+
			"value survives restarts. Mutually exclusive with -token; ignored "+
			"with -no-auth.")
	fs.StringVar(&out.certFile, "tls-cert", "", "TLS certificate file (self-signed if empty)")
	fs.StringVar(&out.keyFile, "tls-key", "", "TLS key file")
	fs.BoolVar(&out.insecure, "insecure", false, "serve plain HTTP (no TLS) — local dev only")
	fs.BoolVar(&out.noAuth, "no-auth", false,
		"disable bearer-token AND WS-ticket auth — local dev only (loopback only). "+
			"Bind MUST be 127.0.0.1/localhost; refuses non-loopback addrs.")
	fs.BoolVar(&out.allowNoAuthNonLoop, "allow-non-loopback-no-auth", false,
		"opt-in escape hatch: allow -no-auth on non-loopback binds (e.g. 0.0.0.0). "+
			"DANGEROUS — exposes the unauthenticated REST/WS surface to the "+
			"network. Only intended for isolated dev networks.")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("agent-grid: unexpected positional argument %q", fs.Arg(0))
	}
	return out, nil
}
