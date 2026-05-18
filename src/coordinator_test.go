package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/config"
	appruntime "github.com/takezoh/agent-roost/runtime"
)

func TestNewAgentLauncher_direct(t *testing.T) {
	for _, mode := range []string{"", "direct"} {
		resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: mode})
		l, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: mode}, resolver, config.ProjectsConfig{}, t.TempDir(), "")
		if err != nil {
			t.Errorf("mode=%q: unexpected error: %v", mode, err)
			continue
		}
		d, ok := l.(*appruntime.SandboxDispatcher)
		if !ok {
			t.Errorf("mode=%q: expected *SandboxDispatcher, got %T", mode, l)
			continue
		}
		if d.Devcontainer != nil {
			t.Errorf("mode=%q: expected Devcontainer=nil for direct mode, got %T", mode, d.Devcontainer)
		}
	}
}

func TestNewAgentLauncher_devcontainer_missing(t *testing.T) {
	t.Setenv("PATH", "")
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	_, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: "devcontainer"}, resolver, config.ProjectsConfig{}, t.TempDir(), "")
	if err == nil {
		t.Error("expected error when devcontainer CLI is not in PATH, got nil")
	}
}

func TestResolveShellDisplayFromValues(t *testing.T) {
	cases := []struct {
		tmuxDefault string
		envSHELL    string
		want        string
	}{
		{"/usr/bin/zsh", "/bin/bash", "zsh"},
		{"", "/bin/bash", "bash"},
		{"", "/usr/bin/zsh", "zsh"},
		{"", "", "shell"},
		{".", "", "shell"},
		{"", ".", "shell"},
		{".", ".", "shell"},
	}
	for _, c := range cases {
		got := resolveShellDisplayFromValues(c.tmuxDefault, c.envSHELL)
		if got != c.want {
			t.Errorf("resolveShellDisplayFromValues(%q, %q) = %q, want %q",
				c.tmuxDefault, c.envSHELL, got, c.want)
		}
	}
}

func TestRunCoordinatorRejectsInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	err := runCoordinator()
	if err == nil {
		t.Fatal("expected error when $TMUX is set, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to start coordinator") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// SIGHUP must not kill the daemon. Regression test for the failure mode
// where the daemon process vanished after `attaching to tmux session`,
// leaving every TUI pane dead and the user staring at a broken session.
//
// `tmux attach-session` runs as a child of the daemon; once it takes the
// TTY the parent terminal can deliver a spurious SIGHUP (pane closed in
// WSL/Windows Terminal, controlling-tty races, etc.). The default action
// for SIGHUP is process termination, which would kill the daemon while
// the tmux session itself stays up — exactly the "all 4 TUI panes EOFed
// simultaneously, daemon gone, no shutdown log" pattern.
//
// installSignalHandlers must log the signal and ignore it, leaving the
// context live so the daemon keeps serving the tmux session.
func TestInstallSignalHandlers_SIGHUP_IgnoredKeepsContextAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := installSignalHandlers(cancel)
	defer stop()

	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
	// Allow the goroutine to consume the signal.
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			if err := ctx.Err(); err != nil {
				t.Fatalf("SIGHUP cancelled the context: %v", err)
			}
			return
		default:
			if ctx.Err() != nil {
				t.Fatalf("SIGHUP cancelled the context: %v", ctx.Err())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// SIGTERM must cancel the context for graceful shutdown.
func TestInstallSignalHandlers_SIGTERM_CancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := installSignalHandlers(cancel)
	defer stop()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("SIGTERM did not cancel context within 1s")
	}
}

// stop() must unblock even when no signals arrived. Guards against a
// goroutine leak in long-lived tests / repeated start-stop cycles.
func TestInstallSignalHandlers_StopUnblocksWithNoSignals(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := installSignalHandlers(cancel)
	doneCh := make(chan struct{})
	go func() {
		stop()
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("stop() did not return within 1s with no signals delivered")
	}
}

func TestShouldKeepRuntimeAliveAfterAttach(t *testing.T) {
	errAttach := errors.New("attach failed")
	if !shouldKeepRuntimeAliveAfterAttach(errAttach, true) {
		t.Fatal("want keep-alive when attach failed and session exists")
	}
	if shouldKeepRuntimeAliveAfterAttach(nil, true) {
		t.Fatal("did not expect keep-alive on clean detach")
	}
	if shouldKeepRuntimeAliveAfterAttach(errAttach, false) {
		t.Fatal("did not expect keep-alive when session is gone")
	}
}
