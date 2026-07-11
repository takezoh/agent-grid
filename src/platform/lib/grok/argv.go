// Package grok contains Grok Build CLI command construction shared by client
// launch code and its contract tests.
package grok

import (
	"fmt"
	"strings"

	"github.com/takezoh/agent-grid/platform/agentlaunch"
)

const DriverName = "grok"

type Lifecycle uint8

const (
	LifecycleFresh Lifecycle = iota
	LifecycleContinue
	LifecycleResume
	LifecycleFork
)

type CommandConfig struct {
	Model  string
	Effort string
}

func ParseCommand(command string) (CommandConfig, error) {
	argv, err := agentlaunch.SplitArgs(strings.TrimSpace(command))
	if err != nil {
		return CommandConfig{}, err
	}
	if len(argv) == 0 || argv[0] != DriverName {
		return CommandConfig{}, fmt.Errorf("grok: unsupported command %q", command)
	}
	var cfg CommandConfig
	for i := 1; i < len(argv); i++ {
		switch argv[i] {
		case "-m", "--model":
			if i+1 < len(argv) {
				cfg.Model = argv[i+1]
				i++
			}
		case "--effort", "--reasoning-effort":
			if i+1 < len(argv) {
				cfg.Effort = argv[i+1]
				i++
			}
		default:
			if strings.HasPrefix(argv[i], "--model=") {
				cfg.Model = strings.TrimPrefix(argv[i], "--model=")
			}
			if strings.HasPrefix(argv[i], "--effort=") {
				cfg.Effort = strings.TrimPrefix(argv[i], "--effort=")
			}
		}
	}
	return cfg, nil
}

func BuildCommand(command string, lifecycle Lifecycle, sessionID, model, effort string) (string, error) {
	argv, err := agentlaunch.SplitArgs(strings.TrimSpace(command))
	if err != nil {
		return "", err
	}
	if len(argv) == 0 || argv[0] != DriverName {
		return "", fmt.Errorf("grok: unsupported command %q", command)
	}
	if hasSessionFlag(argv) {
		return "", fmt.Errorf("grok: command already contains a session lifecycle flag")
	}
	if lifecycle != LifecycleContinue && sessionID == "" {
		return "", fmt.Errorf("grok: session id required")
	}
	argv = append(argv, "--no-auto-update")
	switch lifecycle {
	case LifecycleFresh:
		argv = append(argv, "--session-id", sessionID)
	case LifecycleContinue:
		argv = append(argv, "--continue")
	case LifecycleResume:
		argv = append(argv, "--resume", sessionID)
	case LifecycleFork:
		argv = append(argv, "--resume", sessionID, "--fork-session")
	default:
		return "", fmt.Errorf("grok: unknown lifecycle")
	}
	if model != "" && !hasFlag(argv, "--model", "-m") {
		argv = append(argv, "--model", model)
	}
	if effort != "" && !hasFlag(argv, "--effort", "--reasoning-effort") {
		argv = append(argv, "--effort", effort)
	}
	return agentlaunch.JoinArgs(argv), nil
}

// BuildForkCommand names both the source session and the new fork session.
// Grok allows --session-id together with --fork-session, making the child
// independently recoverable after an agent-grid restart.
func BuildForkCommand(command, parentSessionID, childSessionID, model, effort string) (string, error) {
	if parentSessionID == "" || childSessionID == "" {
		return "", fmt.Errorf("grok: parent and child session ids required for fork")
	}
	argv, err := agentlaunch.SplitArgs(strings.TrimSpace(command))
	if err != nil {
		return "", err
	}
	if len(argv) == 0 || argv[0] != DriverName {
		return "", fmt.Errorf("grok: unsupported command %q", command)
	}
	if hasSessionFlag(argv) {
		return "", fmt.Errorf("grok: command already contains a session lifecycle flag")
	}
	argv = append(argv, "--no-auto-update", "--resume", parentSessionID, "--fork-session", "--session-id", childSessionID)
	if model != "" && !hasFlag(argv, "--model", "-m") {
		argv = append(argv, "--model", model)
	}
	if effort != "" && !hasFlag(argv, "--effort", "--reasoning-effort") {
		argv = append(argv, "--effort", effort)
	}
	return agentlaunch.JoinArgs(argv), nil
}

func hasSessionFlag(argv []string) bool {
	return hasFlag(argv, "--session-id", "-s") || hasFlag(argv, "--resume", "-r") || hasFlag(argv, "--continue") || hasFlag(argv, "--fork-session")
}

func hasFlag(argv []string, flags ...string) bool {
	for _, arg := range argv {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}
