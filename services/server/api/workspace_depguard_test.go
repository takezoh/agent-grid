package api

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceWriteDepguardExclusionPresent(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(moduleRoot(t), ".golangci.yml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "workspace_write") || !strings.Contains(text, "forbidigo") {
		t.Fatal("forbidigo exclusion for workspace_write.go missing from .golangci.yml")
	}
	if !strings.Contains(text, "server/api/workspace_write") {
		t.Fatal("workspace_write.go per-file exclusion path missing from .golangci.yml")
	}
}

func TestWorkspaceWriteMutationAllowlistIsOnlyWriteHandler(t *testing.T) {
	t.Parallel()
	root := filepath.Join(moduleRoot(t), "server/api")
	writeData, err := os.ReadFile(filepath.Join(root, "workspace_write.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(writeData), "os.WriteFile(") {
		t.Fatal("workspace_write.go must contain sanctioned os.WriteFile")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"os.WriteFile(", "os.Rename(", "os.CreateTemp("}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, "workspace") || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == "workspace_write.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		for _, needle := range forbidden {
			if strings.Contains(src, needle) {
				t.Fatalf("%s contains forbidden %q; only workspace_write.go may mutate (removing yml exclusion would violate allowlist)", name, needle)
			}
		}
	}
}

func TestWorkspaceWriteForbidigoExclusionPassesLint(t *testing.T) {
	if testing.Short() {
		t.Skip("forbidigo exclusion lint skipped in -short mode")
	}
	root := moduleRoot(t)
	cmd := exec.Command(
		"go", "tool", "golangci-lint", "run",
		"-c", ".golangci.yml",
		"--enable-only=forbidigo",
		"--tests=false",
		"./server/api/",
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOLANGCI_LINT_CACHE="+filepath.Join(t.TempDir(), "golangci-lint-cache"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("workspace_write.go should pass forbidigo via exclusion; err=%v stderr=%s", err, stderr.String())
	}
}

func TestWorkspaceReadHandlersHaveNoDirectWriteSyscalls(t *testing.T) {
	t.Parallel()
	root := filepath.Join(moduleRoot(t), "server/api")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"os.WriteFile(", "os.Rename(", "os.CreateTemp("}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, "workspace") || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == "workspace_write.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		for _, needle := range forbidden {
			if strings.Contains(src, needle) {
				t.Fatalf("%s contains forbidden %q (only workspace_write.go may mutate)", name, needle)
			}
		}
	}
}
