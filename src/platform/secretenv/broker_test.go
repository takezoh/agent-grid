package secretenv

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeCredproxy writes a shell script that acts as a fake "credproxy resolve"
// and returns its path. The script emits the given JSON response to stdout.
func writeFakeCredproxy(t *testing.T, dir, jsonResponse string) string {
	t.Helper()
	path := filepath.Join(dir, "credproxy")
	// The script accepts "resolve --env-file <path>" and emits fixed JSON.
	script := "#!/bin/sh\nprintf '%s' '" + jsonResponse + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake credproxy: %v", err)
	}
	return path
}

func startTestBroker(t *testing.T, allow []string, credproxyBin string) string {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	br := &broker{
		ctx:          context.Background(),
		sock:         sockPath,
		ln:           ln,
		project:      "/test/project",
		gate:         NewGate(allow),
		credproxyBin: credproxyBin,
		onStop:       func() {},
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

	fakeJSON := `{"env":{"SECRET":"s3cr3t"}}`
	fakeBin := writeFakeCredproxy(t, dir, fakeJSON)
	sockPath := startTestBroker(t, []string{filepath.Join(dir, "*.env")}, fakeBin)

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

	// Allow only /other/*.env — different dir. Fake bin never called (gate fires first).
	fakeBin := writeFakeCredproxy(t, dir, `{"env":{"SECRET":"s3cr3t"}}`)
	sockPath := startTestBroker(t, []string{"/other/*.env"}, fakeBin)

	resp := sendRequest(t, sockPath, Request{EnvFilePath: envFile})
	if resp.Error == "" {
		t.Fatal("expected error, got nil")
	}
	if len(resp.Env) > 0 {
		t.Errorf("expected no env on gate deny, got %v", resp.Env)
	}
}
