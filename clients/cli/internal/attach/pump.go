package attach

import (
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

// pump.go holds the two periodic senders: the lease renewal and the geometry
// forwarder. Both run on their own goroutines and neither touches the terminal
// output path, which is the whole point of splitting them out.

// DefaultRenewInterval is how often the lease is republished.
//
// The execution plane's lease lives 12 seconds
// (services/host/runtime/terminal_lifecycle_runtime.go). Renewing at a third of
// that leaves room for two consecutive missed renewals — a stalled scheduler, a
// slow link — before the subscription is lost, without making the socket busy
// when nothing is happening.
const DefaultRenewInterval = 4 * time.Second

// leaseRenewer republishes the desired subscription on a fixed schedule.
//
// It exists as a separate goroutine because of one specific failure: an
// implementation that renews inside the output loop stops renewing the moment
// the terminal blocks, and twelve seconds later the operator loses the session
// they were reading. Renewal must not be able to wait on a screen.
type leaseRenewer struct {
	send      func(frame []byte) error
	sessionID string
	instance  string
	revision  *atomic.Uint64
	geometry  func() (int, int)
	interval  time.Duration
	stop      chan struct{}
	failures  chan error
}

func newLeaseRenewer(
	send func([]byte) error,
	sessionID, instance string,
	revision *atomic.Uint64,
	geometry func() (int, int),
	interval time.Duration,
) *leaseRenewer {
	if interval <= 0 {
		interval = DefaultRenewInterval
	}
	return &leaseRenewer{
		send: send, sessionID: sessionID, instance: instance, revision: revision,
		geometry: geometry, interval: interval,
		stop: make(chan struct{}), failures: make(chan error, 1),
	}
}

func (renewer *leaseRenewer) run() {
	ticker := time.NewTicker(renewer.interval)
	defer ticker.Stop()
	for {
		select {
		case <-renewer.stop:
			return
		case <-ticker.C:
			cols, rows := renewer.geometry()
			frame := gateway.EncodeLifecycleDesired(
				gateway.Correlation{
					ClientInstanceID:     renewer.instance,
					ConnectionGeneration: 1,
					// Advancing before sending is what keeps the revision
					// strictly monotonic. The daemon admits a newer publication
					// over an older one by comparing this value, so a reused or
					// decremented revision is a renewal the daemon ignores —
					// and an ignored renewal is a lease that quietly lapses.
					ClientRevision: renewer.revision.Add(1),
				},
				renewer.sessionID, cols, rows, true, "",
			)
			if err := renewer.send(frame); err != nil {
				select {
				case renewer.failures <- err:
				default:
				}
				return
			}
		}
	}
}

func (renewer *leaseRenewer) close() { close(renewer.stop) }

// resizeForwarder turns local size changes into geometry frames.
type resizeForwarder struct {
	terminal  tty.Terminal
	send      func(frame []byte) error
	sessionID string
	current   func(cols, rows int)
	stop      chan struct{}
}

func newResizeForwarder(
	terminal tty.Terminal, send func([]byte) error, sessionID string, current func(int, int),
) *resizeForwarder {
	return &resizeForwarder{
		terminal: terminal, send: send, sessionID: sessionID, current: current,
		stop: make(chan struct{}),
	}
}

// run forwards the latest size after every burst of changes.
//
// Coalescing is deliberate and is the reason the size is re-queried rather than
// carried on the event. Dragging a window edge produces a change per pixel row;
// forwarding each one would queue sizes behind each other and apply a stale one
// last, leaving the session rendering at a width the window no longer has.
func (forwarder *resizeForwarder) run() {
	changes := forwarder.terminal.SizeChanges()
	for {
		select {
		case <-forwarder.stop:
			return
		case <-changes:
			forwarder.drainPending(changes)
			cols, rows, err := forwarder.terminal.Size()
			if err != nil {
				continue
			}
			cols, rows = clampGeometry(cols, rows)
			forwarder.current(cols, rows)
			if err := forwarder.send(gateway.EncodeResize(forwarder.sessionID, cols, rows)); err != nil {
				return
			}
		}
	}
}

// drainPending consumes every change already queued, so one query answers the
// whole burst.
func (forwarder *resizeForwarder) drainPending(changes <-chan struct{}) {
	for {
		select {
		case <-changes:
		default:
			return
		}
	}
}

func (forwarder *resizeForwarder) close() { close(forwarder.stop) }
