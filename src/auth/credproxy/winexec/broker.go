package winexec

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"sync/atomic"

	"github.com/takezoh/agent-roost/config"
)

// broker listens on a per-project Unix socket and handles one exec per connection.
type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	cfg     atomic.Pointer[config.WinExecConfig]
	onStop  func() // called when serve exits, to remove from parent map
}

func (b *broker) serve() {
	defer b.ln.Close()
	defer func() { _ = os.Remove(b.sock) }()
	defer b.onStop()
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			if b.ctx.Err() != nil {
				return // context cancelled; normal shutdown
			}
			slog.Warn("winexec: accept error", "project", b.project, "err", err)
			return
		}
		go b.handleConn(conn.(*net.UnixConn))
	}
}

func (b *broker) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("winexec: panic in handler", "project", b.project, "recover", r)
		}
	}()

	req, fds, err := RecvRequest(conn)
	if err != nil {
		slog.Warn("winexec: recv request failed", "project", b.project, "err", err)
		return
	}

	exitCode := executeRequest(b.ctx, *b.cfg.Load(), b.project, req, fds)

	resp, _ := json.Marshal(Response{ExitCode: exitCode})
	_, _ = conn.Write(resp)
}
