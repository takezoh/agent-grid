package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExternalBoundaryRegistry is the fail-closed inventory of production callsites.
type ExternalBoundaryRegistry struct {
	Version int                     `json:"version"`
	Entries []ExternalBoundaryEntry `json:"entries"`
}

type ExternalBoundaryEntry struct {
	Path         string   `json:"path"`
	Kinds        []string `json:"kinds"`
	Dependencies []string `json:"dependencies"`
}

func ParseExternalBoundaryRegistry(data []byte) (ExternalBoundaryRegistry, error) {
	var registry ExternalBoundaryRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return registry, fmt.Errorf("parse external boundary registry: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return registry, fmt.Errorf("parse external boundary registry: trailing JSON value")
	}
	if registry.Version != 1 {
		return registry, fmt.Errorf("external boundary registry version must be 1")
	}
	return registry, nil
}

func CheckExternalBoundaries(root string, registry ExternalBoundaryRegistry, dependencies DependencyRegistry) []error {
	knownDependencies := make(map[string]bool)
	for _, dependency := range dependencies.Dependencies {
		knownDependencies[dependency.ID] = true
	}
	expected := make(map[string][]string)
	var errs []error
	for _, entry := range registry.Entries {
		if entry.Path == "" || len(entry.Kinds) == 0 || len(entry.Dependencies) == 0 {
			errs = append(errs, fmt.Errorf("external boundary entry requires path, kinds, and dependencies: %q", entry.Path))
			continue
		}
		if _, exists := expected[entry.Path]; exists {
			errs = append(errs, fmt.Errorf("duplicate external boundary path %s", entry.Path))
		}
		expected[entry.Path] = sortedUnique(entry.Kinds)
		for _, id := range entry.Dependencies {
			if !knownDependencies[id] {
				errs = append(errs, fmt.Errorf("external boundary %s references unknown dependency %s", entry.Path, id))
			}
		}
	}
	discovered, scanErrs := scanExternalBoundaries(root)
	errs = append(errs, scanErrs...)
	for path, kinds := range discovered {
		want, ok := expected[path]
		if !ok {
			errs = append(errs, fmt.Errorf("unregistered external boundary %s (%s)", path, strings.Join(kinds, ",")))
			continue
		}
		if strings.Join(want, ",") != strings.Join(kinds, ",") {
			errs = append(errs, fmt.Errorf("external boundary %s kinds = %v, inventory = %v", path, kinds, want))
		}
		delete(expected, path)
	}
	for path := range expected {
		errs = append(errs, fmt.Errorf("stale external boundary inventory entry %s", path))
	}
	sortErrors(errs)
	return errs
}

func scanExternalBoundaries(root string) (map[string][]string, []error) {
	result := make(map[string][]string)
	var errs []error
	srcRoot := filepath.Join(root, "src")
	err := filepath.WalkDir(srcRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "src/gorules/") || strings.Contains(rel, "/testsupport/") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if parseErr != nil {
			return parseErr
		}
		aliases := importAliases(file)
		kinds := make(map[string]bool)
		if aliases["os/exec"] != "" {
			kinds["process"] = true
		}
		if aliases["github.com/creack/pty"] != "" {
			kinds["pty"] = true
		}
		httpAlias := aliases["net/http"]
		if httpAlias != "" && usesOutboundHTTP(file, httpAlias) {
			kinds["outbound-http"] = true
		}
		if len(kinds) != 0 {
			for kind := range kinds {
				result[rel] = append(result[rel], kind)
			}
			result[rel] = sortedUnique(result[rel])
		}
		return nil
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("scan external boundaries: %w", err))
	}
	return result, errs
}

func importAliases(file *ast.File) map[string]string {
	aliases := make(map[string]string)
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, "\"")
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[path] = name
	}
	return aliases
}

func usesOutboundHTTP(file *ast.File, alias string) bool {
	found := false
	outbound := map[string]bool{"NewRequest": true, "NewRequestWithContext": true, "Get": true, "Post": true, "PostForm": true, "Client": true, "DefaultClient": true}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == alias && outbound[selector.Sel.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool)
	for _, value := range values {
		seen[value] = true
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
