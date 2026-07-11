package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const ptyExceptionADR = "docs/adr/adr-20260705-test-tier-taxonomy.md"

type DependencyRegistry struct {
	Version      int          `json:"version"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	ID        string     `json:"id"`
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
}

type Exception struct {
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
	if registry.Version != 1 {
		errs = append(errs, fmt.Errorf("registry: version must be 1"))
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
		if dependency.Exception != nil {
			errs = append(errs, validateException(dependency)...)
			continue
		}
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
		} {
			if strings.TrimSpace(value) == "" {
				errs = append(errs, violation(id, "missing "+field+"; register executable triple evidence"))
			}
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

func CheckDependencyRegistry(root string, registry DependencyRegistry) []error {
	errs := ValidateDependencyRegistry(registry)
	for _, dependency := range registry.Dependencies {
		if dependency.Exception != nil {
			continue
		}
		checks := []struct{ path, field, marker string }{
			{dependency.Seam.Path, "seam.symbol", dependency.Seam.Symbol},
			{dependency.Fake.Path, "fake.symbol", dependency.Fake.Symbol},
			{dependency.Contract.Path, "contract.test", dependency.Contract.Test},
			{dependency.Contract.Path, "contract.assertion", dependency.Contract.Assertion},
			{dependency.Fidelity.Path, "fidelity.test", dependency.Fidelity.Test},
			{dependency.Fidelity.Path, "fidelity.shared_input", dependency.Fidelity.SharedInput},
			{dependency.Fidelity.Path, "fidelity.real_marker", dependency.Fidelity.RealMarker},
		}
		for _, check := range checks {
			if check.path == "" || check.marker == "" {
				continue
			}
			errs = append(errs, checkMarker(root, dependency.ID, check.path, check.field, check.marker)...)
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

func validateException(dependency Dependency) []error {
	if dependency.ID != "pty" {
		return []error{violation(dependency.ID, "only pty may use the grandfathered exception; add fake, contract, and fidelity evidence")}
	}
	exception := dependency.Exception
	var errs []error
	for field, value := range map[string]string{"reason": exception.Reason, "owner": exception.Owner, "expiry": exception.Expiry, "evidence": exception.Evidence} {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, violation(dependency.ID, "exception missing "+field))
		}
	}
	if exception.Evidence != ptyExceptionADR {
		errs = append(errs, violation(dependency.ID, "exception evidence must reference "+ptyExceptionADR))
	}
	return errs
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
	symbol = strings.TrimSpace(symbol)
	if strings.HasPrefix(symbol, "export ") {
		return true
	}
	first, _ := utf8FirstRune(symbol)
	return unicode.IsUpper(first)
}

func utf8FirstRune(value string) (rune, int) {
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func checkMarker(root, id, path, field, marker string) []error {
	if filepath.IsAbs(path) || strings.Contains(filepath.ToSlash(path), "../") {
		return []error{violation(id, field+" path must stay inside repository: "+path)}
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return []error{violation(id, field+" path "+path+" is unreadable: "+err.Error())}
	}
	if !bytes.Contains(data, []byte(marker)) {
		return []error{violation(id, field+" marker "+fmt.Sprintf("%q", marker)+" not found in "+path)}
	}
	return nil
}

func violation(id, detail string) error {
	return fmt.Errorf("dependency %s: %s (see %s)", id, detail, ptyExceptionADR)
}
