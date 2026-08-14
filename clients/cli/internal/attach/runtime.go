// Package attach is the interactive attach runtime: it turns a subscribed
// session into a local terminal and back again.
//
// Its discipline is easier to state as a list of things it must never do. It
// never enters raw mode before the subscription is established, never leaves
// the terminal raw on any exit path, never sends a termination, never forwards
// a byte typed after the detach key, and never invents a reason for a departure
// that did not carry one.
package attach

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

// Departure names how an attach ended. The three the experience contract
// requires to be distinguishable — an explicit detach, a transport loss, and a
// server-side session end — are three values here, and nothing collapses them.
type Departure int

const (
	// DepartedByDetach is the operator pressing the detach key. The session
	// keeps running.
	DepartedByDetach Departure = iota
	// DepartedBySessionEnd is the session reaching a terminal lifecycle state
	// while attached. It is read from a published transition, not inferred from
	// silence.
	DepartedBySessionEnd
	// DepartedByTransportLoss is the connection ending. Reason carries what the
	// server published, or ReasonUnattributed when it published nothing.
	DepartedByTransportLoss
	// DepartedBySignal is a catchable terminating signal. The session keeps
	// running: a signal to this process says nothing about the session.
	DepartedBySignal
	// DepartedByInputEnd is the local terminal input stream ending.
	DepartedByInputEnd
)

// Result is why the attach ended.
type Result struct {
	Departure Departure
	// Reason is set for DepartedByTransportLoss. It is the value the plane
	// published; ReasonUnattributed means the connection ended saying nothing,
	// which is a statement that the cause is unknown rather than a guess at it.
	Reason gateway.CloseReason
}

// Failure is the error a departure should be reported as, or nil when the
// attach ended without anything going wrong.
//
// The classification is read off the reason the plane published and nothing
// else. A detach, a session end and a signal are all ordinary endings and carry
// no failure at all; a transport loss carries whatever the plane said, and a
// loss that said nothing carries ReasonUnattributed — which is a statement that
// the cause is unknown, not a guess at which known cause it was.
func (result Result) Failure() error {
	if result.Departure != DepartedByTransportLoss {
		return nil
	}
	return gateway.FromCloseReason(result.Reason, "")
}

// Options configure one attach.
type Options struct {
	// Ref is the session, together with the server that owns it.
	Ref gateway.SessionRef
	// Socket is an already-dialed lifecycle connection.
	Socket gateway.Socket
	// ClientInstanceID is the identity the server assigned when the ticket was
	// minted. Every lifecycle correlation carries it.
	ClientInstanceID string
	// Terminal is the local terminal seam.
	Terminal tty.Terminal
	// Out is where surface output is written.
	Out io.Writer
	// DetachByte is the single byte that detaches.
	DetachByte byte
	// Signals delivers catchable signals. Production wires os/signal; a test
	// sends the value directly, which is what makes the signal exit path
	// testable at all.
	Signals <-chan os.Signal
	// RenewInterval overrides the lease renewal period.
	RenewInterval time.Duration
}

// event is an internal reason to stop.
type event struct {
	departure Departure
	reason    gateway.CloseReason
}

// Run attaches until the session, the operator, the connection or a signal ends
// it, and returns having restored the terminal.
//
// Restoring before returning is what FR-014 means by "restore the terminal
// first and then present a reason": the caller prints the reason, and by then
// the shell already echoes again. A caller that had to remember to restore
// would eventually forget on one path.
func Run(ctx context.Context, options Options) (Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	session := newSession(options)
	go session.readFrames(ctx)

	// The subscription is established before anything touches the terminal.
	// A refusal here — capacity, a session that is gone, a lease the daemon
	// will not admit — must leave the invoking terminal exactly as it was.
	if err := session.subscribe(ctx); err != nil {
		return Result{}, err
	}

	if err := options.Terminal.MakeRaw(); err != nil {
		return Result{}, err
	}
	defer session.restore()

	session.startPumps(ctx)
	defer session.stopPumps()

	return session.wait(ctx), nil
}

// session is one attach in progress.
type session struct {
	options Options

	writeMu  sync.Mutex
	revision atomic.Uint64

	geometryMu sync.Mutex
	cols, rows int

	output     *outputPump
	input      *inputPump
	renewer    *leaseRenewer
	resizer    *resizeForwarder
	events     chan event
	subscribed chan error

	established atomic.Bool
	restoreOnce sync.Once
}

func newSession(options Options) *session {
	current := &session{
		options:    options,
		output:     newOutputPump(options.Out),
		events:     make(chan event, 4),
		subscribed: make(chan error, 1),
	}
	return current
}

// send serialises every outbound frame. The renewer, the input pump and the
// resize forwarder all write, and a WebSocket has one frame at a time.
func (current *session) send(frame []byte) error {
	current.writeMu.Lock()
	defer current.writeMu.Unlock()
	return current.options.Socket.Write(context.Background(), frame)
}

func (current *session) geometry() (int, int) {
	current.geometryMu.Lock()
	defer current.geometryMu.Unlock()
	return current.cols, current.rows
}

func (current *session) setGeometry(cols, rows int) {
	current.geometryMu.Lock()
	current.cols, current.rows = cols, rows
	current.geometryMu.Unlock()
}

// subscribeRequestID names the opening publication so its answer can be told
// from a renewal's.
const subscribeRequestID = "attach-subscribe"

// subscribe publishes the desired subscription and waits for a terminal
// outcome.
//
// Geometry is declared here rather than sent afterwards because the server
// requires it: a subscription without a size is refused, and one with an
// out-of-range size is silently discarded. Clamping before sending is what
// turns "the session renders at the wrong width and nobody knows why" into a
// request the server accepts.
func (current *session) subscribe(ctx context.Context) error {
	cols, rows, err := current.options.Terminal.Size()
	if err != nil {
		return err
	}
	cols, rows = clampGeometry(cols, rows)
	current.setGeometry(cols, rows)

	frame := gateway.EncodeLifecycleDesired(
		gateway.Correlation{
			ClientInstanceID:     current.options.ClientInstanceID,
			ConnectionGeneration: 1,
			ClientRevision:       current.revision.Add(1),
		},
		current.options.Ref.SessionID(), cols, rows, true, subscribeRequestID,
	)
	if err := current.send(frame); err != nil {
		return gateway.FromCloseReason(gateway.ReasonBackendUnavailable, "")
	}

	select {
	case <-ctx.Done():
		return gateway.FromCloseReason(gateway.ReasonGoingAway, "")
	case err := <-current.subscribed:
		if err != nil {
			return err
		}
		current.established.Store(true)
		return nil
	}
}

func (current *session) startPumps(ctx context.Context) {
	go current.output.run()

	current.input = newInputPump(
		current.options.Terminal.Reader(), current.send,
		current.options.Ref.SessionID(), current.options.DetachByte,
	)
	go current.input.run()

	current.renewer = newLeaseRenewer(
		current.send, current.options.Ref.SessionID(), current.options.ClientInstanceID,
		&current.revision, current.geometry, current.options.RenewInterval,
	)
	go current.renewer.run()

	current.resizer = newResizeForwarder(
		current.options.Terminal, current.send, current.options.Ref.SessionID(),
		current.setGeometry,
	)
	go current.resizer.run()
	_ = ctx
}

func (current *session) stopPumps() {
	current.renewer.close()
	current.resizer.close()
	current.output.stop()
}

// wait blocks until something ends the attach.
func (current *session) wait(ctx context.Context) Result {
	for {
		select {
		case <-ctx.Done():
			return current.depart(Result{
				Departure: DepartedByTransportLoss, Reason: gateway.ReasonGoingAway,
			})
		case signalled := <-current.options.Signals:
			if result, done := current.handleSignal(signalled); done {
				return current.depart(result)
			}
		case inputEvent := <-current.input.events:
			if inputEvent == inputDetached {
				return current.depart(Result{Departure: DepartedByDetach})
			}
			return current.depart(Result{Departure: DepartedByInputEnd})
		case err := <-current.renewer.failures:
			_ = err
			return current.depart(Result{
				Departure: DepartedByTransportLoss,
				Reason:    current.options.Socket.CloseReason(),
			})
		case ended := <-current.events:
			return current.depart(Result{Departure: ended.departure, Reason: ended.reason})
		}
	}
}

// handleSignal decides whether a signal ends the attach.
//
// Suspend and resume do not: they hand the terminal back to the shell and take
// it again, and the subscription outlives both. Everything else catchable does,
// and it departs without sending a termination — a signal to this process is
// not a statement about the session.
func (current *session) handleSignal(received os.Signal) (Result, bool) {
	if isSuspend(received) {
		_ = current.options.Terminal.Restore()
		suspendProcess()
		_ = current.options.Terminal.MakeRaw()
		return Result{}, false
	}
	return Result{Departure: DepartedBySignal}, true
}

// depart drains whatever output already arrived, discards the terminal's input
// queue, and restores — in that order.
//
// The order is the contract. Discarding after restoring would let the driver
// hand the shell whatever was typed after the detach key, which is how a
// half-typed command becomes an executed one.
func (current *session) depart(result Result) Result {
	current.output.drain()
	if result.Departure == DepartedByDetach {
		_ = current.options.Terminal.DiscardInputQueue()
	}
	current.restore()
	return result
}

// restore is the single restore path. Every exit runs it, and running it twice
// is harmless, which is what lets the deferred call and a signal path coexist.
func (current *session) restore() {
	current.restoreOnce.Do(func() { _ = current.options.Terminal.Restore() })
}

// readFrames is the socket reader. It never writes to the terminal itself: it
// hands payloads to the output pump so that a blocked screen cannot stall frame
// decoding, lease renewal, or the departure it is about to report.
func (current *session) readFrames(ctx context.Context) {
	for {
		raw, err := current.options.Socket.Read(ctx)
		if err != nil {
			current.reportTransportLoss()
			return
		}
		frame, decodeErr := gateway.DecodeFrame(raw)
		if decodeErr != nil {
			continue
		}
		if current.dispatch(frame) {
			return
		}
	}
}

// dispatch handles one frame and reports whether the reader should stop.
func (current *session) dispatch(frame gateway.Frame) bool {
	switch typed := frame.(type) {
	case gateway.OutputFrame:
		// The lifecycle socket multiplexes every session this connection
		// subscribed to. Without this check, the brief overlap around a
		// subscription change leaks another session's output onto this
		// terminal — which is not a cosmetic problem when the other session is
		// printing a password prompt.
		if typed.SessionID != current.options.Ref.SessionID() {
			return false
		}
		current.output.enqueue(typed.Data)
	case gateway.ViewUpdateFrame:
		if current.sessionEnded(typed.Sessions) {
			current.report(event{departure: DepartedBySessionEnd})
			return true
		}
	case gateway.HelloFrame:
		if current.sessionEnded(typed.Sessions) {
			current.report(event{departure: DepartedBySessionEnd})
			return true
		}
	case gateway.ControlFrame:
		current.report(event{departure: DepartedByTransportLoss, reason: typed.Reason})
		return true
	case gateway.ResponseFrame:
		return current.dispatchResponse(typed)
	case gateway.LifecycleOutcomeFrame:
		return current.dispatchOutcome(typed)
	}
	return false
}

func (current *session) dispatchResponse(response gateway.ResponseFrame) bool {
	if response.OK {
		return false
	}
	failure := gateway.FromResponseCode(response.Code, response.Message)
	if !current.established.Load() {
		current.reportSubscribe(failure)
		return true
	}
	// A refusal after establishment means the lease was not renewed. The
	// subscription is gone whether or not the socket is.
	current.report(event{departure: DepartedByTransportLoss, reason: failure.Reason})
	return true
}

func (current *session) dispatchOutcome(outcome gateway.LifecycleOutcomeFrame) bool {
	if !gateway.IsTerminalOutcome(outcome.Status) {
		// accepted and waiting are progress reports, not establishment.
		// Treating either as established would put the terminal into raw mode
		// for a subscription the daemon has not applied.
		return false
	}
	if !current.established.Load() {
		if gateway.IsEstablishedOutcome(outcome.Status) {
			current.reportSubscribe(nil)
			return false
		}
		current.reportSubscribe(gateway.FromCloseReason(
			gateway.ReasonInvalidRequest, "the server "+outcome.Status+" the subscription"))
		return true
	}
	if gateway.IsEstablishedOutcome(outcome.Status) {
		return false
	}
	current.report(event{
		departure: DepartedByTransportLoss, reason: gateway.ReasonBackendUnavailable,
	})
	return true
}

// sessionEnded reports whether this session reached a terminal lifecycle state.
//
// It reads a published transition. Before the execution plane retained ended
// sessions, the only signal was a row disappearing, which is indistinguishable
// from a session that was never there and forced every client to keep private
// memory of what it had seen.
func (current *session) sessionEnded(sessions []gateway.SessionInfo) bool {
	for _, published := range sessions {
		if published.ID != current.options.Ref.SessionID() {
			continue
		}
		return published.Status.Lifecycle == "stopped"
	}
	return false
}

func (current *session) reportTransportLoss() {
	reason := current.options.Socket.CloseReason()
	if !current.established.Load() {
		current.reportSubscribe(gateway.FromCloseReason(reason, ""))
		return
	}
	current.report(event{departure: DepartedByTransportLoss, reason: reason})
}

func (current *session) report(ended event) {
	select {
	case current.events <- ended:
	default:
	}
}

func (current *session) reportSubscribe(err error) {
	select {
	case current.subscribed <- err:
	default:
	}
}

// ErrTerminalRequired is returned when the caller supplied no terminal.
var ErrTerminalRequired = errors.New("attach requires a terminal")

// OpenTerminal opens the local terminal for an attach.
//
// It is here rather than at the call site because the ordering it guarantees is
// part of this package's contract: when standard input is not a terminal, the
// failure happens before any mode change and before any subscription exists, so
// the invoking terminal is left exactly as it was and the target session is
// untouched. A caller that opened the socket first and discovered the pipe
// afterwards would already have taken a subscription it has to give back.
func OpenTerminal(opener tty.Opener) (tty.Terminal, error) {
	if opener == nil {
		return nil, ErrTerminalRequired
	}
	terminal, err := opener()
	if err != nil {
		return nil, err
	}
	return terminal, nil
}
