// Command server runs the agent-reactor web server: it manages agent sessions
// over pty (tmux-free) and serves a web client that operates them from a
// browser over WebSocket. Auth is a bearer token; transport is TLS
// (self-signed by default, or -tls-cert/-tls-key, or -insecure for local dev).
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	clientweb "github.com/takezoh/agent-reactor/client/web"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/server/session"
	serverweb "github.com/takezoh/agent-reactor/server/web"
)

func main() {
	addr := flag.String("addr", ":8443", "listen address")
	tokenFlag := flag.String("token", "", "bearer token (generated and printed if empty)")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	flag.Parse()

	token := *tokenFlag
	if token == "" {
		token = randToken()
	}

	svc := session.NewService(agentlaunch.DirectDispatcher{})
	handler := serverweb.TokenAuth(token, serverweb.NewMux(svc, clientweb.Assets))
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		svc.CloseAll(context.Background())
	}()

	scheme := "https"
	if *insecure {
		scheme = "http"
	}
	log.Printf("agent-reactor server on %s://%s  token=%s", scheme, *addr, token)
	if err := serve(srv, *insecure, *certFile, *keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func serve(srv *http.Server, insecure bool, cert, key string) error {
	switch {
	case insecure:
		return srv.ListenAndServe()
	case cert != "" && key != "":
		return srv.ListenAndServeTLS(cert, key)
	default:
		tlsCert, err := selfSignedCert()
		if err != nil {
			return err
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{tlsCert}, MinVersion: tls.VersionTLS12}
		return srv.ListenAndServeTLS("", "")
	}
}

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
