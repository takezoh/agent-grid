// Package winexec implements the WSL2 Windows exe broker: a host-side Unix
// socket server that executes allowlisted Windows binaries on behalf of
// container processes, forwarding stdio via SCM_RIGHTS file descriptor passing.
package winexec

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
)

// Request is the control message sent by the container-side client.
type Request struct {
	Name string   `json:"name"` // exe basename, e.g. "code.exe"
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"` // working directory hint; empty = unset
}

// Response is the control message returned by the broker after the child exits.
type Response struct {
	ExitCode int `json:"exit_code"`
}

// SendRequest writes req as JSON alongside fds (stdin/stdout/stderr) in a
// single WriteMsgUnix call. The fds are passed via SCM_RIGHTS ancillary data.
func SendRequest(conn *net.UnixConn, req Request, fds [3]int) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("winexec: marshal request: %w", err)
	}
	rights := syscall.UnixRights(fds[0], fds[1], fds[2])
	_, _, err = conn.WriteMsgUnix(data, rights, nil)
	if err != nil {
		return fmt.Errorf("winexec: send request: %w", err)
	}
	return nil
}

// RecvRequest reads a request and its three stdio fds from conn.
func RecvRequest(conn *net.UnixConn) (Request, [3]int, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 128)
	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("winexec: recv request: %w", err)
	}

	var req Request
	if err := json.Unmarshal(buf[:n], &req); err != nil {
		return Request{}, [3]int{}, fmt.Errorf("winexec: unmarshal request: %w", err)
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("winexec: parse cmsg: %w", err)
	}
	if len(scms) == 0 {
		return Request{}, [3]int{}, fmt.Errorf("winexec: no fds in request")
	}
	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("winexec: parse unix rights: %w", err)
	}
	if len(fds) < 3 {
		return Request{}, [3]int{}, fmt.Errorf("winexec: expected 3 fds, got %d", len(fds))
	}
	return req, [3]int{fds[0], fds[1], fds[2]}, nil
}
