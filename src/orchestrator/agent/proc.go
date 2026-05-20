package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// realProc launches "bash -lc cmdLine" with cwd set to cwd and returns its
// stdout (reader), stdin (writer), and a wait func. The process is tied to ctx;
// cancelling ctx terminates it via exec.CommandContext. The caller must invoke
// wait once stdout has been fully read to reap the process.
func realProc(ctx context.Context, cwd, cmdLine string) (io.ReadCloser, io.WriteCloser, func(), error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", cmdLine) //nolint:gosec
	cmd.Dir = cwd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("agent: stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("agent: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("agent: start process: %w", err)
	}
	return stdout, stdin, func() { _ = cmd.Wait() }, nil
}
