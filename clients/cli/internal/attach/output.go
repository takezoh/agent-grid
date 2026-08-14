package attach

import (
	"io"
	"sync"
)

// output.go is the session-to-screen pump.
//
// It runs on its own goroutine for one reason: a blocked terminal — a paused
// pager, a stopped `less`, a slow ssh link — must not stall anything else this
// process is doing. In particular it must not stall the lease renewal, because
// a lease that lapses ends the subscription and the operator loses the session
// they were watching, for no reason other than that they had scrolled.

// outputPump writes surface payloads to the terminal in the order they arrived.
type outputPump struct {
	writer io.Writer
	frames chan []byte
	done   chan struct{}
	once   sync.Once
}

// outputQueueDepth bounds how far the reader may run ahead of a blocked writer.
//
// It is finite on purpose. An unbounded queue turns a stalled terminal into
// unbounded memory growth, and the backpressure that a bounded queue applies to
// the socket reader is the correct behaviour: the server's own flow control
// takes over, which is what it is there for.
const outputQueueDepth = 256

func newOutputPump(writer io.Writer) *outputPump {
	return &outputPump{
		writer: writer,
		frames: make(chan []byte, outputQueueDepth),
		done:   make(chan struct{}),
	}
}

// run writes queued payloads until the pump is stopped.
//
// Order is preserved by construction: one channel, one writer, no reordering
// step. That is what makes "the current screen appears before any later output"
// true — the snapshot is simply the first payload that arrived, and nothing
// reconstructs or re-sorts anything.
func (pump *outputPump) run() {
	for {
		select {
		case <-pump.done:
			return
		case payload := <-pump.frames:
			if _, err := pump.writer.Write(payload); err != nil {
				return
			}
		}
	}
}

// enqueue queues one payload. It blocks when the writer is behind, which is the
// backpressure path, and gives up when the pump has stopped.
func (pump *outputPump) enqueue(payload []byte) {
	select {
	case pump.frames <- payload:
	case <-pump.done:
	}
}

// stop ends the pump. It is idempotent because more than one exit path reaches
// it.
func (pump *outputPump) stop() {
	pump.once.Do(func() { close(pump.done) })
}

// drain writes whatever is already queued, so output that arrived before a
// clean departure is not thrown away by the departure itself.
func (pump *outputPump) drain() {
	for {
		select {
		case payload := <-pump.frames:
			if _, err := pump.writer.Write(payload); err != nil {
				return
			}
		default:
			return
		}
	}
}
