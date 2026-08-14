package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// E2ESuiteRegistry is the single source of truth for Go T3 packages.
type E2ESuiteRegistry struct {
	Version int        `json:"version"`
	Suites  []E2ESuite `json:"suites"`
}

type E2ESuite struct {
	ID string `json:"id"`
	// Module is the Go module the package is relative to, as a repository-
	// relative directory. It defaults to "services", which is where every
	// suite lived when this repository had one Go module for backend code.
	//
	// It exists because the independent client modules — clients/cli today —
	// have external boundaries of their own, and a boundary whose fidelity
	// backstop could not be registered would have to be admitted as an
	// exception instead. That would make "every external dependency ships the
	// full triple" a rule with a hole exactly where the newest code is.
	Module       string `json:"module,omitempty"`
	Package      string `json:"package"`
	DependencyID string `json:"dependency_id"`
}

// DefaultE2EModule is the module a suite belongs to when it names none.
const DefaultE2EModule = "services"

// ModuleDir is the suite's module as a repository-relative directory.
func (s E2ESuite) ModuleDir() string {
	if strings.TrimSpace(s.Module) == "" {
		return DefaultE2EModule
	}
	return s.Module
}

func ParseE2ESuiteRegistry(data []byte) (E2ESuiteRegistry, error) {
	var registry E2ESuiteRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return registry, fmt.Errorf("parse e2e suite registry: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return registry, fmt.Errorf("parse e2e suite registry: trailing JSON value")
	}
	if registry.Version != 1 || len(registry.Suites) == 0 {
		return registry, fmt.Errorf("e2e suite registry requires version 1 and at least one suite")
	}
	seenID := make(map[string]bool)
	seenPackage := make(map[string]bool)
	for _, suite := range registry.Suites {
		if strings.TrimSpace(suite.ID) == "" || strings.TrimSpace(suite.DependencyID) == "" {
			return registry, fmt.Errorf("e2e suite id and dependency_id are required")
		}
		if seenID[suite.ID] {
			return registry, fmt.Errorf("duplicate e2e suite id %q", suite.ID)
		}
		if !strings.HasPrefix(suite.Package, "./") || !strings.HasSuffix(suite.Package, "/...") {
			return registry, fmt.Errorf("e2e suite %s package must end in /... and stay module-relative", suite.ID)
		}
		qualified := suite.ModuleDir() + " " + suite.Package
		if seenPackage[qualified] {
			return registry, fmt.Errorf("duplicate e2e package %q", qualified)
		}
		seenID[suite.ID] = true
		seenPackage[qualified] = true
	}
	sort.Slice(registry.Suites, func(i, j int) bool { return registry.Suites[i].ID < registry.Suites[j].ID })
	return registry, nil
}

func (r E2ESuiteRegistry) Find(id string) (E2ESuite, bool) {
	for _, suite := range r.Suites {
		if suite.ID == id {
			return suite, true
		}
	}
	return E2ESuite{}, false
}
