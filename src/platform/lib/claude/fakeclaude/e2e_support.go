//go:build e2e

package fakeclaude

import (
	"os"
	"os/exec"
	"testing"
)

// E2EClaudeBin returns the path to the real claude binary from
// REACTOR_E2E_CLAUDE_BIN, or skips the test.
//
// Compiled only under the `e2e` build tag, so shipping code cannot accidentally
// import it. Callers under `client/lib/agenthook/` share this helper so the
// env-var name and skip messages stay in sync.
func E2EClaudeBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("REACTOR_E2E_CLAUDE_BIN")
	if bin == "" {
		t.Skip("REACTOR_E2E_CLAUDE_BIN is not set — skipping real-claude e2e")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("REACTOR_E2E_CLAUDE_BIN=%q is not executable: %v", bin, err)
	}
	return bin
}
