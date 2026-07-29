package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/config"
	"github.com/takezoh/agent-grid/services/host/runtime"
	"github.com/takezoh/agent-grid/services/host/state"
	platformconfig "github.com/takezoh/agent-grid/services/platform/config"
)

func TestNewAgentLauncher_direct(t *testing.T) {
	for _, mode := range []string{"", "direct"} {
		resolver := platformconfig.NewSandboxResolver(platformconfig.SandboxConfig{Mode: mode})
		l, _, mgr, err := newAgentLauncher(context.Background(), platformconfig.SandboxConfig{Mode: mode}, resolver, platformconfig.ProjectsConfig{}, t.TempDir(), "", "")
		if err != nil {
			t.Errorf("mode=%q: unexpected error: %v", mode, err)
			continue
		}
		if mgr != nil {
			t.Errorf("mode=%q: expected nil sandbox manager for direct mode", mode)
		}
		if l.IsContainer("/any/project") {
			t.Errorf("mode=%q: expected IsContainer=false for direct mode", mode)
		}
	}
}

func TestNewAgentLauncher_devcontainer_missing(t *testing.T) {
	t.Setenv("PATH", "")
	resolver := platformconfig.NewSandboxResolver(platformconfig.SandboxConfig{Mode: "devcontainer"})
	_, _, mgr, err := newAgentLauncher(context.Background(), platformconfig.SandboxConfig{Mode: "devcontainer"}, resolver, platformconfig.ProjectsConfig{}, t.TempDir(), "", "")
	if err == nil {
		t.Error("expected error when devcontainer CLI is not in PATH, got nil")
	}
	if mgr != nil {
		t.Error("manager must be nil on setup failure")
	}
}

// TestBuildRuntimeWiresRepositoryDevcontainer proves the composition root always
// injects non-nil repository environment authority (typed unavailable in direct mode).
func TestBuildRuntimeWiresRepositoryDevcontainer(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DataDir = dir
	cfg.Sandbox.Mode = "direct"
	cfg.Monitor.PollIntervalMs = 50
	rt, sock, err := buildRuntime(context.Background(), cfg, dir)
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	if sock == "" {
		t.Fatal("socket path empty")
	}
	if !rt.TestHasRepositoryDevcontainer() {
		t.Fatal("RepositoryDevcontainer must be non-nil in production composition")
	}
	// Typed failure: direct mode has no manager → environment_authority_unavailable.
	sessions := filepath.Join(dir, "repository-sessions")
	_ = os.MkdirAll(sessions, 0o755)
	// Exercise through the same interface type buildRuntime injects.
	dc := runtime.UnavailableRepositoryDevcontainer{}
	res, err := dc.Ensure(context.Background(), runtime.RepositoryEnvironmentRequest{
		ServerID: "s",
		Repository: state.GitHubRepositoryIdentity{
			ID: "42", Owner: "acme", Name: "widgets",
		},
		ConfigGen: "g", SessionsRoot: sessions, ClonePath: filepath.Join(sessions, "c"),
	})
	if err != nil || res.Code != runtime.EnvCodeUnavailable {
		t.Fatalf("typed unavailable: %+v err=%v", res, err)
	}
	// Boot ordering: ReconcileRepositoryPreparations is safe after New (no panic).
	rt.ReconcileRepositoryPreparations()
}

func TestShellDisplayName(t *testing.T) {
	cases := []struct {
		shell string
		want  string
	}{
		{"/usr/bin/zsh", "zsh"},
		{"/bin/bash", "bash"},
		{"zsh", "zsh"},
		{"", "shell"},
		{".", "shell"},
		{"/", "shell"},
	}
	for _, c := range cases {
		if got := shellDisplayName(c.shell); got != c.want {
			t.Errorf("shellDisplayName(%q) = %q, want %q", c.shell, got, c.want)
		}
	}
}

// SIGHUP must not kill the daemon. Regression test for the failure mode
// where the daemon process vanished after taking a controlling terminal,
// leaving every frame dead and the user staring at a broken session.
//
// A child holding the TTY can let the parent terminal deliver a spurious
// SIGHUP (terminal closed in WSL/Windows Terminal, controlling-tty races,
// etc.). The default action for SIGHUP is process termination, which would
// kill the daemon while the backend session itself stays up — exactly the
// "all frames EOFed simultaneously, daemon gone, no shutdown log" pattern.
//
// installSignalHandlers must log the signal and ignore it, leaving the
// context live so the daemon keeps serving the backend session.
func TestInstallSignalHandlers_SIGHUP_IgnoredKeepsContextAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := installSignalHandlers(nil, cancel)
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
	stop := installSignalHandlers(nil, cancel)
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
	stop := installSignalHandlers(nil, cancel)
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

// A panic inside the runtime goroutine must not kill the daemon process.
// It must (a) land on errCh as an error, (b) cancel the supervisor
// context so the rest of the daemon shuts down cleanly. Without this
// guard a state.Reduce panic would take the entire daemon down without
// any log line beyond "proto: read loop ended" from the orphaned
// subprocesses.
func TestSuperviseRun_PanicSurfacesErrorAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error {
			panic("synthetic reducer panic")
		})
		close(done)
	}()
	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "synthetic reducer panic") {
			t.Errorf("want panic surfaced via errCh, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not push panic to errCh within 1s")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not cancel the parent context after panic")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not return after panic")
	}
}

// Errors returned by fn (not panics) surface on errCh but do not cancel
// the supervisor context — that path is reserved for unrecoverable
// goroutine panics.
func TestSuperviseRun_ErrorPropagatesWithoutCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	want := errors.New("ordinary runtime error")
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error { return want })
		close(done)
	}()
	select {
	case got := <-errCh:
		if !errors.Is(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("error did not arrive on errCh within 1s")
	}
	if ctx.Err() != nil {
		t.Errorf("ordinary error must not cancel ctx, got %v", ctx.Err())
	}
	<-done
}

// context.Canceled is the cooperative shutdown signal and must never be
// reported as an error. superviseRun closes errCh on return (so callers can
// distinguish "no error reported" from "channel still open"), so the receive
// here uses comma-ok: ok=false means the goroutine returned without sending,
// which is the swallow path we want to assert.
func TestSuperviseRun_ContextCanceledSwallowed(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error { return context.Canceled })
		close(done)
	}()
	select {
	case got, ok := <-errCh:
		if ok {
			t.Fatalf("context.Canceled must not be reported, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not close errCh within 1s")
	}
	<-done
}
