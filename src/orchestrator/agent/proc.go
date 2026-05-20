package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// realProc launches "bash -lc cmdLine" with cwd set to cwd and returns its
// stdout (reader) and stdin (writer). The process is tied to ctx; cancelling
// ctx terminates it via exec.CommandContext.
func realProc(ctx context.Context, cwd, cmdLine string) (io.ReadCloser, io.WriteCloser, error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", cmdLine) //nolint:gosec
	cmd.Dir = cwd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("agent: stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("agent: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("agent: start process: %w", err)
	}
	return stdout, stdin, nil
}
