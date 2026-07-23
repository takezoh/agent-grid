package runtimetest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/runtime"
	"github.com/takezoh/agent-grid/host/state"
)

const (
	defaultTickInterval = time.Hour
	// Wait/Quiesce poll and return as soon as the condition holds, so a large
	// deadline costs nothing when green. 2s deadlines expired under saturated
	// CI-style parallel package runs (-race batch); 10s absorbs that load skew.
	defaultWaitTimeout    = 10 * time.Second
	defaultQuiesceTimeout = 10 * time.Second
)

type options struct {
	cfg            runtime.Config
	waitTimeout    time.Duration
	quiesceTimeout time.Duration
}

// Option customises the harness runtime or polling timeouts.
type Option func(*options)

func WithBackend(backend runtime.FrameBackend) Option {
	return func(o *options) { o.cfg.Backend = backend }
}

func WithPersist(persist runtime.PersistBackend) Option {
	return func(o *options) { o.cfg.Persist = persist }
}

func WithEventLog(eventLog runtime.EventLogBackend) Option {
	return func(o *options) { o.cfg.EventLog = eventLog }
}

func WithToolLog(toolLog runtime.ToolLogBackend) Option {
	return func(o *options) { o.cfg.ToolLog = toolLog }
}

func WithWatcher(watcher runtime.FSWatcher) Option {
	return func(o *options) { o.cfg.Watcher = watcher }
}

func WithTickInterval(interval time.Duration) Option {
	return func(o *options) { o.cfg.TickInterval = interval }
}

func WithWaitTimeout(timeout time.Duration) Option {
	return func(o *options) { o.waitTimeout = timeout }
}

func WithQuiesceTimeout(timeout time.Duration) Option {
	return func(o *options) { o.quiesceTimeout = timeout }
}

// Harness runs a real Runtime loop with test-configurable backends and
// deterministic polling helpers.
type Harness struct {
	runtime        *runtime.Runtime
	waitTimeout    time.Duration
	quiesceTimeout time.Duration
}

func New(t *testing.T, opts ...Option) *Harness {
	t.Helper()

	o := options{
		cfg: runtime.Config{
			TickInterval: defaultTickInterval,
		},
		waitTimeout:    defaultWaitTimeout,
		quiesceTimeout: defaultQuiesceTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}

	r := runtime.New(o.cfg)
	ctx, cancel := context.WithCancel(context.Background())
	h := &Harness{
		runtime:        r,
		waitTimeout:    o.waitTimeout,
		quiesceTimeout: o.quiesceTimeout,
	}

	go func() { _ = r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("runtimetest: runtime did not stop within 2s")
		}
	})

	h.Quiesce(t)
	return h
}

func (h *Harness) Runtime() *runtime.Runtime { return h.runtime }

func (h *Harness) Enqueue(t *testing.T, ev state.Event) {
	t.Helper()
	if err := h.runtime.TestEnqueue(ev, h.waitTimeout); err != nil {
		t.Fatal(err)
	}
}

func (h *Harness) WaitFor(t *testing.T, pred func(state.State) bool) state.State {
	t.Helper()
	snapshot, err := h.waitForSnapshot(pred)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func (h *Harness) Quiesce(t *testing.T) {
	t.Helper()
	if err := h.runtime.TestQuiesce(h.quiesceTimeout); err != nil {
		t.Fatal(err)
	}
}

func (h *Harness) waitForSnapshot(pred func(state.State) bool) (state.State, error) {
	deadline := time.Now().Add(h.waitTimeout)
	for {
		snapshot := h.runtime.TestPublishedState()
		if pred(snapshot) {
			return snapshot, nil
		}
		if time.Now().After(deadline) {
			return state.State{}, fmt.Errorf("runtimetest: WaitFor timed out after %v; snapshot=%#v", h.waitTimeout, snapshot)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
