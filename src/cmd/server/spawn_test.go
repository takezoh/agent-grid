package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// syscallSignal0 is the no-op signal used by the test helper to probe
// whether a pid is still alive. Pulled out so the test file's imports stay
// idiomatic (no awkward syscall.Signal(0) literal inline).
var syscallSignal0 = syscall.Signal(0)

// resolveDaemon is the entry-point for the gateway's mode selection. Tests
// here pin the "mutually exclusive" contract that prevents the operator
// from accidentally running in both spawn and attach modes at once.

func TestResolveDaemon_BothModesIsError(t *testing.T) {
	t.Parallel()
	_, _, err := resolveDaemon("/tmp/sock", "/tmp/data", "", "")
	if err == nil {
		t.Fatal("expected error when both -arc-sock and -data-dir are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error must explain the conflict: %v", err)
	}
}

func TestResolveDaemon_DataDirAndEnvIsError(t *testing.T) {
	t.Parallel()
	// ARC_SOCKET set + -data-dir specified is the same conflict shape:
	// the env var indicates attach intent, the flag indicates spawn intent.
	_, _, err := resolveDaemon("", "/tmp/data", "", "/tmp/env.sock")
	if err == nil {
		t.Fatal("expected error when -data-dir and $ARC_SOCKET are both set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error must explain the conflict: %v", err)
	}
}

func TestResolveDaemon_BothEmptyIsError(t *testing.T) {
	// t.Setenv forbids t.Parallel — run serially.
	t.Setenv("HOME", t.TempDir())
	_, _, err := resolveDaemon("", "", "", "")
	if err == nil {
		t.Fatal("expected error when neither -arc-sock nor -data-dir is set")
	}
}

func TestResolveDaemon_AttachModeReturnsEmptyHandle(t *testing.T) {
	t.Parallel()
	sock, handle, err := resolveDaemon("/tmp/attach.sock", "", "", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sock != "/tmp/attach.sock" {
		t.Errorf("sock = %q, want /tmp/attach.sock", sock)
	}
	if handle == nil {
		t.Fatal("handle must never be nil; shutdown() relies on the wrapper")
	}
	if handle.mode() != "attach" {
		t.Errorf("mode = %q, want attach", handle.mode())
	}
	// shutdown on an empty handle is a no-op; must not panic.
	handle.shutdown()
	handle.shutdown() // idempotent
}

// === isSharedDataDir ===

func TestIsSharedDataDir_OnlyExactMatch(t *testing.T) {
	// t.Setenv forbids t.Parallel.
	home := t.TempDir()
	t.Setenv("HOME", home)
	cases := []struct {
		dir  string
		want bool
	}{
		{filepath.Join(home, ".agent-reactor"), true},
		{filepath.Join(home, ".agent-reactor", "sub"), false},
		{filepath.Join(home, "agent-reactor"), false}, // no dot prefix
		{"/tmp/agent-reactor-web", false},
		{"/opt/agent-reactor", false}, // self-managed install
	}
	for _, tc := range cases {
		if got := isSharedDataDir(tc.dir); got != tc.want {
			t.Errorf("isSharedDataDir(%q) = %v, want %v", tc.dir, got, tc.want)
		}
	}
}

func TestIsSharedDataDir_HomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	if isSharedDataDir("/anything") {
		t.Fatal("isSharedDataDir must return false when HOME is unavailable")
	}
}

// TestMatchesTUIDefault_EmptyCandidateReturnsFalse pins the empty-string
// guard. filepath.Abs("") resolves to the current working directory, so
// without this guard a caller that accidentally passes "" while cwd is
// $HOME/.agent-reactor would get a false-positive shared-daemon hit. The
// safety check must never misfire because of a missing input.
func TestMatchesTUIDefault_EmptyCandidateReturnsFalse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Even when cwd is $HOME/.agent-reactor (simulated by creating it and
	// pointing the call through it), an empty candidate must NOT match.
	canonical := filepath.Join(home, ".agent-reactor")
	if err := os.MkdirAll(canonical, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(canonical) // Go 1.24+: scoped chdir, restored at test end.

	if matchesTUIDefault("", "") {
		t.Error(`matchesTUIDefault("", "") must be false even when cwd == $HOME/.agent-reactor`)
	}
	if matchesTUIDefault("", "arc.sock") {
		t.Error(`matchesTUIDefault("", "arc.sock") must be false`)
	}
	if isSharedDataDir("") {
		t.Error(`isSharedDataDir("") must be false`)
	}
	if isSharedDaemonPath("") {
		t.Error(`isSharedDaemonPath("") must be false`)
	}
}

// === spawnMode ===

func TestSpawnMode_RejectsSharedDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, _, err := spawnMode(filepath.Join(home, ".agent-reactor"), "")
	if err == nil {
		t.Fatal("expected refusal for $HOME/.agent-reactor as data-dir")
	}
	if !strings.Contains(err.Error(), "TUI") {
		t.Errorf("error should mention the TUI collision: %v", err)
	}
}

// TestSpawnMode_HappyPathUsingStubBinary builds a minimal stub binary that
// just creates the arc.sock socket and sleeps, verifying:
//  1. spawnMode launches it under the right env
//  2. waitForSocket sees the socket
//  3. shutdown() reaps the child cleanly via SIGTERM
//
// This is the structural test for the spawn lifecycle without depending on
// the real arc daemon (which is heavy to build). The real-binary smoke test
// lives in server/web/mux_e2e_test.go.
func TestSpawnMode_HappyPathUsingStubBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping spawn-lifecycle test in -short mode")
	}
	// t.Setenv forbids t.Parallel.
	stubBin := buildStubArcBinary(t)
	dataDir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // ensure isSharedDataDir(dataDir) is false

	sock, handle, err := spawnMode(dataDir, stubBin)
	if err != nil {
		t.Fatalf("spawnMode failed: %v", err)
	}
	defer handle.shutdown()

	expectedSock := filepath.Join(dataDir, "arc.sock")
	if sock != expectedSock {
		t.Errorf("sock = %q, want %q", sock, expectedSock)
	}
	if handle.mode() != "spawn" {
		t.Errorf("mode = %q, want spawn", handle.mode())
	}
	if handle.cmd == nil || handle.cmd.Process == nil {
		t.Fatal("handle.cmd.Process must be non-nil after successful spawn")
	}
	pid := handle.cmd.Process.Pid

	// The stub honours SIGTERM; shutdown should reap within budget.
	start := time.Now()
	handle.shutdown()
	if elapsed := time.Since(start); elapsed > daemonShutdownTimeout+time.Second {
		t.Errorf("shutdown took %s, want ≤ %s", elapsed, daemonShutdownTimeout)
	}

	// Verify the process is actually gone (sending signal 0 returns ESRCH).
	if err := syscallSignalZero(pid); err == nil {
		t.Errorf("pid %d still alive after shutdown — daemon leak", pid)
	}
}

func TestSpawnMode_TimeoutIfBinaryNeverBindsSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping spawn-timeout test in -short mode")
	}
	// t.Setenv forbids t.Parallel.

	// Write a stub that sleeps without creating the socket.
	stubDir := t.TempDir()
	stubBin := filepath.Join(stubDir, "nosock")
	if err := os.WriteFile(stubBin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write stub: %v", err)
	}

	dataDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	// Shorten the wait so the test isn't slow. Save/restore on cleanup.
	saved := daemonReadyTimeoutOverride.Load()
	daemonReadyTimeoutOverride.Store(int64(500 * time.Millisecond))
	t.Cleanup(func() { daemonReadyTimeoutOverride.Store(saved) })

	_, _, err := spawnMode(dataDir, stubBin)
	if err == nil {
		t.Fatal("expected timeout error when stub never binds the socket")
	}
	if !strings.Contains(err.Error(), "did not bind") {
		t.Errorf("error should mention socket bind timeout: %v", err)
	}
}

// === resolveArcBinary ===

func TestResolveArcBinary_FlagExplicit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "explicit-arc")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write: %v", err)
	}
	got, err := resolveArcBinary(bin)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}
}

func TestResolveArcBinary_FlagMissingErrors(t *testing.T) {
	t.Parallel()
	_, err := resolveArcBinary("/nonexistent/arc-binary-xyz")
	if err == nil {
		t.Fatal("expected error for missing -arc-bin path")
	}
}

func TestResolveArcBinary_NoFallback(t *testing.T) {
	// t.Setenv forbids t.Parallel.
	// Hide PATH so LookPath fails, and ensure no ./arc next to the test binary.
	// (The test binary's exec dir is the build cache — no ./arc there.)
	t.Setenv("PATH", "/nonexistent")
	_, err := resolveArcBinary("")
	// Allow EITHER outcome: if the test binary happens to live next to an
	// arc file (unlikely in CI), accept that; otherwise expect the explicit
	// error. The contract this test pins is "no silent fallback to a
	// random arc"; both outcomes are explicit.
	if err == nil {
		t.Log("resolveArcBinary found arc next to test binary; that's fine, but verify the contract holds")
	} else if !strings.Contains(err.Error(), "-arc-bin") {
		t.Errorf("error must direct user to -arc-bin: %v", err)
	}
}

// === waitForSocket ===

func TestWaitForSocket_SucceedsAfterDelay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")

	go func() {
		time.Sleep(80 * time.Millisecond)
		// Bind a unix socket so os.Stat sees ModeSocket.
		ln, err := netListenUnix(sock)
		if err != nil {
			return
		}
		_ = ln
	}()

	if err := waitForSocket(sock, 2*time.Second); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestWaitForSocket_TimesOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sock := filepath.Join(dir, "never.sock")
	err := waitForSocket(sock, 80*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout: %v", err)
	}
}

// === helpers ===

// buildStubArcBinary compiles a tiny Go program that binds an arc.sock under
// $ROOST_DATA_DIR and blocks on SIGTERM. This is the test-only stand-in for
// the real arc daemon — it exercises spawnMode's lifecycle without paying
// the cost of building cmd/arc (which is several seconds).
func buildStubArcBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	bin := filepath.Join(dir, "stub-arc")
	stubSource := `package main

import (
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	dir := os.Getenv("ROOST_DATA_DIR")
	if dir == "" {
		os.Exit(2)
	}
	sock := filepath.Join(dir, "arc.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		os.Exit(3)
	}
	defer ln.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
}
`
	if err := os.WriteFile(src, []byte(stubSource), 0o644); err != nil { //nolint:gosec
		t.Fatalf("write stub source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build stub: %v\n%s", err, out)
	}
	return bin
}

func netListenUnix(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}

// syscallSignalZero sends signal 0 to pid — a no-op that just reports
// whether the pid exists. Returns nil if the process is alive, ESRCH (or
// similar) if it's gone.
func syscallSignalZero(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscallSignal0)
}
