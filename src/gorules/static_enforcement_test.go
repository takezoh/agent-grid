package gorules_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintRejectsRealBinaryAndE2EEnvOutsideAllowedScopes(t *testing.T) {
	srcRoot := repoSrcRoot(t)
	pkgDir, err := os.MkdirTemp(filepath.Join(srcRoot, "gorules"), "zzlintstaticenforcement")
	if err != nil {
		t.Fatalf("mkdir temp package: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(pkgDir) })

	const src = `package zzlintstaticenforcement

import (
	"os"
	"os/exec"
)

func violate() {
	const dockerBin = "docker"
	_ = os.Getenv("AG_E2E_CODEX_BIN")
	_ = exec.Command(dockerBin, "ps")
	_, _ = exec.LookPath(dockerBin)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "bad.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write temp package: %v", err)
	}

	relPkgDir, err := filepath.Rel(srcRoot, pkgDir)
	if err != nil {
		t.Fatalf("resolve temp package: %v", err)
	}
	cmd := exec.Command("go", "tool", "golangci-lint", "run", "--allow-parallel-runners", "./"+filepath.ToSlash(relPkgDir))
	cmd.Dir = srcRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("golangci-lint unexpectedly passed:\n%s", out)
	}
	text := string(out)
	if !strings.Contains(text, "real binary exec is restricted") {
		t.Fatalf("expected real-binary enforcement failure, got:\n%s", text)
	}
	if !strings.Contains(text, "AG_E2E_* env access is restricted") {
		t.Fatalf("expected AG_E2E env enforcement failure, got:\n%s", text)
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

	t.Run("fails with composite e2e build sibling only", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "pkg", "suite_e2e_test.go"), "//go:build e2e\n\npackage pkg\n")
		writeFile(t, filepath.Join(root, "pkg", "suite_test.go"), "//go:build e2e && linux\n\npackage pkg\n")

		cmd := exec.Command(script, root)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("script unexpectedly passed:\n%s", out)
		}
		if !strings.Contains(string(out), "missing always-on sibling test in package: pkg") {
			t.Fatalf("unexpected script output:\n%s", out)
		}
	})

	t.Run("fails with legacy e2e build sibling only", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "pkg", "suite_e2e_test.go"), "//go:build e2e\n\npackage pkg\n")
		writeFile(t, filepath.Join(root, "pkg", "suite_test.go"), "// +build e2e\n\npackage pkg\n")

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

func TestCheckCoverageScriptDedupsUnknownPackageReport(t *testing.T) {
	scriptPath := filepath.Join(repoRoot(t), "scripts", "check-coverage.sh")
	scriptRaw, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read coverage script: %v", err)
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "scripts", "check-coverage.sh"), string(scriptRaw))
	writeFile(t, filepath.Join(root, "scripts", "coverage-floors.txt"), "github.com/takezoh/agent-grid/pkg/known 75\n")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.Chmod(filepath.Join(root, "scripts", "check-coverage.sh"), 0o755); err != nil {
		t.Fatalf("chmod coverage script: %v", err)
	}

	goBin := filepath.Join(root, "bin", "go")
	writeFile(t, goBin, `#!/usr/bin/env bash
set -euo pipefail

if [[ ${1:-} == "list" ]]; then
	cat <<'EOF'
github.com/takezoh/agent-grid/pkg/known 1 0
github.com/takezoh/agent-grid/pkg/unknown 1 0
EOF
	exit 0
fi

if [[ ${1:-} == "test" ]]; then
	cat <<'EOF'
ok  	github.com/takezoh/agent-grid/pkg/known	0.001s	coverage: 80.0% of statements
ok  	github.com/takezoh/agent-grid/pkg/unknown	0.001s	coverage: 60.0% of statements
EOF
	exit 0
fi

echo "unexpected go invocation: $*" >&2
exit 1
`)
	if err := os.Chmod(goBin, 0o755); err != nil {
		t.Fatalf("chmod fake go: %v", err)
	}

	cmd := exec.Command(filepath.Join(root, "scripts", "check-coverage.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("script unexpectedly passed:\n%s", out)
	}

	text := string(out)
	needle := "UNKNOWN  github.com/takezoh/agent-grid/pkg/unknown"
	if got := strings.Count(text, needle); got != 1 {
		t.Fatalf("expected a single UNKNOWN report for missing floor, got %d:\n%s", got, text)
	}
}

func TestCIWorkflowRunsWebTestsAndDetectsUntrackedWireFixtures(t *testing.T) {
	workflowPath := filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "scripts/run-verification-profile.sh pr core") {
		t.Fatalf("CI does not run the shared PR verification profile")
	}
	profileRaw, err := os.ReadFile(filepath.Join(repoRoot(t), "test-harness", "profiles.json"))
	if err != nil {
		t.Fatalf("read verification profiles: %v", err)
	}
	profileText := string(profileRaw)
	if !strings.Contains(profileText, "npm run test:web") {
		t.Fatalf("PR profile does not run the web test entrypoint")
	}

	pkgPath := filepath.Join(repoRoot(t), "src", "client", "web", "package.json")
	pkgRaw, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(pkgRaw, &pkg); err != nil {
		t.Fatalf("decode package.json: %v", err)
	}
	testWeb := pkg.Scripts["test:web"]
	testUnit := pkg.Scripts["test:unit"]
	testCoverage := pkg.Scripts["test:coverage"]
	if !strings.Contains(testWeb, "npm run test:coverage") {
		t.Fatalf("test:web does not include the coverage-enforced unit entrypoint: %q", testWeb)
	}
	if !strings.Contains(testUnit, "vitest") || !strings.Contains(testUnit, "--run") {
		t.Fatalf("test:unit is not an all-files vitest run: %q", testUnit)
	}
	if !strings.Contains(testCoverage, "vitest") || !strings.Contains(testCoverage, "--coverage") {
		t.Fatalf("test:coverage is not an all-files coverage run: %q", testCoverage)
	}
	if !strings.Contains(profileText, "git status --porcelain -- client/web/src/wire/testdata") {
		t.Fatalf("PR profile does not check untracked wire fixtures")
	}
}

func TestNightlyE2EWorkflowExportsAllRealBinaryEnvVars(t *testing.T) {
	workflowPath := filepath.Join(repoRoot(t), ".github", "workflows", "e2e-nightly.yml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	text := string(raw)

	if !strings.Contains(text, "echo \"ag_e2e_docker_bin=$(command -v docker)\" >> \"$GITHUB_OUTPUT\"") {
		t.Fatalf("nightly workflow does not export ag_e2e_docker_bin in suite selection step")
	}
	if !strings.Contains(text, "AG_E2E_DOCKER_BIN: ${{ steps.suite_env.outputs.ag_e2e_docker_bin }}") {
		t.Fatalf("nightly workflow does not pass AG_E2E_DOCKER_BIN to make test-e2e")
	}
	if !strings.Contains(text, "name: Require full nightly suite coverage") {
		t.Fatalf("nightly workflow still allows skip-green when required suites are unavailable")
	}
	if !strings.Contains(text, "scripts/install-pinned-codex.sh fidelity") {
		t.Fatalf("nightly workflow does not install the fidelity-pinned Codex version")
	}
}

func TestCodexSchemaAndFidelityPinsAreDistinctAndWired(t *testing.T) {
	repo := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(repo, "test-harness", "tool-versions.json"))
	if err != nil {
		t.Fatalf("read tool version registry: %v", err)
	}
	var registry struct {
		Codex struct {
			Schema struct {
				Version string `json:"version"`
			} `json:"schema"`
			Fidelity struct {
				Version string `json:"version"`
			} `json:"fidelity"`
		} `json:"codex"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatalf("parse tool version registry: %v", err)
	}
	if registry.Codex.Schema.Version == "" || registry.Codex.Fidelity.Version == "" {
		t.Fatal("schema and fidelity Codex pins must both be declared")
	}
	if registry.Codex.Schema.Version == registry.Codex.Fidelity.Version {
		t.Fatal("schema and fidelity pins represent different contracts and must not be conflated")
	}
	assertContains := func(path, marker string) {
		t.Helper()
		contents, readErr := os.ReadFile(filepath.Join(repo, path))
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if !strings.Contains(string(contents), marker) {
			t.Fatalf("%s does not reference %q", path, marker)
		}
	}
	assertContains(".github/workflows/ci.yml", "scripts/install-pinned-codex.sh schema")
	assertContains(".github/workflows/e2e-nightly.yml", "scripts/install-pinned-codex.sh fidelity")
	assertContains("src/platform/agent/codexschema/README.md", registry.Codex.Schema.Version)
}

func TestHarnessEnforcementCannotDisappearSilently(t *testing.T) {
	repo := repoRoot(t)
	assertFileContains := func(path string, markers ...string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			t.Fatalf("required harness enforcement file %s: %v", path, err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(raw), marker) {
				t.Fatalf("%s is missing required harness marker %q; see adr-20260711-test-harness-anti-tampering", path, marker)
			}
		}
	}

	assertFileContains("test-harness/protected.json", "required_markers")
	assertFileContains("test-harness/dependencies.json", `"id": "pty"`)
	assertFileContains("test-harness/e2e-suites.json", "grok-cli", "stream-routing")
	assertFileContains(".github/CODEOWNERS", "test-harness/")
	assertFileContains("scripts/check-harness-tampering.sh", "--mode tampering")
	assertFileContains("scripts/run-trusted-harness-gate.sh", "check-harness-tampering.sh")
	assertFileContains("test-harness/profiles.json",
		"scripts/check-harness-dependencies.sh",
		"scripts/check-test-skips.sh",
		"scripts/repeat-changed-tests.sh",
		"scripts/run-trusted-harness-gate.sh",
		"scripts/run-mutation-pilot.sh",
	)
	assertFileContains(".github/workflows/ci.yml", "scripts/run-verification-profile.sh pr core", "scripts/run-verification-profile.sh pr race", "scripts/run-verification-profile.sh pr fuzz")
	assertFileContains(".github/workflows/e2e-nightly.yml", "scripts/run-verification-profile.sh nightly", "nightly-e2e-results.json")
	assertFileContains("test-harness/profiles.json", "scripts/run-nightly-e2e-report.sh")
	assertFileContains("scripts/run-go-e2e.sh", "e2e-suites.json", "go test -json -tags e2e")
	assertFileContains("scripts/run-nightly-e2e-report.sh", "run-go-e2e.sh", `event.Action==="skip"`)
}

func TestGatewayScenarioTestsOnlySkipInShortMode(t *testing.T) {
	repo := repoRoot(t)

	codexScenarioPath := filepath.Join(repo, "src", "server", "web", "mux_scenario_test.go")
	codexRaw, err := os.ReadFile(codexScenarioPath)
	if err != nil {
		t.Fatalf("read %s: %v", codexScenarioPath, err)
	}
	codexText := string(codexRaw)
	if !strings.Contains(codexText, "func TestE2E_GatewayScenarioFakeCodexSurfaceAndSessionState") {
		t.Fatalf("gateway codex scenario test missing from %s", codexScenarioPath)
	}
	shortSkips := strings.Count(codexText, `t.Skip("skipping real-daemon scenario e2e in -short mode")`)
	if shortSkips == 0 {
		t.Fatalf("gateway scenario suite must retain its shared -short skip in %s", codexScenarioPath)
	}
	if strings.Count(codexText, "t.Skip(") != shortSkips {
		t.Fatalf("unexpected additional skip found in %s", codexScenarioPath)
	}

	roostScenarioPath := filepath.Join(repo, "src", "server", "web", "mux_scenario_roost_test.go")
	roostRaw, err := os.ReadFile(roostScenarioPath)
	if err != nil {
		t.Fatalf("read %s: %v", roostScenarioPath, err)
	}
	roostText := string(roostRaw)
	roostShortSkips := strings.Count(roostText, `t.Skip("skipping real-daemon scenario e2e in -short mode")`)
	if roostShortSkips == 0 {
		t.Fatalf("roost gateway scenario must keep its shared -short skip in %s", roostScenarioPath)
	}
	if strings.Count(roostText, "t.Skip(") != roostShortSkips {
		t.Fatalf("unexpected additional skip found in %s", roostScenarioPath)
	}
}

func TestSimplifyWorkflowUsesSupportedVerificationCommands(t *testing.T) {
	workflowPath := filepath.Join(repoRoot(t), ".github", "workflows", "simplify.yml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"make vet",
		"cd src && go test ./...",
		"make build-all",
		"scripts/check-coverage.sh",
		"make lint",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("simplify workflow missing verification command %q", want)
		}
	}
	if strings.Contains(text, "go build -o ../arc .") {
		t.Fatalf("simplify workflow still uses broken src-root go build command")
	}
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
