package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const (
	bcastBuf = 256
	readBuf  = 32 * 1024
)

// Session is one pty-backed program whose output is (a) parsed by a server-side
// VT emulator — for OSC handling and reattach snapshots — and (b) fanned out to
// any number of subscribed clients. It is the PoC stand-in for the production
// PtyBackend that will replace tmux.
//
// Single-writer discipline: readLoop is the only writer of the emulator and the
// only producer of frames. mu guards the emulator, the subscriber set, and the
// pending control buffer together, so a Subscribe snapshot (Render) is atomic
// with respect to live writes.
type Session struct {
	ptmx  *os.File
	cmd   *exec.Cmd
	em    *vt.Emulator
	start time.Time

	mu      sync.Mutex
	subs    map[int]chan []byte
	pending [][]byte // control frames produced by OSC handlers during em.Write
	nextID  int
	cols    int
	rows    int
}

// NewSession spawns argv in a pty sized cols×rows and starts the read loop.
func NewSession(argv []string, cols, rows int) (*Session, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // PoC: operator-supplied command
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	s := &Session{
		ptmx:  ptmx,
		cmd:   cmd,
		em:    vt.NewEmulator(cols, rows),
		start: time.Now(),
		subs:  map[int]chan []byte{},
		cols:  cols,
		rows:  rows,
	}
	s.registerOSC()
	go s.readLoop()
	return s, nil
}

// registerOSC wires the server-side OSC "tee": semantic OSC sequences are
// captured here and surfaced to clients as structured control events instead of
// being left in the raw byte stream. Handlers run synchronously inside em.Write
// (under mu) and append to s.pending; readLoop fans them out.
func (s *Session) registerOSC() {
	s.em.SetCallbacks(vt.Callbacks{
		Title: func(t string) { s.pending = append(s.pending, controlFrame("title", 0, t)) },
		Bell:  func() { s.pending = append(s.pending, controlFrame("bell", 0, "")) },
	})
	// OSC 9: desktop notification — captured server-side so it reaches the
	// operating client rather than firing on the server host.
	s.em.RegisterOscHandler(9, func(data []byte) bool {
		s.pending = append(s.pending, controlFrame("osc", 9, oscText(data)))
		return true
	})
	// OSC 133: shell prompt / command markers — drives run-state detection.
	s.em.RegisterOscHandler(133, func(data []byte) bool {
		s.pending = append(s.pending, controlFrame("prompt", 133, oscText(data)))
		return true
	})
}

// oscText drops the leading "<cmd>;" that x/vt includes in the OSC payload.
func oscText(data []byte) string {
	s := string(data)
	if i := strings.IndexByte(s, ';'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func (s *Session) readLoop() {
	buf := make([]byte, readBuf)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.pending = s.pending[:0]
			_, _ = s.em.Write(chunk) // fires OSC handlers → append to s.pending
			for _, cf := range s.pending {
				s.fanout(cf)
			}
			s.fanout(outputFrame(s.elapsed(), chunk))
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	s.mu.Lock()
	s.fanout(controlFrame("exit", 0, ""))
	for id, ch := range s.subs {
		close(ch)
		delete(s.subs, id)
	}
	s.mu.Unlock()
}

// fanout sends a pre-encoded frame to every subscriber. Caller must hold mu.
// PoC backpressure: drop on a full buffer (production must disconnect instead,
// since dropping output corrupts the terminal stream).
func (s *Session) fanout(frame []byte) {
	for id, ch := range s.subs {
		select {
		case ch <- frame:
		default:
			log.Printf("subscriber %d slow, dropping frame", id)
		}
	}
}

// Subscribe registers a client and returns its id plus a frame channel. The
// first frame is a reattach snapshot of the current screen, captured atomically
// with respect to live writes (mu held).
func (s *Session) Subscribe() (int, <-chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	ch := make(chan []byte, bcastBuf)
	ch <- outputFrame(s.elapsed(), []byte(s.em.Render()))
	s.subs[id] = ch
	return id, ch
}

func (s *Session) Unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.subs[id]; ok {
		close(ch)
		delete(s.subs, id)
	}
}

// WriteInput forwards client keystrokes to the pty. Safe to call concurrently
// with the read loop (os.File supports concurrent read/write on a pty).
func (s *Session) WriteInput(b []byte) {
	_, _ = s.ptmx.Write(b)
}

// Resize updates the pty window size and the emulator grid.
func (s *Session) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cols, s.rows = cols, rows
	s.em.Resize(cols, rows)
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.ptmx.Close()
}

func (s *Session) elapsed() float64 { return time.Since(s.start).Seconds() }
