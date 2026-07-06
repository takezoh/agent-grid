package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-grid/client/state"
)

type job struct {
	id  state.JobID
	run func(ctx context.Context) (any, error)
}

type Pool struct {
	ctx         context.Context
	cancel      context.CancelFunc
	jobs        chan job
	results     chan state.Event
	stopped     chan struct{}
	closed      bool
	mu          sync.Mutex
	outstanding atomic.Int64
	// testAfterDequeue blocks the worker between receive and runJob so tests can
	// deterministically observe the dequeue window. Nil in production.
	testAfterDequeue func()
	// testBeforeEnqueue blocks Submit after the job has been admitted but before
	// it reaches the queue, letting tests pin Submit/Stop serialization.
	testBeforeEnqueue func()
	drainOnce         sync.Once
}

func NewPool(parent context.Context, size int) *Pool {
	ctx, cancel := context.WithCancel(parent)
	p := &Pool{
		ctx:     ctx,
		cancel:  cancel,
		jobs:    make(chan job, 64),
		results: make(chan state.Event, 64),
		stopped: make(chan struct{}),
	}
	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.run()
		}()
	}
	go func() {
		wg.Wait()
		close(p.stopped)
	}()
	return p
}

// Submit enqueues a typed job. The runner receives the pool's shutdown
// context so it can cancel in-flight I/O when Stop is called.
func Submit[In, Out any](p *Pool, id state.JobID, input In, runner func(context.Context, In) (Out, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	j := job{
		id:  id,
		run: func(ctx context.Context) (any, error) { return runner(ctx, input) },
	}
	p.outstanding.Add(1)
	if p.testBeforeEnqueue != nil {
		p.testBeforeEnqueue()
	}
	select {
	case p.jobs <- j:
	default:
		p.outstanding.Add(-1)
		slog.Warn("worker: job queue full, dropping",
			"job_id", id, "input", fmt.Sprintf("%T", input))
	}
}

func (p *Pool) Results() <-chan state.Event { return p.results }

// Idle reports whether the pool has no queued, running, or undrained result
// work left. Used by runtimetest quiescence checks.
func (p *Pool) Idle() bool {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return p.outstanding.Load() == 0
	}
	return len(p.results) == 0 && p.outstanding.Load() == 0
}

// Stop cancels the pool context (signalling all runners) and waits up
// to 500 ms for workers to drain. Queued jobs that haven't started are
// discarded. Idempotent.
func (p *Pool) Stop() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.discardQueuedJobsLocked()
	p.mu.Unlock()
	p.cancel()
	select {
	case <-p.stopped:
	case <-time.After(500 * time.Millisecond):
		slog.Warn("worker: stop deadline exceeded, leaking goroutines")
	}
}

func (p *Pool) discardQueuedJobsLocked() {
	p.drainOnce.Do(func() {
		for {
			select {
			case <-p.jobs:
				p.outstanding.Add(-1)
			default:
				return
			}
		}
	})
}

func (p *Pool) run() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case j := <-p.jobs:
			if p.testAfterDequeue != nil {
				p.testAfterDequeue()
			}
			p.runJob(j)
		}
	}
}

func (p *Pool) runJob(j job) {
	defer p.outstanding.Add(-1)
	// Drop jobs that were dequeued after the pool was stopped.
	// This is distinct from the Submit guard (p.closed): a job can sit in
	// the queue and be dequeued by a worker goroutine after Stop has
	// cancelled p.ctx. Checking here ensures such jobs are silently
	// discarded rather than run.
	if p.ctx.Err() != nil {
		return
	}
	result, err := j.run(p.ctx)
	ev := state.EvJobResult{
		JobID:  j.id,
		Result: result,
		Err:    err,
	}
	select {
	case p.results <- ev:
	case <-p.ctx.Done():
	}
}
