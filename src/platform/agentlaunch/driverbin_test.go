package agentlaunch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDriverBinResolver_ConfiguredOverrideWinsOverPATH pins the invariant
// that an explicit [drivers.<name>] bin config beats PATH lookup — the
// operator's declared binary is authoritative even when the daemon PATH
// happens to hold a same-named tool. Missing this ordering would let a
// PATH-mounted stale binary silently shadow an intentional override.
func TestDriverBinResolver_ConfiguredOverrideWinsOverPATH(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(override, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake: %v", err)
	}
	r := &DriverBinResolver{Overrides: map[string]string{"codex": override}}

	got, err := r.Resolve("codex")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != override {
		t.Errorf("Resolve = %q, want configured override %q", got, override)
	}
}

// TestDriverBinResolver_MissingOverrideReturnsActionableError proves the
// resolver rejects a stale config entry rather than silently degrading to
// PATH lookup — a stale override that pointed at an uninstalled tool would
// otherwise mask itself behind an unrelated PATH-lookup success or failure.
func TestDriverBinResolver_MissingOverrideReturnsActionableError(t *testing.T) {
	r := &DriverBinResolver{Overrides: map[string]string{"codex": "/nonexistent/path/codex"}}

	_, err := r.Resolve("codex")
	if err == nil {
		t.Fatal("Resolve must fail when configured override does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error must name the stale-override condition: %v", err)
	}
	if !strings.Contains(err.Error(), "[drivers.codex] bin") {
		t.Errorf("error must name the config key: %v", err)
	}
}

// TestDriverBinResolver_RelativeOverrideIgnored pins that a relative-path
// override is treated as unset (fall through to LookPath) rather than
// resolved against the daemon's cwd — resolving relative to daemon cwd
// would re-introduce the exact implicit-context defect this resolver exists
// to close.
func TestDriverBinResolver_RelativeOverrideIgnored(t *testing.T) {
	r := &DriverBinResolver{Overrides: map[string]string{"codex": "./relative/codex"}}
	// exec.LookPath("codex") behavior is environment-dependent; assert only
	// that the override was NOT taken as-is (either LookPath finds it and
	// returns an absolute path, or fails with the PATH-lookup message).
	got, err := r.Resolve("codex")
	if err == nil {
		if got == "./relative/codex" {
			t.Errorf("relative override must not be taken as-is, got %q", got)
		}
	}
	if err != nil && strings.Contains(err.Error(), "./relative/codex") {
		t.Errorf("error should not attribute failure to relative override: %v", err)
	}
}

// TestDriverBinResolver_NoOverrideFallsBackToPath verifies the PATH-lookup
// fallback works for a driver whose binary exists on the daemon's PATH.
// Uses `sh` as the probe binary since it is universally present.
func TestDriverBinResolver_NoOverrideFallsBackToPath(t *testing.T) {
	r := &DriverBinResolver{}
	got, err := r.Resolve("sh")
	if err != nil {
		t.Fatalf("Resolve fallback to PATH failed for 'sh' (must exist on any test host): %v", err)
	}
	if !strings.HasPrefix(got, "/") {
		t.Errorf("resolved path must be absolute, got %q", got)
	}
}

// TestDriverBinResolver_UnresolvableReturnsActionableError is the primary
// contract test: when no override AND no PATH match, the resolver must
// return an error naming both the config key AND the PATH-augmentation
// escape hatch, so an operator hitting the daemon-launched-process ENOENT
// class of failure sees the fix path directly instead of grepping the
// codebase.
func TestDriverBinResolver_UnresolvableReturnsActionableError(t *testing.T) {
	r := &DriverBinResolver{}
	_, err := r.Resolve("this-tool-does-not-exist-anywhere-xyzzy")
	if err == nil {
		t.Fatal("Resolve must fail when neither config nor PATH resolves the name")
	}
	for _, want := range []string{
		`"this-tool-does-not-exist-anywhere-xyzzy"`,
		"[drivers.this-tool-does-not-exist-anywhere-xyzzy] bin",
		"PATH",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing required token %q; got: %v", want, err)
		}
	}
}

// TestDriverBinResolver_EmptyNameRejected fails the resolver fast on a
// programmer-error input rather than surfacing a confusing PATH-lookup
// message for an empty string.
func TestDriverBinResolver_EmptyNameRejected(t *testing.T) {
	r := &DriverBinResolver{}
	_, err := r.Resolve("")
	if err == nil {
		t.Fatal("Resolve must reject empty name")
	}
}

// TestDriverBinResolver_NilReceiverIsPurePathLookup keeps the resolver
// zero-value-safe. Callers that have no config source may pass a nil
// resolver; behavior collapses to pure PATH lookup with the same error
// shape.
func TestDriverBinResolver_NilReceiverIsPurePathLookup(t *testing.T) {
	var r *DriverBinResolver
	got, err := r.Resolve("sh")
	if err != nil {
		t.Fatalf("nil resolver PATH fallback failed for 'sh': %v", err)
	}
	if !strings.HasPrefix(got, "/") {
		t.Errorf("resolved path must be absolute, got %q", got)
	}
}
