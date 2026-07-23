package harnesspolicy

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseNameStatusPreservesRenameAndDeletePaths(t *testing.T) {
	entries, err := ParseNameStatus("M\tsrc/a/a.go\nR100\tsrc/old/x.ts\tsrc/new/x.ts\nD\tsrc/gone/y.go\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := entries[1].Paths; !reflect.DeepEqual(got, []string{"src/old/x.ts", "src/new/x.ts"}) {
		t.Fatalf("rename paths = %v", got)
	}
	if got := entries[2].Paths; !reflect.DeepEqual(got, []string{"src/gone/y.go"}) {
		t.Fatalf("delete paths = %v", got)
	}
}

func TestSelectRepeatTargetsAddModifyRenameDeleteAndSort(t *testing.T) {
	files := []RepositoryFile{
		{Path: "src/a/a_test.go"}, {Path: "src/old/old_test.go"}, {Path: "src/new/new_test.go"},
		{Path: "clients/ui/src/a.test.ts", Content: `import "./a"`},
		{Path: "clients/ui/src/deleted/z-sibling.test.ts", Content: `import "./renamed"`},
	}
	entries := []DiffEntry{
		{Status: "M", Paths: []string{"src/a/a.go"}},
		{Status: "D", Paths: []string{"src/old/old.go"}},
		{Status: "R100", Paths: []string{"src/new/before.go", "src/new/after.go"}},
		{Status: "M", Paths: []string{"clients/ui/src/a.ts"}},
		{Status: "D", Paths: []string{"clients/ui/src/deleted/z.test.ts"}},
	}
	selection := SelectRepeatTargets(entries, files)
	if want := []string{"./a", "./new", "./old"}; !reflect.DeepEqual(selection.GoPackages, want) {
		t.Fatalf("Go packages = %v, want %v", selection.GoPackages, want)
	}
	if want := []string{"clients/ui/src/a.test.ts", "clients/ui/src/deleted/z-sibling.test.ts"}; !reflect.DeepEqual(selection.TypeScript, want) {
		t.Fatalf("TS tests = %v, want %v", selection.TypeScript, want)
	}
}

func TestSelectRepeatTargetsFallsBackForEmptyOrUnmappedDiff(t *testing.T) {
	for _, entries := range [][]DiffEntry{nil, {{Status: "M", Paths: []string{"README.md"}}}} {
		selection := SelectRepeatTargets(entries, nil)
		if !selection.GoFallback || !selection.TSFallback || selection.GoPackages[0] != "./..." || selection.TypeScript[0] != "__ALL__" {
			t.Fatalf("selection did not fail closed: %#v", selection)
		}
	}
}

func TestValidateAttemptsFailsClosed(t *testing.T) {
	pass := make([]AttemptResult, RepeatAttempts)
	for index := range pass {
		pass[index] = AttemptResult{Attempt: index + 1, Result: "pass"}
	}
	if err := ValidateAttempts(pass); err != nil {
		t.Fatal(err)
	}
	for name, attempts := range map[string][]AttemptResult{
		"one failure": append(append([]AttemptResult{}, pass[:4]...), append([]AttemptResult{{Attempt: 5, Result: "fail", ExitCode: 1}}, pass[5:]...)...),
		"timeout":     append(append([]AttemptResult{}, pass[:4]...), append([]AttemptResult{{Attempt: 5, Result: "timeout", ExitCode: 124}}, pass[5:]...)...),
		"interrupted": pass[:9],
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateAttempts(attempts); err == nil || !strings.Contains(err.Error(), "fail") && !strings.Contains(err.Error(), "incomplete") {
				t.Fatalf("expected fail-closed error, got %v", err)
			}
		})
	}
}
