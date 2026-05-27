package secretenv

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/credproxy/secretenv"
)

type stubHook struct {
	vals map[string]string
}

func (h *stubHook) Resolve(_ context.Context, ref string) (string, error) {
	if v, ok := h.vals[ref]; ok {
		return v, nil
	}
	return "", nil
}

func startTestBroker(t *testing.T, allow []string, hook secretenv.Hook) string {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	br := &broker{
		ctx:     context.Background(),
		sock:    sockPath,
		ln:      ln,
		project: "/test/project",
		gate:    NewGate(allow),
		hook:    hook,
		timeout: 5 * time.Second,
		onStop:  func() {},
	}
	go br.serve()
	t.Cleanup(func() { ln.Close() })
	return sockPath
}

func sendRequest(t *testing.T, sockPath string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestBroker_resolves(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	if err := os.WriteFile(envFile, []byte("SECRET=op://vault/item/field\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	hook := &stubHook{vals: map[string]string{"op://vault/item/field": "s3cr3t"}}
	sockPath := startTestBroker(t, []string{filepath.Join(dir, "*.env")}, hook)

	resp := sendRequest(t, sockPath, Request{EnvFilePath: envFile})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Env["SECRET"] != "s3cr3t" {
		t.Errorf("want SECRET=s3cr3t, got %q", resp.Env["SECRET"])
	}
}

func TestBroker_gateBlocks(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	_ = os.WriteFile(envFile, []byte("SECRET=op://vault/item/field\n"), 0o600)

	hook := &stubHook{vals: map[string]string{"op://vault/item/field": "s3cr3t"}}
	// Allow only /other/*.env — different dir.
	sockPath := startTestBroker(t, []string{"/other/*.env"}, hook)

	resp := sendRequest(t, sockPath, Request{EnvFilePath: envFile})
	if resp.Error == "" {
		t.Fatal("expected error, got nil")
	}
	if len(resp.Env) > 0 {
		t.Errorf("expected no env on gate deny, got %v", resp.Env)
	}
}
