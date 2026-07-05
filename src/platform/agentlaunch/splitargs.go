package agentlaunch

import (
	"fmt"
	"strings"
)

type ArgToken struct {
	Raw   string
	Value string
}

// SplitArgs tokenizes a POSIX-style shell command string into an argv slice.
// Handles single-quoted and double-quoted spans; backslash-escape inside
// double quotes. Intended for simple codex/claude command strings read from
// WORKFLOW.md — not a full POSIX shell lexer.
func SplitArgs(command string) ([]string, error) {
	tokens, err := LexArgs(command)
	if err != nil {
		return nil, err
	}
	args := make([]string, len(tokens))
	for i, token := range tokens {
		args[i] = token.Value
	}
	return args, nil
}

// LexArgs tokenizes a shell command into raw tokens and cooked argv values.
// Raw preserves the original token text so callers rewriting only a subset of
// flags can keep unrelated quoting and expansions byte-for-byte intact.
func LexArgs(command string) ([]ArgToken, error) {
	var args []string
	var cur strings.Builder
	inToken := false
	tokenStart := -1
	const (
		stateBare = iota
		stateSingle
		stateDouble
	)
	mode := stateBare

	for i := 0; i < len(command); {
		c := command[i]
		switch mode {
		case stateSingle:
			if c == '\'' {
				mode = stateBare
			} else {
				cur.WriteByte(c)
			}
			i++
			continue
		case stateDouble:
			if c == '"' {
				mode = stateBare
				i++
				continue
			}
			if c == '\\' {
				if i+1 >= len(command) {
					return nil, fmt.Errorf("agentlaunch: unterminated escape in %q", command)
				}
				i++
				cur.WriteByte(command[i])
				i++
				continue
			}
			cur.WriteByte(c)
			i++
			continue
		}

		switch c {
		case ' ', '\t', '\n':
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
				tokenStart = -1
			}
			i++
		case '\'':
			if !inToken {
				inToken = true
				tokenStart = i
			}
			mode = stateSingle
			i++
		case '"':
			if !inToken {
				inToken = true
				tokenStart = i
			}
			mode = stateDouble
			i++
		case '\\':
			if i+1 >= len(command) {
				return nil, fmt.Errorf("agentlaunch: unterminated escape in %q", command)
			}
			if !inToken {
				inToken = true
				tokenStart = i
			}
			i++
			cur.WriteByte(command[i])
			i++
		default:
			if !inToken {
				inToken = true
				tokenStart = i
			}
			cur.WriteByte(c)
			i++
		}
	}
	if mode == stateSingle {
		return nil, fmt.Errorf("agentlaunch: unterminated single quote in %q", command)
	}
	if mode == stateDouble {
		return nil, fmt.Errorf("agentlaunch: unterminated double quote in %q", command)
	}
	if inToken {
		args = append(args, cur.String())
	}

	rawTokens := make([]ArgToken, 0, len(args))
	cur.Reset()
	inToken = false
	mode = stateBare
	tokenStart = -1
	valueIdx := 0
	for i := 0; i < len(command); {
		c := command[i]
		switch mode {
		case stateSingle:
			if c == '\'' {
				mode = stateBare
			}
			i++
			continue
		case stateDouble:
			if c == '"' {
				mode = stateBare
				i++
				continue
			}
			if c == '\\' && i+1 < len(command) {
				i += 2
				continue
			}
			i++
			continue
		}
		switch c {
		case ' ', '\t', '\n':
			if inToken {
				rawTokens = append(rawTokens, ArgToken{Raw: command[tokenStart:i], Value: args[valueIdx]})
				valueIdx++
				inToken = false
				tokenStart = -1
			}
			i++
		case '\'':
			if !inToken {
				inToken = true
				tokenStart = i
			}
			mode = stateSingle
			i++
		case '"':
			if !inToken {
				inToken = true
				tokenStart = i
			}
			mode = stateDouble
			i++
		case '\\':
			if !inToken {
				inToken = true
				tokenStart = i
			}
			if i+1 < len(command) {
				i += 2
			} else {
				i++
			}
		default:
			if !inToken {
				inToken = true
				tokenStart = i
			}
			i++
		}
	}
	if inToken {
		rawTokens = append(rawTokens, ArgToken{Raw: command[tokenStart:], Value: args[valueIdx]})
	}
	return rawTokens, nil
}
