package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/services/platform/lib/systemdnotify"
	"github.com/takezoh/agent-grid/services/platform/lib/tlsdev"
	serverapi "github.com/takezoh/agent-grid/services/server/api"
)

// shutdownGracePeriod bounds the time gateway.Shutdown waits for in-flight
// requests to drain before forcing the listener closed. Used by both the
// belt-and-braces Close() path on the daemon's exit and by the ctx-cancel
// goroutine that watches the shared daemon context.
const shutdownGracePeriod = 5 * time.Second

var notifyReady = systemdnotify.Ready

// gatewayHandle keeps the resources the gateway goroutine owns so the daemon
// can reap them on shutdown.
type gatewayHandle struct {
	daemon *serverapi.DaemonClient
	srv    *http.Server
	done   <-chan struct{}
}

// Close shuts the gateway HTTP server down (best-effort shutdownGracePeriod
// grace) and closes the in-process DaemonClient. Safe to call once; the
// goroutine that bound the listener watches ctx and tears down on cancel as
// well, so Close is a belt-and-braces hook for the deferred path on the
// daemon's exit.
func (g *gatewayHandle) Close() {
	if g == nil {
		return
	}
	if g.srv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		_ = g.srv.Shutdown(shutCtx)
		cancel()
	}
	if g.done != nil {
		<-g.done
	}
	if g.daemon != nil {
		g.daemon.Close()
	}
}

// startGateway boots the co-resident HTTP/WS gateway as a goroutine that dials
// the daemon's IPC socket. The gateway shares the daemon context, so signal-
// driven shutdown cascades into it. A panic inside the gateway goroutine is
// contained by recover() — it cancels the daemon ctx so the binary exits, but
// the recovered stack is logged rather than dropped into the void.
func startGateway(ctx context.Context, cancel context.CancelFunc, sockPath, dataDir string, df *daemonFlagSet, stdout io.Writer) (*gatewayHandle, error) {
	return startGatewayWithAuthority(ctx, cancel, sockPath, dataDir, df, stdout, nil)
}

func startGatewayWithAuthority(ctx context.Context, cancel context.CancelFunc, sockPath, dataDir string, df *daemonFlagSet, stdout io.Writer, authority serverapi.RepositoryAuthoritySink) (*gatewayHandle, error) {
	token, err := resolveAuth(df.token, df.tokenFile, df.noAuth)
	if err != nil {
		return nil, err
	}
	daemon := serverapi.NewDaemonClient(sockPath)
	srv := &http.Server{
		Addr:              df.addr,
		Handler:           buildHTTPHandlerWithAuthority(daemon, dataDir, token, df.noAuth, authority),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Listener has to bind synchronously so a port conflict (or TLS cert error)
	// is reported BEFORE startGateway returns nil. tlsdev.Serve calls
	// ListenAndServe internally, which blocks; so we open the listener here
	// and hand it to a custom Serve loop on a goroutine.
	ln, err := net.Listen("tcp", df.addr)
	if err != nil {
		daemon.Close()
		return nil, fmt.Errorf("listen %s: %w", df.addr, err)
	}

	doneCh := make(chan struct{})
	effectiveAddr := ln.Addr()
	logStartup(effectiveAddr.String(), df.insecure, df.noAuth, sockPath)
	writeStartupStatus(stdout, effectiveAddr, df.insecure, df.noAuth, df.printToken, token)

	go func() {
		<-ctx.Done()
		grace, gcancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer gcancel()
		_ = srv.Shutdown(grace)
	}()

	go func() {
		defer close(doneCh)
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("gateway: goroutine panicked",
					"err", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()))
				cancel()
			}
		}()
		if err := notifyReady(); err != nil {
			slog.Warn("gateway: readiness notify failed", "err", err)
		}
		serveErr := tlsdev.ServeListener(srv, ln, df.insecure, df.certFile, df.keyFile)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("gateway: serve failed", "err", serveErr)
			cancel()
		}
	}()

	return &gatewayHandle{daemon: daemon, srv: srv, done: doneCh}, nil
}

func writeStartupStatus(w io.Writer, addr net.Addr, insecure, noAuth, printToken bool, token string) {
	if w == nil {
		return
	}
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	scheme := "https"
	tlsEnabled := true
	if insecure {
		scheme = "http"
		tlsEnabled = false
	}
	auth := "bearer-token"
	if noAuth {
		auth = "disabled"
	}
	fmt.Fprintf(w, "server: listening address=%s port=%s tls=%t insecure=%t auth=%s url=%s://%s\n",
		host, port, tlsEnabled, insecure, auth, scheme, addr.String())
	if printToken && !noAuth {
		fmt.Fprintf(w, "server: bearer token=%s\n", token)
	}
}

// resolveAuth materializes the effective bearer token. -no-auth wins and
// forces the token to "" (the gateway-side TokenAuth contract treats "" as
// "reject everything", which is why no-auth mounts the API handler directly
// instead of routing it through TokenAuth). Without -no-auth, an empty user
// token is rejected so normal restarts cannot silently change identity.
// Network exposure is evaluated after the listener is bound; logStartup emits a strong warning
// whenever -no-auth is active on a non-loopback listener.
//
// Precedence (highest first): -no-auth > -token > -token-file.
// -token and -token-file are mutually exclusive: silent precedence between
// "value flag" and "file flag" hides operator intent (was the literal stale,
// or did the file path typo?). Surface that ambiguity BEFORE the -no-auth
// short-circuit so a misconfig is reported now, not the moment an operator
// later removes -no-auth and the hidden conflict suddenly becomes fatal.
func resolveAuth(tokenFlag, tokenFile string, noAuth bool) (string, error) {
	if tokenFlag != "" && tokenFile != "" {
		return "", errors.New("-token and -token-file are mutually exclusive; pick one")
	}
	if noAuth {
		return "", nil
	}
	if tokenFlag != "" {
		return tokenFlag, nil
	}
	if tokenFile != "" {
		return tokenFromFile(tokenFile)
	}
	return "", errors.New("authentication mode is required: pass -token, -token-file, or -no-auth")
}

// tokenFromFile reads a stable, operator-owned bearer token. It never creates
// or replaces credentials: missing, unreadable, and empty files fail startup
// so server and clients cannot rotate independently.
func tokenFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file %q: %w", path, err)
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", fmt.Errorf("token file %q is empty", path)
	}
	_ = os.Chmod(path, 0o600)
	return tok, nil
}

func buildHTTPHandlerWithAuthority(daemon *serverapi.DaemonClient, dataDir, token string, noAuth bool, authority serverapi.RepositoryAuthoritySink) http.Handler {
	serverapi.InitWorkspaceOperatorAuditor(daemon, dataDir)
	mux := http.NewServeMux()
	if noAuth {
		mux.Handle("/", serverapi.NewMuxNoAuthWithRepositoryAuthority(daemon, authority))
	} else {
		mux.Handle("/", serverapi.NewMuxWithRepositoryAuthority(daemon, token, authority))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, daemon)
	})
	return mux
}

func logStartup(addr string, insecure, noAuth bool, sockPath string) {
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	authDesc := "bearer-token"
	if noAuth {
		authDesc = "disabled"
	}
	slog.Info("gateway listening",
		"url", fmt.Sprintf("%s://%s", scheme, addr),
		"sock", sockPath,
		"auth", authDesc)
	if noAuth {
		if isLoopbackAddr(addr) {
			slog.Warn("gateway: -no-auth — bearer-token and WS-ticket checks are disabled. " +
				"Anyone reaching this loopback port can drive every session.")
		} else {
			slog.Warn("gateway: -no-auth on NON-LOOPBACK bind — auth is disabled and the "+
				"REST/WS surface is reachable from the network. Anyone who can reach this "+
				"port can drive every session.",
				"addr", addr)
		}
	}
}

func writeHealth(w http.ResponseWriter, d *serverapi.DaemonClient) {
	healthy := d.Health()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	status := "ok"
	if !healthy {
		status = "daemon-unavailable"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          status,
		"last_attempt_at": d.LastAttemptAt(),
	})
}

// isLoopbackAddr reports whether listenAddr binds to a loopback interface.
// Used to select the startup warning for -no-auth after the listener has
// successfully bound. A non-loopback result means the unauthenticated REST
// and WebSocket surface may be reachable from the network.
// Accepts:  "127.0.0.1:8443", "[::1]:8443", "localhost:8443", "127.0.0.1"
// Rejects:  ":8443" (wildcard), "0.0.0.0:8443", "192.168.1.5:8443"
func isLoopbackAddr(listenAddr string) bool {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		// SplitHostPort fails on a bare host with no port. Treat the input as
		// a host literal in that case.
		host = listenAddr
	}
	if host == "" {
		return false // ":8443" form binds the wildcard — explicitly unsafe.
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
