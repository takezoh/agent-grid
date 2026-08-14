package attach

import (
	"io"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
)

// input.go is the keyboard-to-session pump.
//
// Two rules shape it. Bytes go to the remote PTY in read order and unmodified,
// including Ctrl-C: a client that trapped Ctrl-C locally would kill itself
// instead of interrupting the agent's current operation. And the detach byte
// splits the buffer rather than being filtered out of it, because everything
// after it belongs to the shell the operator is returning to — or to nobody.

// inputEvent is what the pump reports back to the runtime.
type inputEvent int

const (
	// inputDetached means the configured detach byte was consumed.
	inputDetached inputEvent = iota
	// inputEnded means the terminal input stream ended or failed.
	inputEnded
)

// inputPump reads the terminal and sends surface input frames.
type inputPump struct {
	reader     io.Reader
	send       func(frame []byte) error
	sessionID  string
	detachByte byte
	events     chan inputEvent
	// pending holds an incomplete trailing UTF-8 sequence between reads. The
	// input payload travels as a JSON string, so a partial sequence would be
	// replaced by U+FFFD and reach the PTY as different bytes than were typed.
	pending []byte
}

func newInputPump(
	reader io.Reader, send func([]byte) error, sessionID string, detachByte byte,
) *inputPump {
	return &inputPump{
		reader: reader, send: send, sessionID: sessionID, detachByte: detachByte,
		events: make(chan inputEvent, 1),
	}
}

// run pumps until the detach byte, an input error, or a send failure.
//
// It returns as soon as the detach byte is consumed. Everything the operator
// types after that point is not read at all, which is half of FR-010; the other
// half is the terminal driver's own queue, which the runtime discards through
// the tty seam before restoring cooked mode.
func (pump *inputPump) run() {
	buffer := make([]byte, 4096)
	for {
		read, err := pump.reader.Read(buffer)
		if read > 0 {
			forward, detached := splitAtDetachByte(buffer[:read], pump.detachByte)
			if err := pump.forward(forward); err != nil {
				pump.report(inputEnded)
				return
			}
			if detached {
				pump.report(inputDetached)
				return
			}
		}
		if err != nil {
			pump.report(inputEnded)
			return
		}
	}
}

// forward sends the complete part of what was read, holding any incomplete
// trailing sequence for the next read.
func (pump *inputPump) forward(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	complete, pending := gateway.SplitCompleteUTF8(append(pump.pending, data...))
	pump.pending = pending
	if len(complete) == 0 {
		return nil
	}
	return pump.send(gateway.EncodeInput(pump.sessionID, complete))
}

func (pump *inputPump) report(event inputEvent) {
	select {
	case pump.events <- event:
	default:
	}
}

// splitAtDetachByte returns the bytes before the first detach byte and whether
// one was found.
//
// The detach byte is consumed rather than forwarded, and everything after it in
// the same read is dropped here: it was typed after the operator decided to
// leave, so it belongs neither to the remote session nor — once the driver
// queue is discarded — to the returning shell.
func splitAtDetachByte(data []byte, detachByte byte) (forward []byte, detached bool) {
	for index, character := range data {
		if character == detachByte {
			return data[:index], true
		}
	}
	return data, false
}
