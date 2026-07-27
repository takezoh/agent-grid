//go:build windows

package wslhost

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyWindowsACLRejectsUnresolvableIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	applier := WindowsACLApplier{}
	err := applier.ApplyLogACL(path, "not-a-real-account@example.com")
	if err == nil {
		t.Fatal("expected an error for an unresolvable identity, got nil")
	}
	if !strings.Contains(err.Error(), "whoami") {
		t.Errorf("error should point the operator at `whoami` to find their local account name, got: %v", err)
	}
}

func TestApplyAndValidateWindowsACLRoundTrip(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	applier := WindowsACLApplier{}
	if err := applier.ApplyLogACL(path, current.Username); err != nil {
		t.Fatalf("ApplyLogACL(%q): %v", current.Username, err)
	}
	if err := applier.ValidateLogACL(path, current.Username); err != nil {
		t.Fatalf("ValidateLogACL(%q): %v", current.Username, err)
	}
}
