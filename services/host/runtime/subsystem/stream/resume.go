package stream

import (
	"errors"
	"strings"

	"github.com/takezoh/agent-grid/services/host/state"
)

type normalizedResumeTarget struct {
	rpc state.ResumeTarget
}

func normalizeResumeTarget(target state.ResumeTarget) (normalizedResumeTarget, error) {
	target.ThreadID = strings.TrimSpace(target.ThreadID)
	target.RolloutPath = strings.TrimSpace(target.RolloutPath)
	if target.ThreadID == "" && target.RolloutPath == "" {
		return normalizedResumeTarget{}, nil
	}
	if target.RolloutPath == "" {
		return normalizedResumeTarget{rpc: state.ResumeTarget{ThreadID: target.ThreadID}}, nil
	}
	cliPath, _, err := translateRolloutPath(target.RolloutPath)
	if err != nil {
		return normalizedResumeTarget{}, err
	}
	return normalizedResumeTarget{
		rpc: state.ResumeTarget{ThreadID: target.ThreadID, RolloutPath: cliPath},
	}, nil
}

// translateRolloutPath validates the rollout path. The codex CLI and the daemon
// share one filesystem namespace, so the CLI-visible path and the host path are
// the same value; the pair is kept so callers stay explicit about which one
// they consume.
func translateRolloutPath(path string) (cliPath, hostPath string, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", errors.New("stream backend: codex resume requires rollout path")
	}
	return path, path, nil
}
