package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// claudeLauncher starts a claude process and returns its stdout, a wait func, and any startup error.
// resumeSessionID is empty for a new session; non-empty triggers --resume.
type claudeLauncher func(ctx context.Context, cwd, resumeSessionID, prompt string) (io.ReadCloser, func() error, error)

func realLaunch(ctx context.Context, cwd, resumeSessionID, prompt string) (io.ReadCloser, func() error, error) {
	bin := os.Getenv("CLAUDE_BIN")
	if bin == "" {
		bin = "claude"
	}

	args := []string{"-p", "--output-format", "stream-json"}
	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill the whole process group on context cancellation so claude's children
	// (tool subprocesses) are also terminated.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, cmd.Wait, nil
}
