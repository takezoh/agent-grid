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
//	-no-auth  > -token  > -token-file  > random
//
// Mutually exclusive: -token AND -token-file is an explicit error so that a
// stale literal or a typo'd path is never silently masked by the other.

func TestResolveAuth_NoAuthZeroesToken(t *testing.T) {
	// Using only -token (not -token-file) so the mutex check stays out of
	// the way; what we're asserting here is that -no-auth wins over an
	// explicit token literal.
	got, err := resolveAuth("ignored", "", true, false, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "" {
		t.Errorf("token = %q, want empty (no-auth)", got)
	}
}

func TestResolveAuth_NoAuthRejectsNonLoopback(t *testing.T) {
	if _, err := resolveAuth("", "", true, false, "0.0.0.0:8443"); err == nil {
		t.Fatal("expected error on non-loopback bind with -no-auth")
	}
}

// TestResolveAuth_NoAuthAllowsNonLoopbackWithOptIn documents the explicit
// escape hatch: -allow-non-loopback-no-auth suppresses the loopback guard
// so an operator can (dangerously) expose the unauthenticated surface on
// an isolated dev network. Without the opt-in the guard MUST still fire —
// that is the invariant TestResolveAuth_NoAuthRejectsNonLoopback pins.
func TestResolveAuth_NoAuthAllowsNonLoopbackWithOptIn(t *testing.T) {
	got, err := resolveAuth("", "", true, true, "0.0.0.0:8443")
	if err != nil {
		t.Fatalf("unexpected err with opt-in: %v", err)
	}
	if got != "" {
		t.Errorf("token = %q, want empty (no-auth still zeroes the token)", got)
	}
}

// TestResolveAuth_AllowNonLoopbackWithoutNoAuthIsNoop guards against the
// opt-in flag accidentally becoming an implicit -no-auth. Without -no-auth
// it must have zero effect: token resolution follows the normal precedence.
func TestResolveAuth_AllowNonLoopbackWithoutNoAuthIsNoop(t *testing.T) {
	got, err := resolveAuth("literal", "", false, true, "0.0.0.0:8443")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "literal" {
		t.Errorf("token = %q, want %q (opt-in must not bypass token resolution)", got, "literal")
	}
}

func TestResolveAuth_TokenFlagWins(t *testing.T) {
	got, err := resolveAuth("literal", "", false, false, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "literal" {
		t.Errorf("token = %q, want %q", got, "literal")
	}
}

func TestResolveAuth_TokenAndTokenFileConflict(t *testing.T) {
	_, err := resolveAuth("literal", "/some/path", false, false, "127.0.0.1:0")
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
	_, err := resolveAuth("literal", "/some/path", true, false, "127.0.0.1:0")
	if err == nil {
		t.Fatal("expected mutual-exclusion error even with -no-auth")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want mutually exclusive", err.Error())
	}
}

func TestResolveAuth_RandomWhenBothEmpty(t *testing.T) {
	got, err := resolveAuth("", "", false, false, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 48 { // randToken() = 24 bytes hex-encoded
		t.Errorf("random token length = %d, want 48", len(got))
	}
}

func TestTokenFromFile_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "server.token")

	first, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first == "" {
		t.Fatal("first call returned empty token")
	}

	// File must exist with mode 0600 after first call.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("token file mode = %o, want 0600", got)
	}

	// Parent dir must be 0700.
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := parentInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("token dir mode = %o, want 0700", got)
	}

	// Second call must read the same token, not regenerate.
	second, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first != second {
		t.Errorf("token rotated across calls: first=%q second=%q", first, second)
	}
}

func TestTokenFromFile_EmptyFileRegenerates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if strings.TrimSpace(tok) == "" {
		t.Error("empty file should regenerate, got empty token")
	}

	// File now must hold the new token.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != tok {
		t.Errorf("file content = %q, want %q", strings.TrimSpace(string(data)), tok)
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

// TestTokenFromFile_WriteIsAtomicAtFinalMode regenerates a token into a
// new path and asserts that, post-rename, the final file is at 0600 even
// though the source path was created via CreateTemp (which defaults to
// the umask-influenced mode). The atomic-write path is what survives a
// crash without leaving a half-written zero-byte file.
func TestTokenFromFile_WriteIsAtomicAtFinalMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.token")
	tok, err := tokenFromFile(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("freshly-written token file mode = %o, want 0600", mode)
	}
	// No stray temp file left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir entries = %d (%v), want exactly 1 (the renamed token file)",
			len(entries), entries)
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
