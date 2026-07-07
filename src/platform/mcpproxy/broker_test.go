package mcpproxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStartMCPProcess_DoesNotForwardSocketToken(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH"))
	stderrPath := filepath.Join(t.TempDir(), "stderr.log")
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	defer stderr.Close()

	b := &broker{ctx: context.Background()}
	req := Request{Alias: "managed", Token: "secret-token"}
	srv := &serverEntry{alias: "managed", command: "sh", args: []string{"-c", "exit 0"}}
	stdinW, stdoutR, cmd, err := b.startMCPProcess(req, srv, stderr)
	if err != nil {
		t.Fatalf("start managed alias: %v", err)
	}
	_ = stdinW.Close()
	_ = stdoutR.Close()
	_ = cmd.Wait()
	if got := envValue(cmd.Env, "AG_SOCKET_TOKEN"); got != "" {
		t.Fatalf("token = %q, want empty", got)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if len(kv) > len(prefix) && kv[:len(prefix)] == prefix {
			return kv[len(prefix):]
		}
	}
	return ""
}
