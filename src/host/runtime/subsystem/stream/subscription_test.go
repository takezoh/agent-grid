package stream

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
)

func TestReadyWaitsForAllPredicatesAndCommitsOnce(t *testing.T) {
	b, rt := newTestBackend()
	if err := b.registerBoundFrame("f1", "/work", "", "t1", "", ""); err != nil {
		t.Fatal(err)
	}
	b.ActivateFrame("f1")
	if len(rt.events) != 0 {
		t.Fatalf("activation alone emitted %d events", len(rt.events))
	}
	if err := b.subscribeObserver("f1"); err != nil {
		t.Fatal(err)
	}
	b.ActivateFrame("f1")
	if len(rt.events) != 1 || rt.events[0].(state.EvSubsystem).Kind != state.SubsystemSessionReady {
		t.Fatalf("events = %#v, want one SessionReady", rt.events)
	}
}

func TestObserverResumeStaleSuccessCompensatesWithUnsubscribe(t *testing.T) {
	b, rt := newTestBackend()
	if err := b.registerBoundFrame("f1", "/work", "", "t1", "", ""); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	resume := make(chan struct{})
	b.resumeObserver = func(codexclient.ResumeOptions) (codexclient.ThreadSession, error) {
		close(entered)
		<-resume
		return codexclient.ThreadSession{ThreadID: "t1"}, nil
	}
	unsubscribed := make(chan string, 1)
	b.unsubscribeObserver = func(threadID string) (codexclient.ThreadUnsubscribeStatus, error) {
		unsubscribed <- threadID
		return codexclient.ThreadUnsubscribed, nil
	}
	done := make(chan error, 1)
	go func() { done <- b.subscribeObserver("f1") }()
	<-entered
	b.ReleaseFrame("f1")
	close(resume)
	if err := <-done; err != nil {
		t.Fatalf("stale completion: %v", err)
	}
	select {
	case got := <-unsubscribed:
		if got != "t1" {
			t.Fatalf("unsubscribed %q, want t1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("stale successful resume was not compensated")
	}
	if len(rt.events) != 0 {
		t.Fatalf("stale completion emitted %d events", len(rt.events))
	}
}

func TestReleaseFrameUnsubscribesOnceAfterTombstone(t *testing.T) {
	b, _ := newTestBackend()
	if err := b.registerBoundFrame("f1", "/work", "", "t1", "", ""); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	b.frames["f1"].observerSubscribed = true
	b.mu.Unlock()
	var calls atomic.Int32
	done := make(chan struct{}, 1)
	b.unsubscribeObserver = func(threadID string) (codexclient.ThreadUnsubscribeStatus, error) {
		if got := b.frameForThread(threadID); got != "" {
			t.Errorf("routing still live during unsubscribe: %q", got)
		}
		calls.Add(1)
		done <- struct{}{}
		return codexclient.ThreadUnsubscribed, nil
	}
	b.ReleaseFrame("f1")
	b.ReleaseFrame("f1")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unsubscribe was not called")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("unsubscribe calls = %d, want 1", got)
	}
}
