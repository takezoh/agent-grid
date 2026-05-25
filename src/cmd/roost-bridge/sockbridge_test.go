package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBridgeHandler_routes verifies that a valid session ID in the URL path
// causes the handler to proxy the connection to the matching unix socket.
func TestBridgeHandler_routes(t *testing.T) {
	dir := t.TempDir()

	ln, err := net.Listen("unix", dir+"/test-session.sock")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	backendDone := make(chan struct{})
	go func() {
		defer close(backendDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 512)
		conn.Read(buf)                                                          //nolint:errcheck
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello")) //nolint:errcheck
	}()

	srv := httptest.NewServer(newBridgeHandler(dir, "test-", ".sock"))
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "GET /session HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n") //nolint:errcheck

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", string(body), "hello")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	<-backendDone
}

// TestBridgeHandler_invalidSessionID verifies that malformed session IDs return 400.
func TestBridgeHandler_invalidSessionID(t *testing.T) {
	srv := httptest.NewServer(newBridgeHandler(t.TempDir(), "p-", ".sock"))
	defer srv.Close()

	cases := []struct{ path, desc string }{
		{"/", "empty session ID"},
		{"/foo/bar", "session ID with slash"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			conn, err := net.Dial("tcp", srv.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n", tc.path) //nolint:errcheck
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("path %q: status = %d, want 400", tc.path, resp.StatusCode)
			}
		})
	}
}

// TestBridgeHandler_backendUnavailable verifies that a missing unix socket returns 503.
func TestBridgeHandler_backendUnavailable(t *testing.T) {
	srv := httptest.NewServer(newBridgeHandler(t.TempDir(), "p-", ".sock"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nosuchsession") //nolint:noctx
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}
