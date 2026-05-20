package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	codexschemav1 "github.com/takezoh/agent-roost/platform/agent/codexschema/v1"
	"github.com/takezoh/agent-roost/platform/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, codexclient.DefaultStdioTransport())
	stop()
	os.Exit(code)
}

type appHandler struct {
	conn *codexclient.Conn
}

func (h *appHandler) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = h.conn.Reply(id, initializeResponse())
	default:
		_ = h.conn.ReplyError(id, fmt.Sprintf("method %q not implemented", method))
	}
}

// initializeResponse builds a schema-valid Codex InitializeResponse. The shim
// reports its own platform metadata; codexHome falls back to the conventional
// ~/.codex when $CODEX_HOME is unset.
func initializeResponse() codexschemav1.InitializeResponse {
	platformOS := runtime.GOOS
	if platformOS == "darwin" {
		platformOS = "macos"
	}
	family := "unix"
	if runtime.GOOS == "windows" {
		family = "windows"
	}
	return codexschemav1.InitializeResponse{
		CodexHome:      codexHome(),
		PlatformFamily: family,
		PlatformOS:     platformOS,
		UserAgent:      "claude-app-server/0",
	}
}

func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return filepath.Join(home, ".codex")
}

func (h *appHandler) OnNotification(method string, _ json.RawMessage) {
	slog.Debug("notification received", "method", method)
}

func run(ctx context.Context, t codexclient.Transport) int {
	if err := logger.Init("info"); err != nil {
		fmt.Fprintf(os.Stderr, "claude-app-server: logger init: %v\n", err)
		return 1
	}
	defer logger.Close()

	conn := codexclient.NewConn(t, 0)
	h := &appHandler{conn: conn}

	done := make(chan error, 1)
	go func() { done <- conn.Run(ctx, h) }()

	select {
	case <-ctx.Done():
		slog.Info("claude-app-server stopping")
	case <-done:
		slog.Info("claude-app-server stopped")
	}
	return 0
}
