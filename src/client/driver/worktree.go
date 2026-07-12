package driver

import (
	"strings"

	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
)

type worktreeRequest struct {
	Enabled bool
	Name    string
}

func parseWorktreeFlags(command string, flags ...string) (worktreeRequest, string) {
	parts, err := splitCommandTokens(command)
	if err != nil {
		return worktreeRequest{}, strings.TrimSpace(command)
	}
	out := make([]string, 0, len(parts))
	var req worktreeRequest
	for i := 0; i < len(parts); i++ {
		p := parts[i].Value
		matched := false
		for _, flag := range flags {
			switch {
			case p == flag:
				req.Enabled = true
				matched = true
				if i+1 < len(parts) && !strings.HasPrefix(parts[i+1].Value, "-") {
					req.Name = parts[i+1].Value
					i++
				}
			case strings.HasPrefix(p, flag+"="):
				req.Enabled = true
				req.Name = strings.TrimPrefix(p, flag+"=")
				matched = true
			}
			if matched {
				break
			}
		}
		if !matched {
			out = append(out, parts[i].Raw)
		}
	}
	return req, joinCommandText(out)
}

func resolveWorktreeRequest(command string, options state.LaunchOptions, flags ...string) (worktreeRequest, string) {
	req, stripped := parseWorktreeFlags(command, flags...)
	if options.Worktree.Enabled {
		req.Enabled = true
	}
	return req, strings.TrimSpace(stripped)
}

func appendFlag(command, flag string, enabled bool) string {
	command = strings.TrimSpace(command)
	if !enabled || command == "" {
		return command
	}
	if tokens, err := splitCommandTokens(command); err == nil {
		out := rawTokenParts(tokens)
		out = append(out, renderCommandArgs(flag))
		return joinCommandText(out)
	}
	return strings.TrimSpace(command + " " + flag)
}

func appendFlagValue(command, flag, value string) string {
	command = strings.TrimSpace(command)
	value = strings.TrimSpace(value)
	if command == "" || value == "" {
		return command
	}
	if tokens, err := splitCommandTokens(command); err == nil {
		out := rawTokenParts(tokens)
		out = append(out, renderCommandArgs(flag, value))
		return joinCommandText(out)
	}
	return strings.TrimSpace(command + " " + flag + " " + value)
}

func replaceFlagValue(command string, flags []string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(command)
	}
	tokens, err := splitCommandTokens(command)
	if err != nil || len(tokens) == 0 {
		return appendFlagValue(command, flags[0], value)
	}

	isFlag := func(arg string) (matched string, ok bool) {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return flag, true
			}
		}
		return "", false
	}

	out := make([]string, 0, len(tokens)+1)
	replaced := false
	for i := 0; i < len(tokens); i++ {
		arg := tokens[i].Value
		if _, ok := isFlag(arg); ok {
			if !replaced {
				out = append(out, renderCommandArgs(flags[0], value))
				replaced = true
			}
			exactFlag := false
			for _, flag := range flags {
				if arg == flag {
					exactFlag = true
					break
				}
			}
			if exactFlag && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1].Value, "-") {
				i++
			}
			continue
		}
		out = append(out, tokens[i].Raw)
	}
	if !replaced {
		out = append(out, renderCommandArgs(flags[0], value))
	}
	return joinCommandText(out)
}

func stripFlagValues(command string, flags []string) string {
	tokens, err := splitCommandTokens(command)
	if err != nil || len(tokens) == 0 {
		return strings.TrimSpace(command)
	}
	isFlag := func(arg string) bool {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
		return false
	}
	out := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		arg := tokens[i].Value
		if isFlag(arg) {
			exactFlag := false
			for _, flag := range flags {
				if arg == flag {
					exactFlag = true
					break
				}
			}
			if exactFlag && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1].Value, "-") {
				i++
			}
			continue
		}
		out = append(out, tokens[i].Raw)
	}
	return joinCommandText(out)
}

// PreserveLaunchOptions copies transport slots (cols/rows, initial input) from
// the incoming options while overriding worktree intent resolved by the driver.
func PreserveLaunchOptions(options state.LaunchOptions, worktreeEnabled bool) state.LaunchOptions {
	opts := options
	opts.Worktree = state.WorktreeOption{Enabled: worktreeEnabled}
	return opts
}

// CommonPrepareCreate strips worktree flags from command and sets
// LaunchOptions.Worktree.Enabled. The subsystem resolves the actual
// worktree directory during BindFrame; drivers only signal intent here.
func CommonPrepareCreate(c *CommonState, project, command string, options state.LaunchOptions, flags ...string) (state.CreateLaunch, error) {
	req, stripped := resolveWorktreeRequest(command, options, flags...)
	return state.CreateLaunch{
		Command:  strings.TrimSpace(stripped),
		StartDir: project,
		Options: state.LaunchOptions{
			Worktree:     state.WorktreeOption{Enabled: req.Enabled},
			InitialInput: options.InitialInput,
			Cols:         options.Cols,
			Rows:         options.Rows,
		},
	}, nil
}

func splitCommandArgs(command string) ([]string, error) {
	return agentlaunch.SplitArgs(strings.TrimSpace(command))
}

func splitCommandTokens(command string) ([]agentlaunch.ArgToken, error) {
	return agentlaunch.LexArgs(strings.TrimSpace(command))
}

func joinCommandArgs(argv []string) string {
	return strings.TrimSpace(agentlaunch.JoinArgs(argv))
}

func joinCommandText(parts []string) string {
	return strings.TrimSpace(strings.Join(parts, " "))
}

func rawTokenParts(tokens []agentlaunch.ArgToken) []string {
	out := make([]string, len(tokens))
	for i, token := range tokens {
		out[i] = token.Raw
	}
	return out
}

func renderCommandArgs(args ...string) string {
	return strings.TrimSpace(agentlaunch.JoinArgs(args))
}
