package harnesspolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryProtectedManifestIsInternallyValid(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "test-harness", "protected.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseProtectedManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ReadProtectedTree(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	result := ClassifyProtectedChanges(manifest, tree, cloneProtectedTree(tree), nil)
	if result.Status != "pass" {
		t.Fatalf("repository baseline is invalid: %+v", result)
	}
}

func TestClassifyProtectedChangesPassesIdenticalTrees(t *testing.T) {
	manifest := testProtectedManifest()
	base := testProtectedTree()
	result := ClassifyProtectedChanges(manifest, base, cloneProtectedTree(base), nil)
	if result.Status != "pass" || len(result.Findings) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClassifyProtectedChangesRejectsDeletionAndWeakening(t *testing.T) {
	tests := []struct {
		name string
		head ProtectedTree
		want []string
	}{
		{"checker deletion", ProtectedTree{"static_test.go": []byte("invoke trusted-gate")}, []string{"checker.go:deleted"}},
		{"invocation removal", ProtectedTree{"checker.go": []byte("checker-body"), "static_test.go": []byte("no invocation")}, []string{"static_test.go:weakened"}},
		{"simultaneous deletion", ProtectedTree{}, []string{"checker.go:deleted", "static_test.go:deleted"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ClassifyProtectedChanges(testProtectedManifest(), testProtectedTree(), test.head, nil)
			if result.Status != "review-required" {
				t.Fatalf("got status %q", result.Status)
			}
			for _, want := range test.want {
				if !resultHasFinding(result, want) {
					t.Fatalf("result %+v lacks %q", result, want)
				}
			}
		})
	}
}

func TestEscalationCannotSelfApproveProtectedChange(t *testing.T) {
	head := cloneProtectedTree(testProtectedTree())
	head["checker.go"] = []byte("checker-body changed")
	complete := &EscalationRequest{Reason: "intentional", Owner: "harness", Expiry: "2026-08-01", Evidence: []string{"artifact.json"}}
	result := ClassifyProtectedChanges(testProtectedManifest(), testProtectedTree(), head, complete)
	if result.Status != "review-required" || len(result.RequestErrors) != 0 {
		t.Fatalf("complete request must still require review: %+v", result)
	}

	incomplete := &EscalationRequest{Reason: "intentional"}
	result = ClassifyProtectedChanges(testProtectedManifest(), testProtectedTree(), head, incomplete)
	if result.Status != "review-required" || len(result.RequestErrors) != 3 {
		t.Fatalf("incomplete request accepted: %+v", result)
	}
}

func testProtectedManifest() ProtectedManifest {
	return ProtectedManifest{Version: 1, Paths: []ProtectedPath{
		{Path: "checker.go", Category: "checker", RequiredMarkers: []string{"checker-body"}},
		{Path: "static_test.go", Category: "independent-pin", RequiredMarkers: []string{"trusted-gate"}},
	}}
}

func testProtectedTree() ProtectedTree {
	return ProtectedTree{"checker.go": []byte("checker-body"), "static_test.go": []byte("invoke trusted-gate")}
}

func cloneProtectedTree(source ProtectedTree) ProtectedTree {
	clone := make(ProtectedTree, len(source))
	for path, data := range source {
		clone[path] = append([]byte(nil), data...)
	}
	return clone
}

func resultHasFinding(result TamperingResult, want string) bool {
	for _, finding := range result.Findings {
		if finding.Path+":"+finding.Change == want || strings.Contains(finding.Detail, want) {
			return true
		}
	}
	return false
}
