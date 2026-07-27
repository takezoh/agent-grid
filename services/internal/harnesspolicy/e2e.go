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
	ID           string `json:"id"`
	Package      string `json:"package"`
	DependencyID string `json:"dependency_id"`
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
		if seenPackage[suite.Package] {
			return registry, fmt.Errorf("duplicate e2e package %q", suite.Package)
		}
		seenID[suite.ID] = true
		seenPackage[suite.Package] = true
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
