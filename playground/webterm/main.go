// Command webterm is a proof-of-concept for the tmux-free, pty-backed remote
// client-server architecture described in
// docs/technical/remote-client/design.md.
//
// It spawns a command in a pty, parses its output with a server-side VT emulator
// (capturing OSC 9/133/title as structured control events), and streams the
// session to any number of browser clients over a WebSocket. A single shared
// session demonstrates multiplexing and reattach (open multiple tabs).
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

//go:embed index.html
var assets embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "listen address")
	cmdStr := flag.String("cmd", defaultShell(), "command to run in the pty (space-split; PoC has no quoting)")
	flag.Parse()

	sess, err := NewSession(strings.Fields(*cmdStr), 80, 24)
	if err != nil {
		log.Fatalf("session: %v", err)
	}
	defer func() { _ = sess.Close() }()

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) { serveWS(sess, w, r) })

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("webterm PoC on http://%s  (cmd=%q) — open in a browser", *addr, *cmdStr)
	log.Fatal(srv.ListenAndServe())
}

func defaultShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}

// serveWS bridges one browser WebSocket to the shared session: it subscribes for
// output/control frames (writer loop) and forwards client input/resize (reader
// goroutine). This is the PoC analogue of the production proto↔ws gateway.
func serveWS(s *Session, w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	id, ch := s.Subscribe()
	defer s.Unsubscribe(id)

	go func() { readInbound(ctx, s, c); cancel() }()

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-ch:
			if !ok {
				return
			}
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return
			}
		}
	}
}

func readInbound(ctx context.Context, s *Session, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var in inbound
		if json.Unmarshal(data, &in) != nil {
			continue
		}
		switch in.K {
		case "i":
			s.WriteInput([]byte(in.D))
		case "r":
			if in.Cols > 0 && in.Rows > 0 {
				_ = s.Resize(in.Cols, in.Rows)
			}
		}
	}
}
