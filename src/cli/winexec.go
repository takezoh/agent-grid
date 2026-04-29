package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/takezoh/agent-roost/auth/credproxy/winexec"
	"github.com/takezoh/agent-roost/runtime"
)

func init() {
	Register("win-exec", "run a Windows exe via the host WSL2 broker", runWinExec)
}

func runWinExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: win-exec <exe> [args...]")
	}

	sockPath := runtime.ContainerWinExecSockPath
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "win-exec: broker unavailable (%v)\n", err)
		os.Exit(127)
	}
	uc := conn.(*net.UnixConn)

	cwd, _ := os.Getwd()
	req := winexec.Request{
		Name: args[0],
		Args: args[1:],
		Cwd:  cwd,
	}

	fds := [3]int{
		int(os.Stdin.Fd()),
		int(os.Stdout.Fd()),
		int(os.Stderr.Fd()),
	}
	if err := winexec.SendRequest(uc, req, fds); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "win-exec: %v\n", err)
		os.Exit(127)
	}

	var resp winexec.Response
	if err := json.NewDecoder(uc).Decode(&resp); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "win-exec: read response: %v\n", err)
		os.Exit(127)
	}

	conn.Close()
	os.Exit(resp.ExitCode)
	return nil // unreachable
}
