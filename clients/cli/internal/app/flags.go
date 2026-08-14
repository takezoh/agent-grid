package app

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// flags.go owns argument parsing for every verb.
//
// The mechanism is the standard library's flag package with ContinueOnError and
// its output discarded. That is deliberate: FlagSet's default ExitOnError would
// decide the process exit code itself, and the exit-code assignment has exactly
// one owner (exit.go). Every parse failure returns through failf instead, so a
// bad flag and an unknown verb reach the user as the same class.

// newFlagSet builds a parser that never writes and never exits. Diagnostics are
// this package's to phrase.
func newFlagSet(verb string) *flag.FlagSet {
	set := flag.NewFlagSet(verb, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.Usage = func() {}
	return set
}

// parse runs the parser and converts any failure — including a stray positional
// argument — into a usage error. Positional arguments are rejected everywhere:
// the only positional slot this CLI has is the verb.
func parse(set *flag.FlagSet, usage string, args []string) error {
	if err := set.Parse(args); err != nil {
		return failf(ClassUsage, "%s\nusage: agent-grid %s", err.Error(), usage)
	}
	if set.NArg() > 0 {
		return failf(ClassUsage,
			"unexpected argument %q; every value is named by a flag\nusage: agent-grid %s",
			set.Arg(0), usage)
	}
	return nil
}

// configFlags are the flags every verb accepts.
//
// --config-dir replaces the whole client configuration directory. It exists
// because every test in this repository must read a temporary directory rather
// than the operator's own configuration, and because the CLI must never write
// there — a test that could only run against the real directory would make that
// rule unverifiable.
type configFlags struct {
	directory string
}

func (config *configFlags) bind(set *flag.FlagSet) {
	set.StringVar(&config.directory, "config-dir", "",
		"read client configuration from this directory instead of the default")
}

// selectorFlags is the target of attach and kill. The kind is always declared:
// exactly one of --id and --prefix, with no fallback between them
// (adr-20260814-cli-selector-kinds-id-and-prefix). Supplying neither, or both,
// is a usage error rather than a guess.
type selectorFlags struct {
	id     string
	prefix string
	server string
}

func (target *selectorFlags) bind(set *flag.FlagSet) {
	set.StringVar(&target.id, "id", "", "select the session whose full identifier is this value")
	set.StringVar(&target.prefix, "prefix", "", "select the session whose identifier starts with this value")
	set.StringVar(&target.server, "server", "",
		"restrict the search to one server, named by its display name or its stable identifier")
}

func (target *selectorFlags) validate(usage string) error {
	switch {
	case target.id == "" && target.prefix == "":
		return failf(ClassUsage,
			"a selector is required: pass --id or --prefix\nusage: agent-grid %s", usage)
	case target.id != "" && target.prefix != "":
		return failf(ClassUsage,
			"--id and --prefix are different selector kinds; pass exactly one\nusage: agent-grid %s",
			usage)
	}
	return nil
}

// parameterFlag collects repeated --param name=value pairs for session
// creation. The profile declares which names are valid and the server validates
// the values, so this only splits and rejects a missing separator.
type parameterFlag struct {
	values map[string]string
}

func (parameters *parameterFlag) String() string {
	if len(parameters.values) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(parameters.values))
	for name, value := range parameters.values {
		pairs = append(pairs, name+"="+value)
	}
	return strings.Join(pairs, ",")
}

func (parameters *parameterFlag) Set(raw string) error {
	name, value, found := strings.Cut(raw, "=")
	if !found || name == "" {
		return fmt.Errorf("--param takes name=value, got %q", raw)
	}
	if parameters.values == nil {
		parameters.values = make(map[string]string)
	}
	parameters.values[name] = value
	return nil
}

// listOptions is the parsed form of `ls`.
type listOptions struct {
	config configFlags
	json   bool
}

const listUsage = "ls [--json] [--config-dir <path>]"

func parseList(args []string) (listOptions, error) {
	var options listOptions
	set := newFlagSet("ls")
	options.config.bind(set)
	set.BoolVar(&options.json, "json", false,
		"emit machine-readable output with server identity and session identity as separate fields")
	if err := parse(set, listUsage, args); err != nil {
		return options, err
	}
	return options, nil
}

// attachOptions is the parsed form of `attach`.
type attachOptions struct {
	config    configFlags
	target    selectorFlags
	detachKey byteFlag
}

const attachUsage = "attach (--id <session-id> | --prefix <id-prefix>) [--server <name-or-id>] " +
	"[--detach-key <byte>]"

func parseAttach(args []string) (attachOptions, error) {
	var options attachOptions
	options.detachKey.value = DefaultDetachByte
	set := newFlagSet("attach")
	options.config.bind(set)
	options.target.bind(set)
	set.Var(&options.detachKey, "detach-key",
		"the single byte that detaches, as a decimal or 0x-prefixed value")
	if err := parse(set, attachUsage, args); err != nil {
		return options, err
	}
	if err := options.target.validate(attachUsage); err != nil {
		return options, err
	}
	return options, nil
}

// DefaultDetachByte is Ctrl-] (0x1D), the byte tmux and telnet both use for the
// same purpose.
//
// Whether it collides with Claude Code's or Codex's own key handling during
// real use is an open observation (evidence-detach-key-chord-collision), which
// is why it is configurable rather than compiled in: the escape hatch exists
// before the answer does.
const DefaultDetachByte = byte(0x1d)

// byteFlag parses a single byte written in decimal or hexadecimal.
type byteFlag struct {
	value byte
}

func (flagValue *byteFlag) String() string {
	return "0x" + strconv.FormatUint(uint64(flagValue.value), 16)
}

func (flagValue *byteFlag) Set(raw string) error {
	parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 0, 8)
	if err != nil {
		return fmt.Errorf("--detach-key takes one byte, like 0x1d or 29; got %q", raw)
	}
	flagValue.value = byte(parsed)
	return nil
}

// newOptions is the parsed form of `new`.
type newOptions struct {
	config     configFlags
	server     string
	agent      string
	cwd        string
	noAttach   bool
	worktree   bool
	detachKey  byteFlag
	parameters parameterFlag
}

const newUsage = "new --server <name-or-id> --agent <profile-id> --cwd <path> " +
	"[--param name=value]... [--worktree] [--no-attach]"

func parseNew(args []string) (newOptions, error) {
	var options newOptions
	options.detachKey.value = DefaultDetachByte
	set := newFlagSet("new")
	options.config.bind(set)
	set.BoolVar(&options.worktree, "worktree", false, "start the session in a fresh git worktree")
	set.Var(&options.detachKey, "detach-key",
		"the single byte that detaches, as a decimal or 0x-prefixed value")
	set.StringVar(&options.server, "server", "",
		"the server to create on, named by its display name or its stable identifier")
	set.StringVar(&options.agent, "agent", "", "the launch profile id to start")
	set.StringVar(&options.cwd, "cwd", "", "the absolute project directory to start in")
	set.BoolVar(&options.noAttach, "no-attach", false, "create the session without attaching to it")
	set.Var(&options.parameters, "param", "a launch profile parameter, as name=value (repeatable)")
	if err := parse(set, newUsage, args); err != nil {
		return options, err
	}
	// The target server is always required, however many are configured. There
	// is no implicit default and no inference from "there is only one"
	// (DP-CREATE-TARGET-SERVER); default_execution in the client configuration
	// exists and would silently satisfy such an inference, so it is never read
	// here.
	if options.server == "" {
		return options, failf(ClassUsage,
			"--server is required; session creation never infers a target server\nusage: agent-grid %s",
			newUsage)
	}
	if options.agent == "" {
		return options, failf(ClassUsage, "--agent is required\nusage: agent-grid %s", newUsage)
	}
	if options.cwd == "" {
		return options, failf(ClassUsage, "--cwd is required\nusage: agent-grid %s", newUsage)
	}
	return options, nil
}

// killOptions is the parsed form of `kill`.
type killOptions struct {
	config configFlags
	target selectorFlags
}

const killUsage = "kill (--id <session-id> | --prefix <id-prefix>) [--server <name-or-id>]"

func parseKill(args []string) (killOptions, error) {
	var options killOptions
	set := newFlagSet("kill")
	options.config.bind(set)
	options.target.bind(set)
	if err := parse(set, killUsage, args); err != nil {
		return options, err
	}
	if err := options.target.validate(killUsage); err != nil {
		return options, err
	}
	return options, nil
}
