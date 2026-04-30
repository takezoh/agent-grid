package winexec

import (
	"encoding/json"
	"net"
	"os"
	"syscall"
	"testing"
)

// TestSendRecvRequest exercises the SCM_RIGHTS fd-passing round-trip on a
// socketpair so no filesystem socket is needed.
func TestSendRecvRequest(t *testing.T) {
	client, server, err := unixSocketpair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer client.Close()
	defer server.Close()

	// Create three dummy fds via pipe pairs (stdin/stdout/stderr surrogates).
	r0, w0, _ := os.Pipe()
	r1, w1, _ := os.Pipe()
	r2, w2, _ := os.Pipe()
	defer r0.Close()
	defer w0.Close()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()

	want := Request{Name: "code.exe", Args: []string{"--wait", "."}, Cwd: "/tmp"}
	fds := [3]int{int(r0.Fd()), int(w1.Fd()), int(w2.Fd())}

	errCh := make(chan error, 1)
	go func() {
		errCh <- SendRequest(client, want, fds)
	}()

	got, gotFDs, err := RecvRequest(server)
	if err != nil {
		t.Fatalf("RecvRequest: %v", err)
	}
	if <-errCh != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if len(got.Args) != len(want.Args) || got.Args[0] != want.Args[0] {
		t.Errorf("Args = %v, want %v", got.Args, want.Args)
	}
	if got.Cwd != want.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, want.Cwd)
	}

	// Received fds must be valid (positive).
	for i, fd := range gotFDs {
		if fd <= 0 {
			t.Errorf("gotFDs[%d] = %d, want > 0", i, fd)
		}
		syscall.Close(fd) //nolint:errcheck
	}
}

// TestResponseMarshal verifies Response round-trips through JSON correctly.
func TestResponseMarshal(t *testing.T) {
	resp := Response{ExitCode: 42}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", got.ExitCode)
	}
}

// unixSocketpair creates a connected pair of Unix stream sockets.
func unixSocketpair() (*net.UnixConn, *net.UnixConn, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	a, err := net.FileConn(os.NewFile(uintptr(fds[0]), ""))
	if err != nil {
		return nil, nil, err
	}
	b, err := net.FileConn(os.NewFile(uintptr(fds[1]), ""))
	if err != nil {
		a.Close()
		return nil, nil, err
	}
	return a.(*net.UnixConn), b.(*net.UnixConn), nil
}
