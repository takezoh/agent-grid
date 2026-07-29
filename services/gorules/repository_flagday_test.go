package gorules_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Closed consumer inventory for repository exact-version flag day.
// Every surface embeds the same protocol token once (or re-exports the SSOT
// projection) and participates in repository/preparation shape migration.
var repositoryExactVersionConsumers = []string{
	"services/server/api/protocol_version.go",
	"clients/web-host/internal/webhost/host.go",
	"clients/ui/src/protocol-version.ts",
	"clients/workspace/src/main/protocol-version.ts",
	"clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ProtocolVersion.cs",
	"clients/sdk/ts/src/transport.ts",
	"clients/sdk/csharp/Transport/GatewayClient.cs",
	"clients/sdk/swift/Transport/GatewayClient.swift",
	"clients/sdk/kotlin/transport/GatewayClient.kt",
	"protocol/capabilities.schema.json",
	"protocol/simulator/server/main.go",
	"protocol/openapi.yaml",
	"protocol/simulator/fixtures/repository-preparation-round-trip.json",
}

// Consumers that must bind the SSOT constant rather than residual literals.
var repositoryExactVersionConstantUsers = []struct {
	path   string
	needle string
}{
	{"clients/ui/e2e/support/fake-backend.ts", "PROTOCOL_VERSION"},
	{"clients/workspace/src/main/fleet-session-runtime.ts", "PROTOCOL_VERSION"},
	{"clients/ui/src/fleet/provider.ts", "PROTOCOL_VERSION"},
	{"clients/workspace/src/main/repository-lease.ts", "PROTOCOL_VERSION"},
}

// Repository/preparation round-trip surfaces (codec, fixtures, generated models).
var repositoryPreparationRoundTripSurfaces = []string{
	"protocol/repository.schema.json",
	"services/host/proto/repository.go",
	"services/host/proto/repository_codec_test.go",
	"services/server/api/repository_wire.go",
	"services/server/api/repository_mismatch_matrix_test.go",
	"clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ProtocolVersion.cs",
	"clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/WsFrameParser.cs",
	"clients/sdk/quicktype-emit.json",
	"protocol/simulator/fixtures/repository-preparation-round-trip.json",
}

type fidelityTriple struct {
	ID               string
	FakePath         string
	FakeSymbol       string
	ContractPath     string
	ContractTest     string
	FakeVsRealPath   string
	FakeVsRealTest   string
	FixtureAuthority string // env var or explicit reason marker
	SkipEvidence     string // typed unavailable reason substring
}

// Five external dependency triples owned by repository sessions.
var repositoryFidelityTriples = []fidelityTriple{
	{
		ID: "github-auth", FakePath: "clients/web-host/internal/githubhost/githubhost_test.go",
		FakeSymbol: "FakeGitHubAuth", ContractPath: "clients/web-host/internal/githubhost/githubhost_test.go",
		ContractTest: "TestGitHubAuthContract", FakeVsRealPath: "clients/web-host/internal/githubhost/githubhost_test.go",
		FakeVsRealTest: "TestFakeVsRealGitHubAuth", FixtureAuthority: "AG_E2E_GITHUB_OAUTH_CLIENT_ID",
		SkipEvidence: "AG_E2E_GITHUB_OAUTH_CLIENT_ID unset",
	},
	{
		ID: "github-catalog", FakePath: "clients/web-host/internal/githubhost/githubhost_test.go",
		FakeSymbol: "FakeGitHubCatalog", ContractPath: "clients/web-host/internal/githubhost/githubhost_test.go",
		ContractTest: "TestGitHubCatalogContract", FakeVsRealPath: "clients/web-host/internal/githubhost/githubhost_test.go",
		FakeVsRealTest: "TestFakeVsRealGitHubCatalog", FixtureAuthority: "AG_E2E_GITHUB_CATALOG_TOKEN",
		SkipEvidence: "AG_E2E_GITHUB_CATALOG_TOKEN unset",
	},
	{
		ID: "git-clone", FakePath: "services/platform/lib/gitclone/process.go",
		FakeSymbol: "FakeGitClone", ContractPath: "services/platform/lib/gitclone/process_test.go",
		ContractTest: "TestGitCloneContract", FakeVsRealPath: "services/platform/lib/gitclone/process_e2e_test.go",
		FakeVsRealTest: "TestFakeVsRealGitClone", FixtureAuthority: "AG_E2E_GIT_CLONE_PRIVATE_REPO",
		SkipEvidence: "CodeAuthorityUnavailable",
	},
	{
		ID: "filesystem-process", FakePath: "services/platform/lib/gitclone/fs_process.go",
		FakeSymbol: "FakeFilesystemProcess", ContractPath: "services/platform/lib/gitclone/publication_test.go",
		ContractTest: "TestFilesystemProcessContract", FakeVsRealPath: "services/platform/lib/gitclone/publication_e2e_test.go",
		FakeVsRealTest: "TestFakeVsRealFilesystemProcess", FixtureAuthority: "AG_E2E_FILESYSTEM_PROCESS",
		SkipEvidence: "AG_E2E_FILESYSTEM_PROCESS",
	},
	{
		ID: "repository-devcontainer", FakePath: "services/host/runtime/repository_environment.go",
		FakeSymbol: "FakeRepositoryDevcontainer", ContractPath: "services/host/runtime/repository_preparation_test.go",
		ContractTest: "TestRepositoryDevcontainerScopeContract", FakeVsRealPath: "services/host/runtime/repository_devcontainer_e2e_test.go",
		FakeVsRealTest: "TestFakeVsRealRepositoryDevcontainer", FixtureAuthority: "AG_E2E_REPOSITORY_DEVCONTAINER",
		SkipEvidence: "environment_authority_unavailable",
	},
}

func TestRepositoryExactVersionConsumerInventory(t *testing.T) {
	root := repoRoot(t)
	var manifest struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "protocol", "protocol-version.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.ProtocolVersion == "" {
		t.Fatalf("manifest: %v %+v", err, manifest)
	}
	token := manifest.ProtocolVersion
	for _, rel := range repositoryExactVersionConsumers {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if !strings.Contains(string(body), token) {
			t.Errorf("%s must embed exact protocol token %q", rel, token)
		}
	}
	for _, c := range repositoryExactVersionConstantUsers {
		body, err := os.ReadFile(filepath.Join(root, c.path))
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if !strings.Contains(string(body), c.needle) {
			t.Errorf("%s must use %s", c.path, c.needle)
		}
		if strings.Contains(string(body), `"`+token+`"`) {
			t.Errorf("%s must not residual-hardcode token %q", c.path, token)
		}
	}
}

func TestRepositoryPreparationRoundTripInventory(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repositoryPreparationRoundTripSurfaces {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing repository/preparation surface %s: %v", rel, err)
		}
	}
	emitRaw, err := os.ReadFile(filepath.Join(root, "clients/sdk/quicktype-emit.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(emitRaw), "repository.schema.json") {
		t.Error("SDK emit inventory must include repository.schema.json")
	}
	// Generated models (or fixture) exist after flag-day generation.
	for _, rel := range []string{
		"clients/sdk/ts/generated/repository.ts",
		"clients/sdk/csharp/Generated/repository.cs",
		"clients/sdk/kotlin/generated/repository.kt",
		"clients/sdk/swift/Generated/repository.swift",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("generated repository consumer missing %s: %v", rel, err)
		}
	}
}

// TestSessionTargetDiscriminatorCrossSDK freezes oneOf enforcement in every
// shipped SDK model surface (TS runtime guard, C# converter, Kotlin init,
// Swift decode). Collapsed project+repository payloads must be rejected.
//
// Always-on gate:
//  1. generated models still contain discriminator enforcement symbols
//  2. every language has a checked-in runnable harness entrypoint
//  3. TypeScript harness always executes (npm test)
//  4. C#/Kotlin/Swift harnesses execute when their toolchain is present
//
// CI sets AG_SDK_CONFORMANCE_REQUIRE_ALL=1 so all four must run, not skip.
func TestSessionTargetDiscriminatorCrossSDK(t *testing.T) {
	root := repoRoot(t)
	checks := []struct {
		path   string
		needle string
	}{
		{"clients/sdk/ts/generated/repository.ts", "export function isSessionTarget"},
		{"clients/sdk/ts/generated/repository.ts", `v.source === "project"`},
		{"clients/sdk/ts/generated/repository.ts", `v.source === "repository"`},
		{"clients/sdk/csharp/Generated/repository.cs", "SessionTargetJsonConverter"},
		{"clients/sdk/csharp/Generated/repository.cs", "oneOf project|repository"},
		{"clients/sdk/kotlin/generated/repository.kt", "fun isValid(): Boolean"},
		{"clients/sdk/kotlin/generated/repository.kt", `"project"`},
		{"clients/sdk/kotlin/generated/repository.kt", `"repository"`},
		{"clients/sdk/swift/Generated/repository.swift", "public static func isValid"},
		{"clients/sdk/swift/Generated/repository.swift", "oneOf project|repository"},
	}
	for _, c := range checks {
		body, err := os.ReadFile(filepath.Join(root, c.path))
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if !strings.Contains(string(body), c.needle) {
			t.Errorf("%s must enforce SessionTarget discriminator (%q)", c.path, c.needle)
		}
	}
	// Schema SSOT still declares oneOf with source const.
	schema, err := os.ReadFile(filepath.Join(root, "protocol/repository.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(schema)
	if !strings.Contains(s, `"const": "project"`) || !strings.Contains(s, `"const": "repository"`) {
		t.Error("repository.schema.json SessionTarget must keep source const discriminators")
	}

	// Runnable harness inventory — string-scan of models alone is not enough.
	harnesses := []struct {
		lang string
		path string
	}{
		{"ts", "clients/sdk/ts/session_target_discriminator.test.ts"},
		{"csharp", "clients/sdk/csharp/harness/Program.cs"},
		{"kotlin", "clients/sdk/kotlin/harness/SessionTargetConformance.kt"},
		{"swift", "clients/sdk/swift/harness/main.swift"},
	}
	for _, h := range harnesses {
		if _, err := os.Stat(filepath.Join(root, h.path)); err != nil {
			t.Errorf("missing runnable SessionTarget harness for %s at %s: %v", h.lang, h.path, err)
		}
	}
	runner := filepath.Join(root, "clients/sdk/scripts/run-session-target-conformance.sh")
	if _, err := os.Stat(runner); err != nil {
		t.Fatalf("missing SessionTarget conformance runner: %v", err)
	}

	// Executable harness runner:
	//  - TypeScript always runs (npm test → isSessionTarget cases)
	//  - C#/Kotlin/Swift run when toolchains are present
	//  - CI sets AG_SDK_CONFORMANCE_REQUIRE_ALL=1 so all four must execute
	cmd := exec.Command("bash", runner)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("SessionTarget cross-SDK conformance harness failed: %v\n%s", err, out)
	}
	t.Logf("SessionTarget conformance:\n%s", out)
	// Hard-require TypeScript execution evidence even when other languages skip.
	if !strings.Contains(string(out), "OK ts") {
		t.Fatalf("TypeScript SessionTarget harness must always run locally:\n%s", out)
	}
}

func TestExternalDependencyFidelityInventory(t *testing.T) {
	root := repoRoot(t)
	if len(repositoryFidelityTriples) != 5 {
		t.Fatalf("want 5 triples, got %d", len(repositoryFidelityTriples))
	}
	seen := map[string]bool{}
	for _, tri := range repositoryFidelityTriples {
		if seen[tri.ID] {
			t.Fatalf("duplicate triple id %s", tri.ID)
		}
		seen[tri.ID] = true
		for _, check := range []struct {
			path, needle, label string
		}{
			{tri.FakePath, tri.FakeSymbol, "fake"},
			{tri.ContractPath, tri.ContractTest, "contract"},
			{tri.FakeVsRealPath, tri.FakeVsRealTest, "FakeVsReal"},
			{tri.FakeVsRealPath, tri.FixtureAuthority, "fixture authority"},
			{tri.FakeVsRealPath, tri.SkipEvidence, "explicit skip/unavailable evidence"},
		} {
			body, err := os.ReadFile(filepath.Join(root, check.path))
			if err != nil {
				t.Errorf("%s %s path %s: %v", tri.ID, check.label, check.path, err)
				continue
			}
			if !strings.Contains(string(body), check.needle) {
				t.Errorf("%s missing %s marker %q in %s", tri.ID, check.label, check.needle, check.path)
			}
		}
	}
}

func TestRepositoryFlagDayNoNewThirdPartyDependencySurface(t *testing.T) {
	// Flag-day work must not introduce module/package deps; composition stays stdlib/existing.
	root := repoRoot(t)
	for _, rel := range []string{
		"services/go.mod",
		"clients/ui/package.json",
		"clients/sdk/package.json",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		// Sentinel: no accidental brand-new dependency names introduced by this flag day.
		for _, banned := range []string{
			`"github.com/google/go-github"`,
			`"octokit"`,
			`"simple-git"`,
			`"isomorphic-git"`,
		} {
			if strings.Contains(string(body), banned) {
				t.Errorf("%s introduces banned third-party dependency %s", rel, banned)
			}
		}
	}
}

func TestOwnerNameSearchValidationEvidenceRegistered(t *testing.T) {
	root := repoRoot(t)
	// contract-repository-search-validation evidence lives on the fleet UX seam.
	path := filepath.Join(root, "clients/ui/src/store/repository_picker.test.ts")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		"UAC-003 owner/name search validation",
		"evaluateSearchValidation",
		"invalidated",
	} {
		if !strings.Contains(string(body), needle) {
			t.Errorf("search validation evidence missing %q", needle)
		}
	}
}
