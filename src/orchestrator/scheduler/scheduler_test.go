package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

const validFrontMatter = `---
tracker:
  kind: linear
  api_key: lin_api_test
  project_slug: test-proj
codex:
  command: codex app-server
---
`

func writeWorkflow(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunGracefulShutdown(t *testing.T) {
	path := writeWorkflow(t, validFrontMatter)
	cfg := wfconfig.Config{
		Polling: wfconfig.PollingConfig{IntervalMS: 1},
	}
	s := New(path, cfg, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not shut down in time")
	}
}

func TestRunContinuesAfterTickPreflightFailure(t *testing.T) {
	// Write a valid workflow initially; we replace it below with an invalid one
	// to verify the loop continues rather than stopping.
	path := writeWorkflow(t, validFrontMatter)
	cfg := wfconfig.Config{
		Polling: wfconfig.PollingConfig{IntervalMS: 1},
	}
	s := New(path, cfg, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	// Overwrite with missing project_slug to trigger preflight error.
	badContent := strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", "")
	if err := os.WriteFile(path, []byte(badContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Loop must not stop; cancel after a short time and expect nil.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not shut down in time")
	}
}
