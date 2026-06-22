package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveSocket is the enforcement point for the gateway's "no shared daemon"
// contract. Behavioural matrix:
//
//	flag       env         HOME/.agent-reactor/arc.sock?  ARC_ALLOW_SHARED_DAEMON  result
//	---------- ----------- ------------------------------ ------------------------ ------------
//	""         ""          n/a                            n/a                      error (no daemon)
//	" "        " "         n/a                            n/a                      error (whitespace == empty)
//	"/tmp/x"   ""          no                             unset                    /tmp/x
//	""         "/tmp/x"    no                             unset                    /tmp/x (env fallback)
//	"/tmp/a"   "/tmp/b"    no                             unset                    /tmp/a (flag wins)
//	"$HOME/.agent-reactor/arc.sock" ""  yes               unset                    error (shared)
//	"$HOME/.agent-reactor/arc.sock" ""  yes               "1"                      $HOME/...arc.sock (opt-in)

func TestResolveSocket_FlagWins(t *testing.T) {
	got, err := resolveSocket("/tmp/flag.sock", "/tmp/env.sock")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/tmp/flag.sock" {
		t.Fatalf("got %q, want flag value", got)
	}
}

func TestResolveSocket_EnvFallback(t *testing.T) {
	got, err := resolveSocket("", "/tmp/env.sock")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/tmp/env.sock" {
		t.Fatalf("got %q, want env value", got)
	}
}

func TestResolveSocket_BothEmptyErrors(t *testing.T) {
	_, err := resolveSocket("", "")
	if err == nil {
		t.Fatal("expected error when both flag and env are empty")
	}
	// The error must direct the user to BOTH the new -data-dir flag (the
	// recommended path post-incident) and the legacy -arc-sock flag, so an
	// existing user upgrading their command line sees both options.
	for _, expect := range []string{"-data-dir", "-arc-sock"} {
		if !strings.Contains(err.Error(), expect) {
			t.Errorf("error must mention %q so the user knows their options: %v", expect, err)
		}
	}
}

func TestResolveSocket_WhitespaceTreatedAsEmpty(t *testing.T) {
	_, err := resolveSocket("   ", "\t\n")
	if err == nil {
		t.Fatal("whitespace-only inputs must error like empty inputs")
	}
}

func TestResolveSocket_RejectsSharedDaemonPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ARC_ALLOW_SHARED_DAEMON", "")
	shared := filepath.Join(home, ".agent-reactor", "arc.sock")

	_, err := resolveSocket(shared, "")
	if err == nil {
		t.Fatal("expected refusal for shared daemon path")
	}
	if !strings.Contains(err.Error(), "shared arc daemon") {
		t.Fatalf("error did not flag shared-daemon path: %v", err)
	}
	if !strings.Contains(err.Error(), "ARC_ALLOW_SHARED_DAEMON") {
		t.Fatalf("error should mention the override env var: %v", err)
	}
}

func TestResolveSocket_AllowSharedOptIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ARC_ALLOW_SHARED_DAEMON", "1")
	shared := filepath.Join(home, ".agent-reactor", "arc.sock")

	got, err := resolveSocket(shared, "")
	if err != nil {
		t.Fatalf("opt-in should succeed, got err: %v", err)
	}
	if got != shared {
		t.Fatalf("got %q, want %q", got, shared)
	}
}

func TestResolveSocket_NonSharedScratchDirAllowed(t *testing.T) {
	// Scratch dirs under /tmp (the dev script pattern) must always pass —
	// they are not the user's TUI daemon.
	dir := t.TempDir()
	sock := filepath.Join(dir, "arc.sock")
	got, err := resolveSocket(sock, "")
	if err != nil {
		t.Fatalf("scratch path %q must be allowed, got err: %v", sock, err)
	}
	if got != sock {
		t.Fatalf("got %q, want %q", got, sock)
	}
}

func TestIsSharedDaemonPath_OnlyExactMatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(home, ".agent-reactor", "arc.sock"), true},
		{filepath.Join(home, ".agent-reactor", "gateway.sock"), false}, // sibling file
		{filepath.Join(home, ".agent-reactor", "sub", "arc.sock"), false},
		{filepath.Join(home, "arc.sock"), false},
		{"/tmp/arc.sock", false},
		{"/opt/agent-reactor/run/arc.sock", false}, // self-managed install, not TUI default
	}
	for _, tc := range cases {
		if got := isSharedDaemonPath(tc.path); got != tc.want {
			t.Errorf("isSharedDaemonPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsSharedDaemonPath_HomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	// On systems where UserHomeDir fails (no HOME / no passwd entry), the
	// safety net must NOT misfire — it should default to "not shared" so the
	// gateway still starts. There is no canonical "shared" path without HOME.
	if isSharedDaemonPath("/anything") {
		t.Fatal("isSharedDaemonPath must return false when HOME is unavailable")
	}
	// Mock UserHomeDir failure more reliably on systems that fall back to
	// /etc/passwd: ensure the function still doesn't crash.
	_ = os.Getenv // keep import non-empty
}
