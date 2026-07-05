package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/worker"
	"github.com/takezoh/agent-reactor/client/state"
)

type testQuiesceJobInput struct{}

func (testQuiesceJobInput) JobKind() string { return "test_quiesce" }

func TestQuiesceWaitsForPendingSpawns(t *testing.T) {
	t.Parallel()

	r := New(Config{})
	r.pendingSpawns["frame-1"] = struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = r.Run(ctx) }()
	defer func() {
		cancel()
		<-r.Done()
	}()

	if err := r.TestQuiesce(20 * time.Millisecond); err == nil {
		t.Fatal("TestQuiesce unexpectedly reported drained with pending spawn")
	}
}

func TestQuiesceWaitsForWorkerJobs(t *testing.T) {
	t.Parallel()

	pool := worker.NewPool(context.Background(), 1)
	defer pool.Stop()

	r := New(Config{Pool: pool})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = r.Run(ctx) }()
	defer func() {
		cancel()
		<-r.Done()
	}()

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	go func() {
		worker.Submit(pool, state.JobID(1), testQuiesceJobInput{}, func(ctx context.Context, _ testQuiesceJobInput) (string, error) {
			close(started)
			<-release
			return "ok", nil
		})
		close(done)
	}()
	<-done
	<-started

	quiesced := make(chan error, 1)
	go func() { quiesced <- r.TestQuiesce(100 * time.Millisecond) }()

	select {
	case err := <-quiesced:
		t.Fatalf("TestQuiesce returned early while worker job was in flight: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-quiesced:
		if err != nil {
			t.Fatalf("TestQuiesce after worker release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("TestQuiesce did not complete after worker release")
	}
}
