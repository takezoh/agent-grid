package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const dependencyPolicyADR = "docs/adr/adr-20260715-test-harness-dependency-strategies.md"

type DependencyRegistry struct {
	Version      int          `json:"version"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	ID        string     `json:"id"`
	Strategy  string     `json:"strategy"`
	Seam      Evidence   `json:"seam"`
	Fake      Evidence   `json:"fake"`
	Contract  Contract   `json:"contract"`
	Fidelity  Fidelity   `json:"fidelity"`
	Exception *Exception `json:"exception,omitempty"`
}

type Evidence struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol"`
}

type Contract struct {
	Path      string `json:"path"`
	Test      string `json:"test"`
	Invariant string `json:"invariant"`
	Assertion string `json:"assertion"`
}

type Fidelity struct {
	Path        string `json:"path"`
	Test        string `json:"test"`
	SharedInput string `json:"shared_input"`
	RealMarker  string `json:"real_marker"`
	SuiteID     string `json:"suite_id"`
}

type Exception struct {
	Kind     string `json:"kind"`
	Reason   string `json:"reason"`
	Owner    string `json:"owner"`
	Expiry   string `json:"expiry"`
	Evidence string `json:"evidence"`
}

func ParseDependencyRegistry(data []byte) (DependencyRegistry, error) {
	var registry DependencyRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return registry, fmt.Errorf("parse dependency registry: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return registry, fmt.Errorf("parse dependency registry: trailing JSON value")
		}
		return registry, fmt.Errorf("parse dependency registry: %w", err)
	}
	return registry, nil
}

func ValidateDependencyRegistry(registry DependencyRegistry) []error {
	var errs []error
	if registry.Version != 2 {
		errs = append(errs, fmt.Errorf("registry: version must be 2"))
	}
	if len(registry.Dependencies) == 0 {
		errs = append(errs, fmt.Errorf("registry: dependencies must not be empty"))
	}
	seen := make(map[string]bool)
	for _, dependency := range registry.Dependencies {
		id := dependency.ID
		if id == "" {
			id = "<empty>"
			errs = append(errs, violation(id, "dependency ID is required"))
		}
		if seen[id] {
			errs = append(errs, violation(id, "duplicate dependency ID"))
		}
		seen[id] = true
		switch dependency.Strategy {
		case "triple":
			if dependency.Exception != nil {
				errs = append(errs, violation(id, "triple must not define exception"))
			}
			errs = append(errs, validateTriple(dependency)...)
		case "exception":
			if dependency.Exception == nil {
				errs = append(errs, violation(id, "exception strategy requires exception metadata"))
			} else {
				errs = append(errs, validateException(dependency)...)
			}
		default:
			errs = append(errs, violation(id, "strategy must be triple or exception"))
		}
	}
	sortErrors(errs)
	return errs
}

func validateTriple(dependency Dependency) []error {
	id := dependency.ID
	var errs []error
	errs = append(errs, requireEvidence(id, "seam", dependency.Seam)...)
	errs = append(errs, requireEvidence(id, "fake", dependency.Fake)...)
	if !isPublicSymbol(dependency.Fake.Symbol) {
		errs = append(errs, violation(id, "fake symbol must be public; export it and update fake.symbol"))
	}
	for field, value := range map[string]string{
		"contract.path": dependency.Contract.Path, "contract.test": dependency.Contract.Test,
		"contract.invariant": dependency.Contract.Invariant, "contract.assertion": dependency.Contract.Assertion,
		"fidelity.path": dependency.Fidelity.Path, "fidelity.test": dependency.Fidelity.Test,
		"fidelity.shared_input": dependency.Fidelity.SharedInput, "fidelity.real_marker": dependency.Fidelity.RealMarker,
		"fidelity.suite_id": dependency.Fidelity.SuiteID,
	} {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, violation(id, "missing "+field+"; register executable triple evidence"))
		}
	}
	if strings.HasSuffix(dependency.Fake.Path, "_test.go") {
		errs = append(errs, violation(id, "fake.path must be production source, not *_test.go"))
	}
	if !strings.HasSuffix(dependency.Contract.Path, "_test.go") || strings.HasSuffix(dependency.Contract.Path, "_e2e_test.go") {
		errs = append(errs, violation(id, "contract.path must be an always-on *_test.go, not an e2e test"))
	}
	if !strings.HasSuffix(dependency.Fidelity.Path, "_e2e_test.go") {
		errs = append(errs, violation(id, "fidelity.path must end in _e2e_test.go"))
	}
	return errs
}

func CheckDependencyRegistry(root string, registry DependencyRegistry, suites E2ESuiteRegistry) []error {
	errs := ValidateDependencyRegistry(registry)
	for _, dependency := range registry.Dependencies {
		if dependency.Strategy == "exception" {
			errs = append(errs, checkException(root, dependency)...)
			continue
		}
		if dependency.Strategy != "triple" {
			continue
		}
		errs = append(errs, checkGoDeclaration(root, dependency.ID, "seam.symbol", dependency.Seam)...)
		errs = append(errs, checkGoDeclaration(root, dependency.ID, "fake.symbol", dependency.Fake)...)
		errs = append(errs, checkGoTest(root, dependency.ID, "contract.test", dependency.Contract.Path, dependency.Contract.Test, false)...)
		errs = append(errs, checkGoTest(root, dependency.ID, "fidelity.test", dependency.Fidelity.Path, dependency.Fidelity.Test, true)...)
		checks := []struct{ path, field, marker string }{
			{dependency.Contract.Path, "contract.assertion", dependency.Contract.Assertion},
			{dependency.Fidelity.Path, "fidelity.shared_input", dependency.Fidelity.SharedInput},
			{dependency.Fidelity.Path, "fidelity.real_marker", dependency.Fidelity.RealMarker},
		}
		for _, check := range checks {
			if check.path != "" && check.marker != "" {
				errs = append(errs, checkMarker(root, dependency.ID, check.path, check.field, check.marker)...)
			}
		}
		errs = append(errs, checkSuiteLink(root, dependency, suites)...)
	}
	sortErrors(errs)
	return errs
}

func validateException(dependency Dependency) []error {
	exception := dependency.Exception
	var errs []error
	if exception.Kind != "hermetic-real" && exception.Kind != "trusted-runtime" {
		errs = append(errs, violation(dependency.ID, "exception kind must be hermetic-real or trusted-runtime"))
	}
	for field, value := range map[string]string{"reason": exception.Reason, "owner": exception.Owner, "expiry": exception.Expiry, "evidence": exception.Evidence} {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, violation(dependency.ID, "exception missing "+field))
		}
	}
	if _, err := time.Parse(time.DateOnly, exception.Expiry); exception.Expiry != "" && err != nil {
		errs = append(errs, violation(dependency.ID, "exception expiry must use YYYY-MM-DD"))
	}
	return errs
}

func checkException(root string, dependency Dependency) []error {
	exception := dependency.Exception
	if exception == nil {
		return nil
	}
	var errs []error
	if expiry, err := time.Parse(time.DateOnly, exception.Expiry); err == nil && expiry.Before(time.Now().UTC().Truncate(24*time.Hour)) {
		errs = append(errs, violation(dependency.ID, "exception expired on "+exception.Expiry))
	}
	if exception.Evidence != "" {
		data, err := readRepositoryFile(root, exception.Evidence)
		if err != nil {
			errs = append(errs, violation(dependency.ID, "exception evidence is unreadable: "+err.Error()))
		} else if !bytes.Contains(data, []byte("kind: adr")) {
			errs = append(errs, violation(dependency.ID, "exception evidence must reference a structured ADR"))
		}
	}
	return errs
}

func checkGoDeclaration(root, id, field string, evidence Evidence) []error {
	if evidence.Path == "" || evidence.Symbol == "" {
		return nil
	}
	file, err := parseGoFile(root, evidence.Path)
	if err != nil {
		return []error{violation(id, field+" cannot be parsed: "+err.Error())}
	}
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			if node.Name.Name == evidence.Symbol && node.Recv == nil && ast.IsExported(node.Name.Name) {
				return nil
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch named := spec.(type) {
				case *ast.TypeSpec:
					if named.Name.Name == evidence.Symbol && ast.IsExported(named.Name.Name) {
						return nil
					}
				case *ast.ValueSpec:
					for _, name := range named.Names {
						if name.Name == evidence.Symbol && ast.IsExported(name.Name) {
							return nil
						}
					}
				}
			}
		}
	}
	return []error{violation(id, field+" exported declaration "+fmt.Sprintf("%q", evidence.Symbol)+" not found in "+evidence.Path)}
}

func checkGoTest(root, id, field, path, name string, e2e bool) []error {
	if path == "" || name == "" {
		return nil
	}
	file, err := parseGoFile(root, path)
	if err != nil {
		return []error{violation(id, field+" cannot be parsed: "+err.Error())}
	}
	if e2e {
		data, readErr := readRepositoryFile(root, path)
		if readErr == nil && !bytes.Contains(data, []byte("//go:build e2e")) {
			return []error{violation(id, "fidelity.path must declare //go:build e2e")}
		}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
			continue
		}
		param := fn.Type.Params.List[0].Type
		selector, ok := param.(*ast.StarExpr)
		if !ok {
			continue
		}
		typeName, ok := selector.X.(*ast.SelectorExpr)
		if ok && typeName.Sel.Name == "T" {
			return nil
		}
	}
	return []error{violation(id, field+" executable func "+name+"(*testing.T) not found in "+path)}
}

func checkSuiteLink(root string, dependency Dependency, suites E2ESuiteRegistry) []error {
	suite, ok := suites.Find(dependency.Fidelity.SuiteID)
	if !ok {
		return []error{violation(dependency.ID, "fidelity.suite_id is not registered in e2e-suites.json")}
	}
	if suite.DependencyID != dependency.ID {
		return []error{violation(dependency.ID, "fidelity suite dependency_id mismatch")}
	}
	modulePath := strings.TrimSuffix(strings.TrimPrefix(suite.Package, "./"), "/...")
	if suite.Package == "./..." {
		modulePath = ""
	}
	fidelityPath := strings.TrimPrefix(filepath.ToSlash(dependency.Fidelity.Path), "src/")
	if modulePath != "" && fidelityPath != modulePath && !strings.HasPrefix(fidelityPath, modulePath+"/") {
		return []error{violation(dependency.ID, "fidelity.path is outside suite package "+suite.Package)}
	}
	if _, err := readRepositoryFile(root, dependency.Fidelity.Path); err != nil {
		return []error{violation(dependency.ID, err.Error())}
	}
	return nil
}

func parseGoFile(root, path string) (*ast.File, error) {
	data, err := readRepositoryFile(root, path)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(token.NewFileSet(), path, data, parser.ParseComments)
}

func readRepositoryFile(root, path string) ([]byte, error) {
	if filepath.IsAbs(path) || strings.Contains(filepath.ToSlash(path), "../") {
		return nil, fmt.Errorf("path must stay inside repository: %s", path)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return nil, fmt.Errorf("path %s is unreadable: %w", path, err)
	}
	return data, nil
}

func requireEvidence(id, name string, evidence Evidence) []error {
	var errs []error
	if strings.TrimSpace(evidence.Path) == "" {
		errs = append(errs, violation(id, "missing "+name+".path"))
	}
	if strings.TrimSpace(evidence.Symbol) == "" {
		errs = append(errs, violation(id, "missing "+name+".symbol"))
	}
	return errs
}

func isPublicSymbol(symbol string) bool {
	first, _ := utf8FirstRune(strings.TrimSpace(symbol))
	return unicode.IsUpper(first)
}

func utf8FirstRune(value string) (rune, int) {
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func checkMarker(root, id, path, field, marker string) []error {
	data, err := readRepositoryFile(root, path)
	if err != nil {
		return []error{violation(id, field+" "+err.Error())}
	}
	if !bytes.Contains(data, []byte(marker)) {
		return []error{violation(id, field+" marker "+fmt.Sprintf("%q", marker)+" not found in "+path)}
	}
	return nil
}

func sortErrors(errs []error) {
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
}

func violation(id, detail string) error {
	return fmt.Errorf("dependency %s: %s (see %s)", id, detail, dependencyPolicyADR)
}
