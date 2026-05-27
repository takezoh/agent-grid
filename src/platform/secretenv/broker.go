package secretenv

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

// resolveTimeout is roost's safety bound on the credproxy resolve subprocess.
// The actual hook timeout is credproxy's concern (credproxy config).
const resolveTimeout = 30 * time.Second

// broker is a per-project unix socket server that gates and resolves secret env-files.
type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	onStop  func()

	// mu guards gate and credproxyBin. These fields may be updated by ensureBroker
	// (under SpecBuilder.mu) while resolve() runs concurrently in connection-handler
	// goroutines, so a broker-level RWMutex is required.
	mu           sync.RWMutex
	gate         *Gate
	credproxyBin string
}

func (b *broker) setConfig(gate *Gate, credproxyBin string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gate = gate
	b.credproxyBin = credproxyBin
}

func (b *broker) serve() {
	defer b.ln.Close()
	defer func() { _ = os.Remove(b.sock) }()
	defer b.onStop()
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			slog.Warn("secretenv: accept error", "project", b.project, "err", err)
			return
		}
		go b.handleConn(conn)
	}
}

func (b *broker) handleConn(conn net.Conn) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("secretenv: panic in handler", "project", b.project, "recover", r)
		}
	}()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		slog.Warn("secretenv: decode request", "project", b.project, "err", err)
		writeResponse(conn, Response{Error: "malformed request"})
		return
	}

	resp := b.resolve(req)
	writeResponse(conn, resp)
}

func (b *broker) resolve(req Request) Response {
	b.mu.RLock()
	gate := b.gate
	bin := b.credproxyBin
	b.mu.RUnlock()

	if err := gate.Check(req.EnvFilePath); err != nil {
		slog.Warn("secretenv: gate denied", "project", b.project, "path", req.EnvFilePath, "err", err)
		return Response{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(b.ctx, resolveTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, bin, "resolve", "--env-file", req.EnvFilePath).Output()
	if err != nil {
		slog.Warn("secretenv: credproxy resolve failed", "project", b.project, "path", req.EnvFilePath, "err", err)
		return Response{Error: err.Error()}
	}

	var result struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		slog.Warn("secretenv: parse resolve output", "project", b.project, "err", err)
		return Response{Error: "invalid resolve output"}
	}
	return Response{Env: result.Env}
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("secretenv: marshal response", "err", err)
		return
	}
	_, _ = conn.Write(data)
}
