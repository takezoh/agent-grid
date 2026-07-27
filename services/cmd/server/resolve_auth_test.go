package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveAuth precedence matrix (highest first):
//
//	-no-auth  > -token  > -token-file
//
// Mutually exclusive: -token AND -token-file is an explicit error so that a
// stale literal or a typo'd path is never silently masked by the other.

func TestResolveAuth_NoAuthZeroesToken(t *testing.T) {
	// Using only -token (not -token-file) so the mutex check stays out of
	// the way; what we're asserting here is that -no-auth wins over an
	// explicit token literal.
	got, err := resolveAuth("ignored", "", true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "" {
		t.Errorf("token = %q, want empty (no-auth)", got)
	}
}

func TestResolveAuth_NoAuthAllowsNonLoopback(t *testing.T) {
	got, err := resolveAuth("", "", true)
	if err != nil {
		t.Fatalf("unexpected err on non-loopback bind with -no-auth: %v", err)
	}
	if got != "" {
		t.Errorf("token = %q, want empty (no-auth still zeroes the token)", got)
	}
}

func TestResolveAuth_TokenFlagWins(t *testing.T) {
	got, err := resolveAuth("literal", "", false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "literal" {
		t.Errorf("token = %q, want %q", got, "literal")
	}
}

func TestResolveAuth_TokenAndTokenFileConflict(t *testing.T) {
	_, err := resolveAuth("literal", "/some/path", false)
	if err == nil {
		t.Fatal("expected error when both -token and -token-file are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want it to mention 'mutually exclusive'", err.Error())
	}
}

// TestResolveAuth_ConflictSurfacesUnderNoAuth ensures the -token + -token-file
// conflict is reported even when -no-auth would zero the token anyway.
// Otherwise an operator who later drops -no-auth from the unit (e.g. when
// adding TLS) hits a fresh "mutually exclusive" error at the moment auth is
// being enabled — exactly the wrong time to learn about a misconfig.
func TestResolveAuth_ConflictSurfacesUnderNoAuth(t *testing.T) {
	_, err := resolveAuth("literal", "/some/path", true)
	if err == nil {
		t.Fatal("expected mutual-exclusion error even with -no-auth")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want mutually exclusive", err.Error())
	}
}

func TestResolveAuth_RequiresExplicitMode(t *testing.T) {
	if _, err := resolveAuth("", "", false); err == nil {
		t.Fatal("missing -token, -token-file, and -no-auth must fail")
	}
}

func TestTokenFromFile_MissingFailsWithoutCreating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	if _, err := tokenFromFile(path); err == nil {
		t.Fatal("missing token file must fail")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing token file was created or stat failed unexpectedly: %v", err)
	}
}

func TestTokenFromFile_EmptyFailsWithoutReplacing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	const empty = "   \n"
	if err := os.WriteFile(path, []byte(empty), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tokenFromFile(path); err == nil {
		t.Fatal("empty token file must fail")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != empty {
		t.Fatalf("empty token file was modified: %q", data)
	}
}
func TestTokenFromFile_ExistingTokenIsTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	if err := os.WriteFile(path, []byte("  preset-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "preset-token" {
		t.Errorf("token = %q, want %q (whitespace trimmed)", got, "preset-token")
	}
}

// TestTokenFromFile_TightensExistingLoosePermissions guards against the
// os.WriteFile behavior that *inherits* a pre-existing file's mode. If an
// operator (or a buggy older boot) wrote the token file at 0644, reading
// the same token on next boot must tighten it to 0600 — otherwise the
// secret stays world-readable forever.
func TestTokenFromFile_TightensExistingLoosePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	if err := os.WriteFile(path, []byte("preset-token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force the loose mode explicitly: os.WriteFile honors umask, so on a
	// hardened host (umask 0077) the file would already be 0600 and the
	// post-assertion would silently false-pass even if tokenFromFile never
	// tightened anything.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "preset-token" {
		t.Errorf("token = %q, want preset-token", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode after read = %o, want 0600 (loose perms must be tightened)", mode)
	}
}

func TestTokenFromFile_ReadErrorIsSurfaced(t *testing.T) {
	dir := t.TempDir()
	// Create a directory at the path so os.ReadFile returns a non-NotExist error.
	path := filepath.Join(dir, "server.token")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := tokenFromFile(path)
	if err == nil {
		t.Fatal("expected error when token path is a directory")
	}
	// Make sure we did not silently fall through to "generate" — that would
	// have tried to write a file at the directory path and failed with a
	// different error. We want the read error surfaced.
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error path = NotExist, want a read error: %v", err)
	}
}
