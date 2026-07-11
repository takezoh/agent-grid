package harnesspolicy

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"time"
)

type SkipInventory struct {
	Version int         `json:"version"`
	Entries []SkipEntry `json:"entries"`
}

type SkipEntry struct {
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Match       string   `json:"match"`
	Occurrences int      `json:"occurrences"`
	Reason      string   `json:"reason"`
	Owner       string   `json:"owner"`
	Expires     string   `json:"expires"`
	Evidence    []string `json:"evidence"`
}

type SkipUse struct {
	Path string
	Kind string
	Text string
	Line int
}

func ParseSkipInventory(data []byte) (SkipInventory, error) {
	var inventory SkipInventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		return SkipInventory{}, fmt.Errorf("parse skip inventory: %w", err)
	}
	if inventory.Version != 1 {
		return SkipInventory{}, fmt.Errorf("unsupported skip inventory version %d", inventory.Version)
	}
	return inventory, nil
}

func ScanGoSkips(path string, source []byte) ([]SkipUse, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	uses := make([]SkipUse, 0)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Skip" && sel.Sel.Name != "Skipf" && sel.Sel.Name != "SkipNow") {
			return true
		}
		position := fset.Position(call.Pos())
		line := sourceLine(source, position.Line)
		uses = append(uses, SkipUse{Path: path, Kind: "go." + sel.Sel.Name, Text: strings.TrimSpace(line), Line: position.Line})
		return true
	})
	return uses, nil
}

var tsSkipPattern = regexp.MustCompile(`\b(?:describe|it|test)\.(?:skip|todo|fixme)\s*\(`)

func ScanTypeScriptSkips(path string, source []byte) []SkipUse {
	lines := strings.Split(string(source), "\n")
	uses := make([]SkipUse, 0)
	for index, line := range lines {
		matches := tsSkipPattern.FindAllStringIndex(line, -1)
		for range matches {
			uses = append(uses, SkipUse{Path: path, Kind: "ts.skip", Text: strings.TrimSpace(line), Line: index + 1})
		}
	}
	return uses
}

func ValidateSkips(inventory SkipInventory, uses []SkipUse, now time.Time) error {
	errors := make([]string, 0)
	matched := make([]int, len(inventory.Entries))
	for index, entry := range inventory.Entries {
		if entry.Path == "" || entry.Kind == "" || entry.Match == "" || entry.Occurrences < 1 ||
			entry.Reason == "" || entry.Owner == "" || entry.Expires == "" || len(entry.Evidence) == 0 {
			errors = append(errors, fmt.Sprintf("inventory entry %d is missing required metadata", index))
		}
		expires, err := time.Parse("2006-01-02", entry.Expires)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: invalid expiry %q", entry.Path, entry.Expires))
		} else if expires.Before(midnightUTC(now)) {
			errors = append(errors, fmt.Sprintf("%s: skip inventory expired on %s", entry.Path, entry.Expires))
		}
	}
	for _, use := range uses {
		found := -1
		for index, entry := range inventory.Entries {
			if entry.Path == use.Path && entry.Kind == use.Kind && strings.Contains(use.Text, entry.Match) {
				if found != -1 {
					errors = append(errors, fmt.Sprintf("%s:%d: skip matches multiple inventory entries", use.Path, use.Line))
				}
				found = index
			}
		}
		if found == -1 {
			errors = append(errors, fmt.Sprintf("%s:%d: unregistered %s: %s", use.Path, use.Line, use.Kind, use.Text))
		} else {
			matched[found]++
		}
	}
	for index, entry := range inventory.Entries {
		if matched[index] != entry.Occurrences {
			errors = append(errors, fmt.Sprintf("%s: inventory expects %d occurrence(s), found %d for %q", entry.Path, entry.Occurrences, matched[index], entry.Match))
		}
	}
	sort.Strings(errors)
	if len(errors) > 0 {
		return fmt.Errorf("skip policy violations:\n%s", strings.Join(errors, "\n"))
	}
	return nil
}

func sourceLine(source []byte, line int) string {
	lines := strings.Split(string(source), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}

func midnightUTC(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
