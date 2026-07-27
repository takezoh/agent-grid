//go:build e2e

package grok

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFakeVsRealGrokLifecycleFlags(t *testing.T) {
	bin := os.Getenv("AG_E2E_GROK_BIN")
	if bin == "" {
		t.Skip("AG_E2E_GROK_BIN is not set")
	}
	out, err := exec.Command(bin, "--no-auto-update", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("grok --help: %v\n%s", err, out)
	}
	help := string(out)
	for _, flag := range []string{"--session-id", "--resume", "--continue", "--fork-session"} {
		if !strings.Contains(help, flag) {
			t.Fatalf("real grok help missing %s; update fake/contract rather than weakening this assertion", flag)
		}
	}
	if out, err := exec.Command(bin, "--no-auto-update", "version").CombinedOutput(); err != nil {
		t.Fatalf("real grok --no-auto-update version: %v\n%s", err, out)
	}
}
