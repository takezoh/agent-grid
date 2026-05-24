package agent

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/takezoh/agent-roost/platform/procgroup"
)

// realProc launches "bash -lc command" with Dir=dir and the process environment
// set to os.Environ() extended by env. Returns stdout, stdin, and a wait func
// that reaps the process after stdout has been fully drained.
//
// procgroup.Command runs the agent in its own process group and SIGKILLs the
// whole group when ctx is cancelled, so codex / tool subprocesses spawned by
// the shell are reaped with it rather than orphaned.
func realProc(ctx context.Context, dir string, env map[string]string, command string) (io.ReadCloser, io.WriteCloser, func(), error) {
	merged := os.Environ()
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}
	cmd := procgroup.Command(procgroup.Spec{
		Ctx:  ctx,
		Bin:  "bash",
		Args: []string{"-lc", command},
		Dir:  dir,
		Env:  merged,
	})

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
