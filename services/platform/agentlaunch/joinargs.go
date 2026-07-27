package agentlaunch

import "strings"

// JoinArgs renders argv back into a POSIX-style shell command string that
// SplitArgs can parse without losing argument boundaries. Plain safe tokens are
// emitted unchanged. Unsafe args are single-quoted with the standard POSIX
// '\” idiom so shell evaluation cannot reinterpret literals.
func JoinArgs(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = quoteArg(arg)
	}
	return strings.Join(out, " ")
}

func quoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if isSafeArg(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func isSafeArg(arg string) bool {
	for _, r := range arg {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("@%_+=:,./-", r):
		default:
			return false
		}
	}
	return true
}
