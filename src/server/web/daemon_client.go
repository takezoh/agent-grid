package web

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
)

// ErrDaemonUnavailable is returned by SendCommand when the DaemonClient is
// not currently connected to the daemon.
var ErrDaemonUnavailable = errors.New("server/web: daemon unavailable")

// defaultMinDelay / defaultMaxDelay define the exponential backoff bounds for
// supervisor reconnect attempts.
const (
	defaultMinDelay = 250 * time.Millisecond
	defaultMaxDelay = 4 * time.Second
)

// dialFunc is the signature used by the supervisor to open a new connection.
type dialFunc func() (*proto.Client, error)

type daemonSubscriber struct {
	events chan proto.ServerEvent
	pushes chan []byte
}

// DaemonClient is a proto.Client wrapper that maintains a persistent
// connection to the server daemon over a Unix socket. A supervisor goroutine
// reconnects on disconnect using full-jitter exponential backoff.
type DaemonClient struct {
	dial     dialFunc
	sockPath string

	mu  sync.RWMutex
	cli *proto.Client // nil while disconnected

	subsMu      sync.RWMutex
	subs        map[chan proto.ServerEvent]*daemonSubscriber
	subsByEvent sync.Map // uintptr → *daemonSubscriber

	health      atomic.Bool
	lastErr     atomic.Pointer[error]
	lastAttempt atomic.Pointer[time.Time]

	stop chan struct{}
	once sync.Once

	minDelay time.Duration
	maxDelay time.Duration
}

// perSubscriberBuf sizes each subscriber's outbox. Slow subscribers are severed
// individually rather than blocking the daemon reader or other subscribers.
const perSubscriberBuf = 64

// NewDaemonClient eagerly dials sockPath then starts the supervisor goroutine
// that watches the connection and reconnects on loss.
func NewDaemonClient(sockPath string) *DaemonClient {
	d := &DaemonClient{
		sockPath: sockPath,
		dial:     func() (*proto.Client, error) { return proto.Dial(sockPath) },
		subs:     make(map[chan proto.ServerEvent]*daemonSubscriber),
		stop:     make(chan struct{}),
		minDelay: defaultMinDelay,
		maxDelay: defaultMaxDelay,
	}
	d.tryDialOnce(1)
	go d.supervise()
	return d
}

// NewDaemonClientWithDialer is a test-only constructor that accepts a custom
// dialer and backoff delays, enabling unit tests to inject net.Pipe
// connections without a real Unix socket.
func NewDaemonClientWithDialer(dialFn func() (*proto.Client, error), minDelay, maxDelay time.Duration) *DaemonClient {
	d := &DaemonClient{
		sockPath: "<dialer>",
		dial:     dialFn,
		subs:     make(map[chan proto.ServerEvent]*daemonSubscriber),
		stop:     make(chan struct{}),
		minDelay: minDelay,
		maxDelay: maxDelay,
	}
	d.tryDialOnce(1)
	go d.supervise()
	return d
}

// Close shuts down the supervisor goroutine and the underlying connection.
func (d *DaemonClient) Close() {
	d.once.Do(func() {
		close(d.stop)
		d.mu.Lock()
		cli := d.cli
		d.cli = nil
		d.mu.Unlock()
		if cli != nil {
			cli.Close()
		}
		d.closeAllSubs()
	})
}

func (d *DaemonClient) Health() bool { return d.health.Load() }

func (d *DaemonClient) LastError() error {
	p := d.lastErr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (d *DaemonClient) LastAttemptAt() time.Time {
	p := d.lastAttempt.Load()
	if p == nil {
		return time.Time{}
	}
	return *p
}

func (d *DaemonClient) SendCommand(ctx context.Context, cmd proto.Command) (proto.Response, error) {
	d.mu.RLock()
	cli := d.cli
	d.mu.RUnlock()
	if cli == nil {
		return nil, ErrDaemonUnavailable
	}
	return cli.Send(ctx, cmd)
}

// SubscribeEvents registers a fresh subscriber and returns its dedicated event
// channel. Use PushChannelFor to obtain the paired push channel for the same
// subscription.
func (d *DaemonClient) SubscribeEvents(ctx context.Context) <-chan proto.ServerEvent {
	events := make(chan proto.ServerEvent, perSubscriberBuf)
	pushes := make(chan []byte, 8)
	sub := &daemonSubscriber{events: events, pushes: pushes}

	d.subsMu.Lock()
	select {
	case <-d.stop:
		d.subsMu.Unlock()
		close(events)
		close(pushes)
		return events
	default:
	}
	d.subs[events] = sub
	d.subsByEvent.Store(reflect.ValueOf(events).Pointer(), sub)
	d.subsMu.Unlock()

	go func() {
		<-ctx.Done()
		d.subsMu.Lock()
		if cur, ok := d.subs[events]; ok && cur == sub {
			delete(d.subs, events)
			d.subsByEvent.Delete(reflect.ValueOf(events).Pointer())
			close(events)
			close(pushes)
		}
		d.subsMu.Unlock()
	}()
	return events
}

// PushChannelFor returns the push-notification channel paired with eventsCh
// from the same SubscribeEvents call.
func (d *DaemonClient) PushChannelFor(eventsCh <-chan proto.ServerEvent) <-chan []byte {
	if v, ok := d.subsByEvent.Load(reflect.ValueOf(eventsCh).Pointer()); ok {
		return v.(*daemonSubscriber).pushes
	}
	ch := make(chan []byte)
	close(ch)
	return ch
}

func (d *DaemonClient) broadcastEvent(ev proto.ServerEvent) {
	d.subsMu.RLock()
	snapshot := make([]*daemonSubscriber, 0, len(d.subs))
	for _, sub := range d.subs {
		snapshot = append(snapshot, sub)
	}
	d.subsMu.RUnlock()

	for _, sub := range snapshot {
		select {
		case sub.events <- ev:
		default:
			d.severSubscriber(sub, "event", ev.EventName())
		}
	}
}

func (d *DaemonClient) broadcastPush(push proto.PushNotification) {
	frame := encodePushFrame(push)
	if frame == nil {
		return
	}
	d.subsMu.RLock()
	snapshot := make([]*daemonSubscriber, 0, len(d.subs))
	for _, sub := range d.subs {
		snapshot = append(snapshot, sub)
	}
	d.subsMu.RUnlock()

	for _, sub := range snapshot {
		select {
		case sub.pushes <- frame:
		default:
			d.severSubscriber(sub, "push", push.Cmd)
		}
	}
}

func (d *DaemonClient) severSubscriber(sub *daemonSubscriber, kind, detail string) {
	slog.Warn("server/web: daemon fan-out severing slow subscriber",
		"kind", kind, "detail", detail)
	d.subsMu.Lock()
	if cur, ok := d.subs[sub.events]; ok && cur == sub {
		delete(d.subs, sub.events)
		d.subsByEvent.Delete(reflect.ValueOf(sub.events).Pointer())
		close(sub.events)
		close(sub.pushes)
	}
	d.subsMu.Unlock()
}

func (d *DaemonClient) closeAllSubs() {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	for _, sub := range d.subs {
		close(sub.events)
		close(sub.pushes)
	}
	d.subs = make(map[chan proto.ServerEvent]*daemonSubscriber)
}

func (d *DaemonClient) tryDialOnce(attempt int) {
	now := time.Now()
	d.lastAttempt.Store(&now)

	cli, err := d.dial()
	if err != nil {
		d.lastErr.Store(&err)
		d.mu.Lock()
		d.cli = nil
		d.mu.Unlock()
		d.health.Store(false)
		return
	}

	d.mu.Lock()
	prev := d.cli
	d.cli = cli
	d.mu.Unlock()
	if prev != nil {
		prev.Close()
	}
	d.health.Store(true)
	slog.Info("server/web: daemon connected", "sock", d.sockPath, "attempt", attempt)
}

func (d *DaemonClient) supervise() {
	attempt := 1
	for {
		select {
		case <-d.stop:
			return
		default:
		}

		d.mu.RLock()
		cli := d.cli
		d.mu.RUnlock()

		if cli == nil {
			attempt = d.reconnectLoop(attempt)
			continue
		}

		fanoutDone := make(chan struct{})
		go d.fanoutEvents(cli, fanoutDone)
		go d.fanoutPushes(cli, fanoutDone)
		select {
		case <-d.stop:
			return
		case <-fanoutDone:
		}

		d.markDown("events channel closed")
		attempt = d.reconnectLoop(attempt)
	}
}

func (d *DaemonClient) reconnectLoop(attempt int) int {
	delay := d.minDelay
	for {
		select {
		case <-d.stop:
			return attempt
		default:
		}

		jitter := time.Duration(rand.Float64() * float64(delay))
		slog.Warn("server/web: backing off before next dial",
			"sock", d.sockPath,
			"attempt", attempt,
			"delay", jitter)

		select {
		case <-d.stop:
			return attempt
		case <-time.After(jitter):
		}

		attempt++
		d.tryDialOnce(attempt)

		if d.health.Load() {
			return attempt
		}

		slog.Warn("server/web: daemon dial failed",
			"sock", d.sockPath,
			"attempt", attempt,
			"err", d.LastError(),
			"delay", jitter)

		delay *= 2
		if delay > d.maxDelay {
			delay = d.maxDelay
		}
	}
}

func (d *DaemonClient) fanoutEvents(cli *proto.Client, done chan<- struct{}) {
	defer close(done)
	src := cli.Events()
	for {
		ev, ok := <-src
		if !ok {
			return
		}
		d.broadcastEvent(ev)
	}
}

func (d *DaemonClient) fanoutPushes(cli *proto.Client, done <-chan struct{}) {
	src := cli.Pushes()
	for {
		select {
		case <-done:
			return
		case push, ok := <-src:
			if !ok {
				return
			}
			d.broadcastPush(push)
		}
	}
}

func (d *DaemonClient) markDown(reason string) {
	d.mu.Lock()
	d.cli = nil
	d.mu.Unlock()

	d.health.Store(false)
	d.closeAllSubs()
	slog.Warn("server/web: daemon disconnected", "reason", reason)
}
