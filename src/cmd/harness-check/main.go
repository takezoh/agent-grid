package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-grid/internal/harnesspolicy"
)

func main() {
	mode := flag.String("mode", "dependencies", "check mode: dependencies or tampering")
	registryPath := flag.String("registry", "test-harness/dependencies.json", "dependency registry path relative to repository root")
	root := flag.String("root", ".", "repository root")
	baseRoot := flag.String("base-root", "", "trusted merge-base tree root")
	headRoot := flag.String("head-root", "", "head tree root")
	manifestPath := flag.String("manifest", "test-harness/protected.json", "protected manifest path relative to base root")
	requestPath := flag.String("request", "", "optional escalation request JSON")
	outputPath := flag.String("output", "", "optional result artifact path")
	flag.Parse()
	if *mode == "tampering" {
		runTampering(*baseRoot, *headRoot, *manifestPath, *requestPath, *outputPath)
		return
	}
	if *mode != "dependencies" {
		fmt.Fprintf(os.Stderr, "harness-check: unknown mode %q\n", *mode)
		os.Exit(2)
	}

	data, err := os.ReadFile(filepath.Join(*root, filepath.FromSlash(*registryPath)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-check: read registry: %v\n", err)
		os.Exit(2)
	}
	registry, err := harnesspolicy.ParseDependencyRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness-check: %v\n", err)
		os.Exit(2)
	}
	errs := harnesspolicy.CheckDependencyRegistry(*root, registry)
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err)
	}
	if len(errs) != 0 {
		os.Exit(1)
	}
	fmt.Printf("harness-check: %d external dependencies admitted\n", len(registry.Dependencies))
}

func runTampering(baseRoot, headRoot, manifestPath, requestPath, outputPath string) {
	if baseRoot == "" || headRoot == "" {
		fmt.Fprintln(os.Stderr, "harness-check: tampering mode requires --base-root and --head-root")
		os.Exit(2)
	}
	data, err := os.ReadFile(filepath.Join(baseRoot, filepath.FromSlash(manifestPath)))
	if err != nil {
		fatalTampering(err)
	}
	manifest, err := harnesspolicy.ParseProtectedManifest(data)
	if err != nil {
		fatalTampering(err)
	}
	base, err := harnesspolicy.ReadProtectedTree(baseRoot, manifest)
	if err != nil {
		fatalTampering(err)
	}
	head, err := harnesspolicy.ReadProtectedTree(headRoot, manifest)
	if err != nil {
		fatalTampering(err)
	}
	var request *harnesspolicy.EscalationRequest
	if requestPath != "" {
		requestData, readErr := os.ReadFile(requestPath)
		if readErr != nil {
			fatalTampering(readErr)
		}
		parsed, parseErr := harnesspolicy.ParseEscalationRequest(requestData)
		if parseErr != nil {
			fatalTampering(parseErr)
		}
		request = &parsed
	}
	result := harnesspolicy.ClassifyProtectedChanges(manifest, base, head, request)
	artifact, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fatalTampering(err)
	}
	artifact = append(artifact, '\n')
	if outputPath != "" {
		if err := os.WriteFile(outputPath, artifact, 0o644); err != nil {
			fatalTampering(err)
		}
	} else {
		_, _ = os.Stdout.Write(artifact)
	}
	if result.Status != "pass" {
		os.Exit(1)
	}
}

func fatalTampering(err error) {
	fmt.Fprintf(os.Stderr, "harness-check: tampering: %v\n", err)
	os.Exit(2)
}
