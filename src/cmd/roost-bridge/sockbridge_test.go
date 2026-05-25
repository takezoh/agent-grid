package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestBridgeExtractToken(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/session", "session"},
		{"/abc-123_X", "abc-123_X"},
		{"/sess/extra", "sess"}, // first path segment only
		{"/", ""},               // empty token
		{"", ""},                // empty path
		{"/bad token", ""},      // space is invalid
		{"/has.dot", ""},        // '.' is invalid
	}
	for _, c := range cases {
		if got := bridgeExtractToken(c.in); got != c.want {
			t.Errorf("bridgeExtractToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

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

// TestBridgeRouteConn_routes verifies routing mode: a valid session token in the
// URL path selects the per-session unix socket, the request line is rewritten to
// "/", and bytes proxy bidirectionally.
func TestBridgeRouteConn_routes(t *testing.T) {
	dir := t.TempDir()
	backend, err := net.Listen("unix", dir+"/cx-session.sock")
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer backend.Close()

	gotLine := make(chan string, 1)
	go func() {
		c, err := backend.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		br := bufio.NewReader(c)
		line, _ := br.ReadString('\n')
		gotLine <- line
		for { // drain remaining request headers
			l, err := br.ReadString('\n')
			if err != nil || l == "\r\n" || l == "\n" {
				break
			}
		}
		io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nhi") //nolint:errcheck
	}()

	client, server := tcpPair(t)
	defer client.Close()
	go bridgeRouteConn(server, dir, "cx-", ".sock")

	fmt.Fprintf(client, "GET /session HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n") //nolint:errcheck
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hi" {
		t.Errorf("body = %q, want %q", string(body), "hi")
	}

	line := <-gotLine
	if !strings.HasPrefix(line, "GET / HTTP/1.1") {
		t.Errorf("backend first line = %q, want path rewritten to %q", line, "GET / HTTP/1.1")
	}
}

// TestBridgeRouteConn_invalidToken verifies a missing/invalid token returns 502.
func TestBridgeRouteConn_invalidToken(t *testing.T) {
	client, server := tcpPair(t)
	defer client.Close()
	go bridgeRouteConn(server, t.TempDir(), "cx-", ".sock")

	fmt.Fprintf(client, "GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n") //nolint:errcheck
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// TestBridgeRouteConn_backendUnavailable verifies a valid token whose socket is
// missing returns 502.
func TestBridgeRouteConn_backendUnavailable(t *testing.T) {
	client, server := tcpPair(t)
	defer client.Close()
	go bridgeRouteConn(server, t.TempDir(), "cx-", ".sock")

	fmt.Fprintf(client, "GET /missing HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n") //nolint:errcheck
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// TestBridgeForward_fixedSocket verifies fixed-socket mode forwards every TCP
// connection to one unix socket with no HTTP parsing (the mode used by the
// per-provider container bridges that origin's routing-only build dropped).
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
