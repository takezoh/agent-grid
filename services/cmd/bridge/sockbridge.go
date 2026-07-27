package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// runSockBridge implements the "sockbridge" subcommand: it forwards every TCP
// connection on -listen to the fixed unix socket at -socket. Used by credproxy
// providers to expose an in-container UDS over a loopback TCP port.
func runSockBridge(args []string) error {
	fs := flag.NewFlagSet("sockbridge", flag.ContinueOnError)
	listen := fs.String("listen", "", "TCP address to listen on (required)")
	socket := fs.String("socket", "", "fixed unix socket path (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *listen == "" {
		return fmt.Errorf("sockbridge: -listen is required")
	}
	if *socket == "" {
		return fmt.Errorf("sockbridge: -socket is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

func bridgeForward(tcp net.Conn, socketPath string) {
	defer tcp.Close()
	unix, err := net.Dial("unix", socketPath)
	if err != nil {
		slog.Warn("sockbridge: dial unix failed", "socket", socketPath, "err", err)
		return
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
