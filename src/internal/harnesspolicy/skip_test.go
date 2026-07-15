package harnesspolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositorySkipInventory(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "test-harness", "skips.json"))
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := ParseSkipInventory(data)
	if err != nil {
		t.Fatal(err)
	}
	uses := make([]SkipUse, 0)
	err = filepath.WalkDir(filepath.Join(root, "src"), func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), "zzlintstaticenforcement") ||
				entry.Name() == "node_modules" || entry.Name() == "coverage" || entry.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(filePath, "_test.go") && !isTypeScriptTest(filePath) {
			return nil
		}
		source, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return readErr
		}
		relative, relErr := filepath.Rel(root, filePath)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		if strings.HasSuffix(filePath, "_test.go") {
			found, scanErr := ScanGoSkips(relative, source)
			if scanErr != nil {
				return scanErr
			}
			uses = append(uses, found...)
		} else {
			uses = append(uses, ScanTypeScriptSkips(relative, source)...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSkips(inventory, uses, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestScanGoSkipsUsesAST(t *testing.T) {
	uses, err := ScanGoSkips("x_test.go", []byte("package x\n// t.Skip(\"comment\")\nfunc TestX(t *testing.T) { t.Skipf(\"reason: %s\", \"x\") }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(uses) != 1 || uses[0].Kind != "go.Skipf" || uses[0].Line != 3 {
		t.Fatalf("unexpected uses: %#v", uses)
	}
}

func TestValidateSkipsRejectsUnregisteredExpiredAndMissingMetadata(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	inventory := SkipInventory{Version: 1, Entries: []SkipEntry{
		{Path: "a_test.go", Kind: "go.Skip", Match: "known", Occurrences: 1, Reason: "", Owner: "team", Expires: "2026-07-10", Evidence: []string{"issue"}},
	}}
	uses := []SkipUse{{Path: "a_test.go", Kind: "go.Skip", Text: `t.Skip("known")`, Line: 4}, {Path: "b.test.ts", Kind: "ts.skip", Text: `test.skip("new")`, Line: 2}}
	err := ValidateSkips(inventory, uses, now)
	for _, want := range []string{"missing required metadata", "expired", "unregistered"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestValidateSkipsAcceptsExactInventory(t *testing.T) {
	inventory := SkipInventory{Version: 1, Entries: []SkipEntry{{
		Path: "a.test.ts", Kind: "ts.skip", Match: `test.skip("tracked"`, Occurrences: 1,
		Reason: "external service", Owner: "team", Expires: "2027-01-01", Evidence: []string{"ADR-1"},
	}}}
	uses := ScanTypeScriptSkips("a.test.ts", []byte(`test.skip("tracked", () => {})`))
	if err := ValidateSkips(inventory, uses, time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}
