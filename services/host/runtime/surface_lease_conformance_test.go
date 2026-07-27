package runtime

import (
	"context"
	"testing"
	"time"
)

func TestFakeVsRealSurfaceLeaseReleaseIsIdempotent(t *testing.T) {
	fake := newFakeSurfaceBackend()
	fakeLease, err := fake.AcquireSurface(context.Background(), "fake-frame", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := fakeLease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := fakeLease.Release(); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	remaining := len(fake.subs)
	fake.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("fake subscribers = %d, want 0", remaining)
	}
}

func TestFakeVsRealSurfaceLeaseReleasesPhysicalSubscriber(t *testing.T) {
	backend := NewPtyBackend(0)
	frameID := "surface-lease-conformance"
	if err := backend.SpawnFrame(frameID, "lease", "cat", "", nil, 80, 24); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backend.KillFrame(frameID) }()

	lease, err := backend.AcquireSurface(context.Background(), frameID, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-lease.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("real lease did not produce initial snapshot")
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-lease.Events():
		if ok {
			t.Fatal("real lease channel remained open after release")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("real lease physical release exceeded 100ms")
	}
}
