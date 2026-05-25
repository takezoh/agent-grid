package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func runSockBridge(args []string) error {
	fs := flag.NewFlagSet("sockbridge", flag.ContinueOnError)
	listen := fs.String("listen", "", "TCP address to listen on (required)")
	routeDir := fs.String("route-dir", "", "directory with per-session unix sockets (required)")
	routePrefix := fs.String("route-prefix", "", "socket filename prefix")
	routeSuffix := fs.String("route-suffix", "", "socket filename suffix")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *listen == "" {
		return fmt.Errorf("sockbridge: -listen is required")
	}
	if *routeDir == "" {
		return fmt.Errorf("sockbridge: -route-dir is required")
	}

	srv := &http.Server{
		Addr:    *listen,
		Handler: newBridgeHandler(*routeDir, *routePrefix, *routeSuffix),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		srv.Close() //nolint:errcheck
	}()

	slog.Info("sockbridge: listening", "addr", *listen, "route-dir", *routeDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("sockbridge: %w", err)
	}
	return nil
}

// newBridgeHandler returns an HTTP handler that routes each incoming connection
// (typically a WebSocket upgrade) to the unix socket at
// routeDir/<prefix><sessionID><suffix>, where sessionID comes from the URL path.
// After forwarding the HTTP request, bytes are copied bidirectionally between
// the TCP frontend and the unix-socket backend.
func newBridgeHandler(routeDir, prefix, suffix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/")
		if sessionID == "" || strings.ContainsAny(sessionID, "/\\") {
			http.Error(w, "invalid session ID", http.StatusBadRequest)
			return
		}

		sockPath := filepath.Join(routeDir, prefix+sessionID+suffix)
		backend, err := net.Dial("unix", sockPath)
		if err != nil {
			slog.Debug("sockbridge: backend unavailable", "session", sessionID, "sock", sockPath, "err", err)
			http.Error(w, "backend unavailable", http.StatusServiceUnavailable)
			return
		}

		if err := r.Write(backend); err != nil {
			backend.Close()
			return
		}

		hj, ok := w.(http.Hijacker)
		if !ok {
			backend.Close()
			slog.Error("sockbridge: ResponseWriter does not support hijacking")
			return
		}
		frontend, _, err := hj.Hijack()
		if err != nil {
			backend.Close()
			slog.Error("sockbridge: hijack failed", "err", err)
			return
		}

		bridgePipe(frontend, backend)
	})
}

// bridgePipe copies bytes bidirectionally between a and b until either
// direction closes, then closes both connections.
func bridgePipe(a, b net.Conn) {
	defer a.Close()
	defer b.Close()
	done := make(chan struct{}, 2)
	relay := func(dst, src net.Conn) {
		defer func() { done <- struct{}{} }()
		io.Copy(dst, src) //nolint:errcheck
	}
	go relay(a, b)
	go relay(b, a)
	<-done
}
