// Command web-host is the browser client host. It loads the same configured
// server fleet as every other client host and exposes server-qualified,
// credential-free routes to the shared UI.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
	"github.com/takezoh/agent-grid/clients/web-host/internal/tlsserve"
	"github.com/takezoh/agent-grid/clients/web-host/internal/webhost"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "listen address")
	defaultConfigDir, err := clientconfig.DefaultDirectory()
	if err != nil {
		return err
	}
	configDir := flag.String("config-dir", defaultConfigDir, "client configuration directory")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	flag.Parse()

	snapshot, err := clientconfig.Load(*configDir)
	if err != nil {
		return err
	}
	handler, err := webhost.Handler(snapshot)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	scheme := "https"
	if *insecure {
		scheme = "http"
	}
	log.Printf(
		"agent-grid web client on %s://%s with %d configured servers",
		scheme,
		*addr,
		len(snapshot.Servers),
	)
	if err := tlsserve.Serve(srv, *insecure, *certFile, *keyFile); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
