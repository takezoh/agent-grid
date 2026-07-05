package gorules_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintRejectsRealBinaryAndE2EEnvOutsideAllowedScopes(t *testing.T) {
	srcRoot := repoSrcRoot(t)
	pkgDir := filepath.Join(srcRoot, "zzlintstaticenforcement")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir temp package: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(pkgDir) })

	const src = `package zzlintstaticenforcement

import (
	"os"
	"os/exec"
)

func violate() {
	_ = os.Getenv("REACTOR_E2E_CODEX_BIN")
	_ = exec.Command("docker", "ps")
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "bad.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write temp package: %v", err)
	}

	cmd := exec.Command("go", "tool", "golangci-lint", "run", "--allow-parallel-runners", "./zzlintstaticenforcement")
	cmd.Dir = srcRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("golangci-lint unexpectedly passed:\n%s", out)
	}
	text := string(out)
	if !strings.Contains(text, "real binary exec is restricted") {
		t.Fatalf("expected real-binary enforcement failure, got:\n%s", text)
	}
	if !strings.Contains(text, "REACTOR_E2E_* env access is restricted") {
		t.Fatalf("expected REACTOR_E2E env enforcement failure, got:\n%s", text)
	}
}

func TestCheckE2ESiblingsScript(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "check-e2e-siblings.sh")

	t.Run("passes with always-on sibling", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "pkg", "suite_e2e_test.go"), "//go:build e2e\n\npackage pkg\n")
		writeFile(t, filepath.Join(root, "pkg", "suite_test.go"), "package pkg\n")

		cmd := exec.Command(script, root)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("script failed unexpectedly: %v\n%s", err, out)
		}
	})

	t.Run("ignores e2e helper packages without e2e tests", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "pkg", "e2e_support.go"), "//go:build e2e\n\npackage pkg\n")

		cmd := exec.Command(script, root)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("script failed unexpectedly: %v\n%s", err, out)
		}
	})

	t.Run("fails without always-on sibling", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "pkg", "suite_e2e_test.go"), "//go:build e2e\n\npackage pkg\n")

		cmd := exec.Command(script, root)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("script unexpectedly passed:\n%s", out)
		}
		if !strings.Contains(string(out), "missing always-on sibling test in package: pkg") {
			t.Fatalf("unexpected script output:\n%s", out)
		}
	})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "Makefile")); err != nil {
		t.Fatalf("repo root not found from %s: %v", wd, err)
	}
	return root
}

func repoSrcRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "src")
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
