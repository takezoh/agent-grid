package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// runSockBridge implements the "sockbridge" subcommand.
// It supports two modes:
//
//   - Fixed-socket mode (-socket): forwards every TCP connection to one UDS.
//   - Routing mode (-route-dir): routes each HTTP/WebSocket connection to a
//     per-session UDS derived from the URL path token.
func runSockBridge(args []string) error {
	fs := flag.NewFlagSet("sockbridge", flag.ContinueOnError)
	listen := fs.String("listen", "", "TCP address to listen on (required)")
	socket := fs.String("socket", "", "fixed unix socket path (fixed-socket mode)")
	routeDir := fs.String("route-dir", "", "directory of per-session sockets (routing mode)")
	routePrefix := fs.String("route-prefix", "codex-", "socket filename prefix (routing mode)")
	routeSuffix := fs.String("route-suffix", ".sock", "socket filename suffix (routing mode)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *listen == "" {
		return fmt.Errorf("sockbridge: -listen is required")
	}
	if (*socket == "") == (*routeDir == "") {
		return fmt.Errorf("sockbridge: exactly one of -socket or -route-dir is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *routeDir != "" {
		return bridgeRunRouter(ctx, *listen, *routeDir, *routePrefix, *routeSuffix)
	}
	return bridgeRun(ctx, *listen, *socket)
}

// bridgeRun listens on listenAddr (TCP) and forwards each connection to socketPath (unix).
func bridgeRun(ctx context.Context, listenAddr, socketPath string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	slog.Debug("sockbridge: listening", "tcp", listenAddr, "unix", socketPath)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go bridgeForward(conn, socketPath)
	}
}

// bridgeRunRouter listens on listenAddr (TCP) and routes each HTTP/WebSocket
// connection to routeDir/routePrefix+<token>+routeSuffix where token is the
// first URL path segment (e.g. /abc123 → "abc123"). Path is rewritten to "/"
// before forwarding. Tokens not matching [0-9a-zA-Z_-] receive a 502.
func bridgeRunRouter(ctx context.Context, listenAddr, routeDir, routePrefix, routeSuffix string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	slog.Debug("sockbridge: routing listener", "tcp", listenAddr, "dir", routeDir)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go bridgeRouteConn(conn, routeDir, routePrefix, routeSuffix)
	}
}

func bridgeForward(tcp net.Conn, socketPath string) {
	defer tcp.Close()
	unix, err := net.Dial("unix", socketPath)
	if err != nil {
		slog.Warn("sockbridge: dial unix failed", "socket", socketPath, "err", err)
		return
	}
	bridgePump(tcp, unix)
}

func bridgeRouteConn(tcp net.Conn, routeDir, routePrefix, routeSuffix string) {
	defer tcp.Close()

	header, pending, err := bridgeReadHTTPHeader(tcp)
	if err != nil {
		return
	}

	lineEnd := bytes.IndexByte(header, '\n')
	if lineEnd < 0 {
		bridgeReply502(tcp)
		return
	}
	firstLine := strings.TrimRight(string(header[:lineEnd]), "\r\n")
	parts := strings.SplitN(firstLine, " ", 3)
	if len(parts) != 3 {
		bridgeReply502(tcp)
		return
	}
	method, rawPath, proto := parts[0], parts[1], parts[2]

	token := bridgeExtractToken(rawPath)
	if token == "" {
		slog.Warn("sockbridge: invalid or missing route token", "path", rawPath)
		bridgeReply502(tcp)
		return
	}

	sockPath := filepath.Join(routeDir, routePrefix+token+routeSuffix)
	unix, err := net.Dial("unix", sockPath)
	if err != nil {
		slog.Warn("sockbridge: route dial failed", "token", token, "sock", sockPath, "err", err)
		bridgeReply502(tcp)
		return
	}

	// Rewrite path to "/" and replay headers to the unix conn.
	rewrittenLine := method + " / " + proto + "\r\n"
	rest := header[lineEnd+1:]
	if _, err := io.WriteString(unix, rewrittenLine); err != nil {
		unix.Close()
		return
	}
	if _, err := unix.Write(rest); err != nil {
		unix.Close()
		return
	}
	if len(pending) > 0 {
		if _, err := unix.Write(pending); err != nil {
			unix.Close()
			return
		}
	}

	bridgePump(tcp, unix)
}

type halfCloser interface {
	CloseWrite() error
}

func bridgePump(tcp, unix net.Conn) {
	defer unix.Close()
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		io.Copy(dst, src) //nolint:errcheck
		if hc, ok := dst.(halfCloser); ok {
			hc.CloseWrite() //nolint:errcheck
		}
		done <- struct{}{}
	}
	go pipe(tcp, unix)
	go pipe(unix, tcp)
	<-done
	<-done
}

func bridgeReadHTTPHeader(conn net.Conn) (header, pending []byte, err error) {
	buf := make([]byte, 0, 1024)
	chunk := make([]byte, 512)
	for len(buf) < 16384 {
		n, e := conn.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if idx := bridgeFindHeadersEnd(buf); idx >= 0 {
				end := idx + 4
				return buf[:end], buf[end:], nil
			}
		}
		if e != nil {
			return nil, nil, e
		}
	}
	return nil, nil, errors.New("HTTP header too large")
}

func bridgeFindHeadersEnd(b []byte) int {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == '\r' && b[i+1] == '\n' && b[i+2] == '\r' && b[i+3] == '\n' {
			return i
		}
	}
	return -1
}

func bridgeExtractToken(rawPath string) string {
	seg := strings.TrimPrefix(rawPath, "/")
	if idx := strings.IndexByte(seg, '/'); idx >= 0 {
		seg = seg[:idx]
	}
	if seg == "" {
		return ""
	}
	for _, c := range seg {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' && c != '-' {
			return ""
		}
	}
	return seg
}

func bridgeReply502(conn net.Conn) {
	fmt.Fprintf(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
}
