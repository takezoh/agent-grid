// Package shellalias resolves a user's interactive shell aliases on the host.
//
// roost lets users list shell aliases (e.g. "sonnet-medium") as session
// commands. Such aliases live only in the login shell's interactive rc files
// and cannot be expanded by a plain exec. Resolve launches the login shell
// once at startup (interactive) and asks it to expand each name, yielding a
// name→expansion map the session reducer consumes before driver lookup.
package shellalias

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// LoginShellCommand is a shell snippet (a quoted command substitution) that
// expands to the current user's login shell path when evaluated in the target
// shell context. It is embedded into generated launch commands so the same
// passwd-based resolution applies on the host (direct mode) and inside the
// container (devcontainer mode).
const LoginShellCommand = `"$(getent passwd "$(id -un)" | cut -d: -f7)"`

const (
	aliasBegin   = "__ROOST_ALIAS_BEGIN__"
	aliasEnd     = "__ROOST_ALIAS_END__"
	queryTimeout = 10 * time.Second
)

// Runner executes name with args and returns its stdout. Injectable for tests.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// RealRunner runs the command and returns stdout only. stderr is captured into
// the returned error on failure (never mixed into stdout).
func RealRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// LoginShell returns the absolute path of the current user's login shell from
// the passwd database (field 7), e.g. "/usr/bin/zsh". Only the first line of
// the getent response is consulted, so a duplicate NSS entry can't corrupt the
// path with an embedded newline.
func LoginShell(ctx context.Context, run Runner) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("shellalias: current user: %w", err)
	}
	out, err := run(ctx, "getent", "passwd", u.Username)
	if err != nil {
		return "", fmt.Errorf("shellalias: getent passwd %s: %w", u.Username, err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	fields := strings.Split(strings.TrimSpace(first), ":")
	if len(fields) < 7 || fields[6] == "" {
		return "", fmt.Errorf("shellalias: no login shell in passwd entry %q", first)
	}
	return fields[6], nil
}

// Resolve launches shell interactively and expands each name that is a shell
// alias. Names with no expansion are omitted (callers fall back to the literal
// command). On any failure it returns an empty map and an error; callers log
// and continue so startup never blocks on a missing or unusual shell.
//
// Success requires the script's closing sentinel: if the shell is killed
// (e.g. the timeout fires) after emitting only part of the block, the partial
// output is discarded rather than silently dropping the unseen aliases.
func Resolve(ctx context.Context, shell string, names []string, run Runner) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}
	script, ok := buildQuery(shell, names)
	if !ok {
		return map[string]string{}, fmt.Errorf("shellalias: unsupported login shell %q", shell)
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	out, runErr := run(ctx, shell, "-i", "-c", script)
	result, closed := parse(string(out))
	if !closed {
		return map[string]string{}, fmt.Errorf("shellalias: incomplete output from %s (truncated or failed: %v)", shell, runErr)
	}
	return result, nil
}

// buildQuery returns an interactive-shell script that prints "name<TAB>value"
// for each name's alias expansion, wrapped in sentinel markers so rc-file
// output on stdout is ignored. ok is false for shells whose alias lookup
// syntax is unknown.
func buildQuery(shell string, names []string) (string, bool) {
	var lookup string
	switch filepath.Base(shell) {
	case "zsh":
		lookup = "${aliases[$n]}"
	case "bash":
		lookup = "${BASH_ALIASES[$n]}"
	default:
		return "", false
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = sqEscape(n)
	}
	script := strings.Join([]string{
		"printf '%s\\n' " + sqEscape(aliasBegin),
		"for n in " + strings.Join(quoted, " ") + "; do printf '%s\\t%s\\n' \"$n\" \"" + lookup + "\"; done",
		"printf '%s\\n' " + sqEscape(aliasEnd),
	}, "; ")
	return script, true
}

// parse extracts the sentinel-delimited block and maps name→expansion,
// dropping entries with an empty name or empty expansion. closed reports
// whether the closing sentinel was seen — false means the output was
// truncated and the (partial) map must not be trusted.
func parse(raw string) (result map[string]string, closed bool) {
	result = map[string]string{}
	inBlock := false
	for line := range strings.SplitSeq(raw, "\n") {
		switch {
		case line == aliasBegin:
			inBlock = true
		case line == aliasEnd:
			return result, true
		case inBlock:
			if tab := strings.IndexByte(line, '\t'); tab > 0 {
				if val := line[tab+1:]; val != "" {
					result[line[:tab]] = val
				}
			}
		}
	}
	return result, false
}

// sqEscape single-quotes s for safe embedding in a shell command.
func sqEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
