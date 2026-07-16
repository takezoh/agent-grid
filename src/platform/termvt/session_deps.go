package termvt

import (
	"io"
	"os"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Emulator is the subset of *vt.Emulator that Session uses. The interface
// exists so tests can drive Session deterministically through
// NewSessionWithDeps without spawning a real pty — fakes return canned
// Render / Read / Write results and capture OSC handler registrations.
//
// Note CloseInputPipe instead of io.Closer: vt.Emulator.Close() writes an
// internal `closed` boolean field that vt.Emulator.Read() reads without
// synchronization, so the race detector flags a benign-but-real race on
// every shutdown. Closing only the input pipe wakes a parked Read() with
// io.EOF via the io.Pipe contract without touching the racy field.
//
// ReattachSnapshot is intentionally opaque at this boundary: x/vt owns row
// provenance, semantic reflow, ANSI rendering, and cursor restoration.
// SetScrollbackSize bounds retained history; 0 keeps x/vt's default.
type Emulator interface {
	io.Writer                                           // shell output bytes go in
	io.Reader                                           // CSI reply bytes come out — drained back into the pty
	Render() string                                     // rendered grid for reattach snapshots
	Resize(cols, rows int)                              // grid dimension change
	SetCallbacks(cb vt.Callbacks)                       // title / bell hooks
	RegisterOscHandler(code int, handler vt.OscHandler) // OSC 9 / 133 hooks
	CloseInputPipe() error                              // shutdown signal — unblocks Read without racing
	SetScrollbackSize(maxLines int)                     // configure scrollback depth
	ReattachSnapshot() ([]byte, error)                  // semantic history + grid + cursor as opaque ANSI
}

// PTY is the subset of *os.File + pty.Setsize that Session needs. Same
// rationale as Emulator: tests substitute an io.Pipe-backed fake so the
// actor loop's read / write paths can be driven without a real terminal.
type PTY interface {
	io.ReadWriteCloser
	SetSize(cols, rows int) error
}

// emulatorFor returns the production emulator with all OSC / callback wiring
// matched to the runtime contract.
func emulatorFor(cols, rows int) Emulator {
	return realEmulator{Emulator: vt.NewEmulator(cols, rows)}
}

// realEmulator wraps *vt.Emulator and adds CloseInputPipe. Embedding the
// pointer satisfies the io.Reader/Writer + Render/Resize/SetCallbacks/
// RegisterOscHandler + SetScrollbackSize + ReattachSnapshot methods promoted
// from *vt.Emulator.
type realEmulator struct {
	*vt.Emulator
}

func (e realEmulator) CloseInputPipe() error {
	if c, ok := e.InputPipe().(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// realPTY wraps an *os.File (the pty master fd) and adapts pty.Setsize to the
// interface's (cols, rows) shape.
type realPTY struct{ *os.File }

func (p realPTY) SetSize(cols, rows int) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}
