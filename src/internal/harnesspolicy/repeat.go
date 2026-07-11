package harnesspolicy

import (
	"bufio"
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	RepeatSeed     = 20260711
	RepeatAttempts = 10
)

type DiffEntry struct {
	Status string
	Paths  []string
}

type RepositoryFile struct {
	Path    string
	Content string
}

type RepeatSelection struct {
	GoPackages []string
	TypeScript []string
	GoFallback bool
	TSFallback bool
}

func ParseNameStatus(input string) ([]DiffEntry, error) {
	entries := make([]DiffEntry, 0)
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		status := fields[0]
		pathCount := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			pathCount = 2
		}
		if len(fields) != pathCount+1 {
			return nil, fmt.Errorf("invalid name-status line %q", line)
		}
		entries = append(entries, DiffEntry{Status: status, Paths: append([]string(nil), fields[1:]...)})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read name-status: %w", err)
	}
	return entries, nil
}

func SelectRepeatTargets(entries []DiffEntry, files []RepositoryFile) RepeatSelection {
	goTestsByDir := make(map[string]bool)
	tsTests := make([]RepositoryFile, 0)
	for _, file := range files {
		if strings.HasSuffix(file.Path, "_test.go") {
			goTestsByDir[path.Dir(file.Path)] = true
		}
		if isTypeScriptTest(file.Path) {
			tsTests = append(tsTests, file)
		}
	}
	goTargets := make(map[string]bool)
	tsTargets := make(map[string]bool)
	for _, entry := range entries {
		for _, changed := range entry.Paths {
			if strings.HasSuffix(changed, ".go") {
				dir := path.Dir(changed)
				if goTestsByDir[dir] {
					goTargets[goPackage(dir)] = true
				}
			}
			if isTypeScriptTest(changed) {
				if repositoryHas(files, changed) {
					tsTargets[changed] = true
				}
				for _, test := range tsTests {
					if path.Dir(test.Path) == path.Dir(changed) {
						tsTargets[test.Path] = true
					}
				}
				continue
			}
			if !isTypeScriptSource(changed) {
				continue
			}
			for _, test := range tsTests {
				if path.Dir(test.Path) == path.Dir(changed) || importsSibling(test, changed) {
					tsTargets[test.Path] = true
				}
			}
		}
	}
	selection := RepeatSelection{GoPackages: sortedKeys(goTargets), TypeScript: sortedKeys(tsTargets)}
	if len(selection.GoPackages) == 0 {
		selection.GoPackages = []string{"./..."}
		selection.GoFallback = true
	}
	if len(selection.TypeScript) == 0 {
		selection.TypeScript = []string{"__ALL__"}
		selection.TSFallback = true
	}
	return selection
}

type AttemptResult struct {
	Attempt    int    `json:"attempt"`
	Case       string `json:"case"`
	DurationMS int64  `json:"duration_ms"`
	Result     string `json:"result"`
	ExitCode   int    `json:"exit_code"`
}

func ValidateAttempts(attempts []AttemptResult) error {
	if len(attempts) != RepeatAttempts {
		return fmt.Errorf("repeat is incomplete: got %d attempts, want %d", len(attempts), RepeatAttempts)
	}
	for index, attempt := range attempts {
		if attempt.Attempt != index+1 {
			return fmt.Errorf("attempt sequence is incomplete at index %d", index)
		}
		if attempt.Result != "pass" || attempt.ExitCode != 0 {
			return fmt.Errorf("attempt %d failed closed: result=%s exit=%d", attempt.Attempt, attempt.Result, attempt.ExitCode)
		}
	}
	return nil
}

func goPackage(dir string) string {
	if dir == "src" || dir == "." {
		return "."
	}
	return "./" + strings.TrimPrefix(dir, "src/")
}

func repositoryHas(files []RepositoryFile, target string) bool {
	for _, file := range files {
		if file.Path == target {
			return true
		}
	}
	return false
}

func importsSibling(test RepositoryFile, source string) bool {
	base := strings.TrimSuffix(path.Base(source), path.Ext(source))
	return strings.Contains(test.Content, "./"+base) || strings.Contains(test.Content, "../"+base)
}

func isTypeScriptSource(file string) bool {
	return strings.HasSuffix(file, ".ts") || strings.HasSuffix(file, ".tsx")
}

func isTypeScriptTest(file string) bool {
	return strings.Contains(file, ".test.ts") || strings.Contains(file, ".test.tsx") ||
		strings.Contains(file, ".spec.ts") || strings.Contains(file, ".spec.tsx")
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
