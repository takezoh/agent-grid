package main

import (
	"io"
	"net"
	"testing"
)

// tcpPair returns the two ends of a connected loopback TCP connection: the
// dialed client side and the accepted server side.
func tcpPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	type res struct {
		c net.Conn
		e error
	}
	ch := make(chan res, 1)
	go func() {
		c, e := ln.Accept()
		ch <- res{c, e}
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	r := <-ch
	if r.e != nil {
		t.Fatalf("accept: %v", r.e)
	}
	return client, r.c
}

// TestBridgeForward_fixedSocket verifies fixed-socket mode forwards every TCP
// connection to one unix socket with no HTTP parsing (the mode used by the
// per-provider credproxy container bridges).
func TestBridgeForward_fixedSocket(t *testing.T) {
	dir := t.TempDir()
	sock := dir + "/fixed.sock"
	backend, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer backend.Close()

	go func() {
		c, err := backend.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 5)
		if _, err := io.ReadFull(c, buf); err != nil {
			return
		}
		c.Write(append([]byte("echo:"), buf...)) //nolint:errcheck
	}()

	client, server := tcpPair(t)
	defer client.Close()
	go bridgeForward(server, sock)

	io.WriteString(client, "hello") //nolint:errcheck
	if tcp, ok := client.(*net.TCPConn); ok {
		tcp.CloseWrite() //nolint:errcheck
	}
	out, _ := io.ReadAll(client)
	if string(out) != "echo:hello" {
		t.Errorf("got %q, want %q", string(out), "echo:hello")
	}
}
