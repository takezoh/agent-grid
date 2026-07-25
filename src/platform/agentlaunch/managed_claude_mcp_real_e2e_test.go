//go:build e2e

package agentlaunch

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestFakeVsRealManagedClaudeMCPConfig validates the T0 launch-contract
// fixture against the installed Claude CLI without making a model request.
func TestFakeVsRealManagedClaudeMCPConfig(t *testing.T) {
	bin := os.Getenv("AG_E2E_CLAUDE_BIN")
	if bin == "" {
		t.Skip("AG_E2E_CLAUDE_BIN is not set — skipping real-claude e2e")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("AG_E2E_CLAUDE_BIN=%q is not executable: %v", bin, err)
	}
	args, err := AppendManagedClaudeMCPConfig(
		[]string{"--init-only"},
		"/bin/true",
		"/tmp/agent-grid-contract.sock",
	)
	if err != nil {
		t.Fatalf("AppendManagedClaudeMCPConfig: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = envWithHome(os.Environ(), t.TempDir())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real Claude rejected managed MCP launch config: %v\n%s", err, output)
	}
}

func envWithHome(base []string, home string) []string {
	out := make([]string, 0, len(base)+1)
	for _, item := range base {
		if len(item) >= len("HOME=") && item[:len("HOME=")] == "HOME=" {
			continue
		}
		out = append(out, item)
	}
	return append(out, "HOME="+home)
}
