package winexec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/takezoh/agent-roost/config"
)

func validateName(name string, allowed []string) error {
	if name == "" {
		return fmt.Errorf("empty exe name")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || strings.ContainsRune(name, 0) {
		return fmt.Errorf("invalid exe name %q: must be a plain basename", name)
	}
	for _, a := range allowed {
		if a == name {
			return nil
		}
	}
	return fmt.Errorf("exe %q is not in the allowlist", name)
}

func resolveExe(name string, resolve map[string]string) string {
	if abs, ok := resolve[name]; ok && abs != "" {
		return abs
	}
	return name
}

func executeRequest(ctx context.Context, cfg config.WinExecConfig, project string, req Request, fds [3]int) int {
	stdin := os.NewFile(uintptr(fds[0]), "stdin")
	stdout := os.NewFile(uintptr(fds[1]), "stdout")
	stderr := os.NewFile(uintptr(fds[2]), "stderr")
	defer stdin.Close()
	defer stdout.Close()
	defer stderr.Close()

	if err := validateName(req.Name, cfg.AllowedExes); err != nil {
		slog.Warn("winexec: request rejected", "project", project, "name", req.Name, "err", err)
		return 1
	}

	abs := resolveExe(req.Name, cfg.Resolve)
	slog.Info("winexec: exec", "project", project, "exe", abs, "args", req.Args)

	cmd := exec.CommandContext(ctx, abs, req.Args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if req.Cwd != "" {
		if _, err := os.Stat(req.Cwd); err == nil {
			cmd.Dir = req.Cwd
		}
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		slog.Warn("winexec: exec failed", "project", project, "exe", abs, "err", err)
		fmt.Fprintf(stderr, "win-exec: exec %s: %v\n", abs, err)
		return 1
	}
	return 0
}
