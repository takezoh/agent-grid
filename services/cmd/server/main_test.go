package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/services/host/config"
)

// TestIsLoopbackAddr pins listener classification for the no-auth warning.
func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8443", true},
		{"127.0.0.5:8443", true},
		{"[::1]:8443", true},
		{"localhost:8443", true},
		{"127.0.0.1", true},
		{"::1", true},
		{":8443", false},        // wildcard — binds all interfaces
		{"0.0.0.0:8443", false}, // explicit all-interfaces
		{"[::]:8443", false},    // IPv6 wildcard
		{"192.168.1.5:8443", false},
		{"10.0.0.1:8443", false},
		{"example.com:8443", false}, // unresolved hostname — refuse
		{"", false},
	}
	for _, c := range cases {
		if got := isLoopbackAddr(c.addr); got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}

// === runMain dispatch — daemon, subcommand, help ===

func TestRunMainDaemonSuccess(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	runDaemonFn = func(*daemonFlagSet, io.Writer) error { return nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if got := stdout.String(); got != "server: exited\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMainDaemonErrorLogsAndPrints(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	wantErr := errors.New("boom")
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	runDaemonFn = func(*daemonFlagSet, io.Writer) error { return wantErr }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != "server: boom\n" {
		t.Fatalf("stderr = %q", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "main failed") {
		t.Fatalf("log = %q, want main failed entry", string(data))
	}
}

func TestMainDaemonStartupErrorSurvivesFD2Redirect(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainDaemonStartupErrorHelper$")
	cmd.Env = append(os.Environ(),
		"AG_TEST_MAIN_DAEMON_STARTUP_ERROR=1",
		"AG_TEST_MAIN_DATA_DIR="+dir,
		"AG_DATA_DIR=",
		"HOME="+t.TempDir(),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("exit error = %v, want exit code 1 (stderr=%q)", err, stderr.String())
	}
	const cause = "authentication mode is required"
	if !strings.Contains(stderr.String(), cause) {
		t.Fatalf("parent stderr = %q, want startup cause %q", stderr.String(), cause)
	}
	logData, readErr := os.ReadFile(filepath.Join(dir, "server.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(logData), cause) {
		t.Fatalf("server.log = %q, want startup cause %q", string(logData), cause)
	}
}

func TestMainDaemonStartupErrorHelper(_ *testing.T) {
	if os.Getenv("AG_TEST_MAIN_DAEMON_STARTUP_ERROR") != "1" {
		return
	}
	os.Args = []string{
		"server",
		"-data-dir", os.Getenv("AG_TEST_MAIN_DATA_DIR"),
		"-addr", "127.0.0.1:0",
		"-insecure",
	}
	main()
}

func TestRunMainUnknownCommandDoesNotRunDaemon(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	daemonCalled := false
	runDaemonFn = func(*daemonFlagSet, io.Writer) error {
		daemonCalled = true
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain([]string{"totally-unknown-command"}, &stdout, &stderr)
	if daemonCalled {
		t.Fatal("runDaemonFn must not be called for unknown subcommands")
	}
	if code == 0 {
		t.Fatal("unknown command should exit non-zero")
	}
}

func TestRunMainHelp(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	initLoggerWithDataDir = func(level, dir string) error { return nil }
	runDaemonFn = func(*daemonFlagSet, io.Writer) error {
		t.Fatal("daemon must not run on help")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want usage banner", stdout.String())
	}
}

func TestRunMainClassifies(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want commandKind
	}{
		{"empty", nil, commandKindDaemon},
		{"data-dir flag", []string{"-data-dir", "/tmp/x"}, commandKindDaemon},
		{"addr flag", []string{"-addr", "127.0.0.1:0"}, commandKindDaemon},
		{"insecure flag", []string{"-insecure"}, commandKindDaemon},
		{"help", []string{"help"}, commandKindHelp},
		{"-h", []string{"-h"}, commandKindHelp},
		{"--help", []string{"--help"}, commandKindHelp},
		{"event subcommand", []string{"event", "SomeType"}, commandKindCLI},
		{"host-exec subcommand", []string{"host-exec", "ls"}, commandKindCLI},
		{"mcp-exec subcommand", []string{"mcp-exec", "alias"}, commandKindCLI},
		{"agent-frames-mcp subcommand", []string{"agent-frames-mcp", "--sock", "/tmp/x.sock"}, commandKindCLI},
		{"frame-exec subcommand", []string{"frame-exec"}, commandKindCLI},
		{"supervisor-probe subcommand", []string{"supervisor-probe", "-data-dir", "/tmp/x"}, commandKindCLI},
		{"supervisor-stop subcommand", []string{"supervisor-stop", "-data-dir", "/tmp/x"}, commandKindCLI},
		{"unknown positional", []string{"bogus-command"}, commandKindCLI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyCommand(tc.args); got != tc.want {
				t.Fatalf("classifyCommand(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func stubMainDeps(t *testing.T) func() {
	t.Helper()
	t.Setenv("AG_DATA_DIR", "")
	prevLoadBootstrapConfig := loadBootstrapConfig
	prevInitLogger := initLoggerWithDataDir
	prevCloseLogger := closeLogger
	prevRedirectStderr := redirectStderr
	prevParseDaemonArgs := parseDaemonArgsFn
	prevRunDaemon := runDaemonFn

	return func() {
		loadBootstrapConfig = prevLoadBootstrapConfig
		initLoggerWithDataDir = prevInitLogger
		closeLogger = prevCloseLogger
		redirectStderr = prevRedirectStderr
		parseDaemonArgsFn = prevParseDaemonArgs
		runDaemonFn = prevRunDaemon
	}
}
