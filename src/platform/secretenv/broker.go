package secretenv

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/takezoh/credproxy/secretenv"
)

// broker is a per-project unix socket server that gates and resolves secret env-files.
type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	onStop  func()

	// mu guards gate, hook, and timeout. These fields may be updated by
	// ensureBroker (under SpecBuilder.mu) while resolve() runs concurrently
	// in connection-handler goroutines, so a broker-level RWMutex is required.
	mu      sync.RWMutex
	gate    *Gate
	hook    secretenv.Hook
	timeout time.Duration
}

func (b *broker) setConfig(gate *Gate, hook secretenv.Hook, timeout time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gate = gate
	b.hook = hook
	b.timeout = timeout
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
	hook := b.hook
	timeout := b.timeout
	b.mu.RUnlock()

	if err := gate.Check(req.EnvFilePath); err != nil {
		slog.Warn("secretenv: gate denied", "project", b.project, "path", req.EnvFilePath, "err", err)
		return Response{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(b.ctx, timeout)
	defer cancel()

	resolver := secretenv.NewResolver(hook)
	env, err := resolver.ResolveFile(ctx, req.EnvFilePath)
	if err != nil {
		slog.Warn("secretenv: resolve failed", "project", b.project, "path", req.EnvFilePath, "err", err)
		return Response{Error: err.Error()}
	}
	return Response{Env: env}
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("secretenv: marshal response", "err", err)
		return
	}
	_, _ = conn.Write(data)
}
