package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// packageSourcesContaining scans this package's production sources for a
// marker. It is how the two "never reads X" rules in this package are checked:
// a behavioural test can only show that the current path does not read
// something, while the scan shows that no path does.
func packageSourcesContaining(t *testing.T, markers ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	var offending []string
	for _, file := range entries {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(".", name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		for _, marker := range markers {
			if strings.Contains(string(raw), marker) {
				offending = append(offending, name)
				break
			}
		}
	}
	return offending
}
