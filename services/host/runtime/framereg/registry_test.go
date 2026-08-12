package framereg_test

import (
	"sync"
	"testing"

	"github.com/takezoh/agent-grid/services/host/runtime/framereg"
)

func TestRegisterAndLookup(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "token-abc")

	got, ok := reg.Lookup("token-abc")
	if !ok || got != "frame1" {
		t.Fatalf("Lookup: got (%q, %v), want (frame1, true)", got, ok)
	}
}

func TestLookupUnknownToken(t *testing.T) {
	reg := framereg.New()
	if _, ok := reg.Lookup("no-such-token"); ok {
		t.Fatal("Lookup of unknown token should return false")
	}
}

func TestRegisterReplacesOldToken(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "old-token")
	reg.Register("frame1", "new-token")

	if _, ok := reg.Lookup("old-token"); ok {
		t.Error("old token should have been removed")
	}
	got, ok := reg.Lookup("new-token")
	if !ok || got != "frame1" {
		t.Fatalf("new token lookup: got (%q, %v)", got, ok)
	}
}

// TestRegisterDetectsTokenRebindAcrossFrames guards the 029 F8 fix: if a
// token already maps to a different frame, the second Register must clear
// the old frame's frameToToken entry (no orphan) and the new mapping must
// stand. Without the cleanup the orphaned entry would survive until the old
// frame's Delete, returning stale tokens via internal iteration.
func TestRegisterDetectsTokenRebindAcrossFrames(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame-a", "shared-tok")
	reg.Register("frame-b", "shared-tok") // F8: rebind to a new frame

	// Forward lookup: latest binding wins.
	if got, ok := reg.Lookup("shared-tok"); !ok || got != "frame-b" {
		t.Errorf("Lookup(shared-tok) = (%q, %v), want (frame-b, true)", got, ok)
	}
	// Reverse: frame-a must NOT still be presenting itself as a token holder.
	// Delete-then-reregister-shared-tok is the realistic warm-token reuse path,
	// and the old frame should leave no shadow entry.
	reg.Delete("frame-a") // cheapest portable assertion: Delete on a "tokenless" frame must be a no-op for the rebind.
	if got, ok := reg.Lookup("shared-tok"); !ok || got != "frame-b" {
		t.Errorf("after Delete(frame-a), Lookup(shared-tok) = (%q, %v); rebind to frame-b must survive", got, ok)
	}
}

func TestDeleteRemovesToken(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "tok1")
	reg.Delete("frame1")

	if _, ok := reg.Lookup("tok1"); ok {
		t.Error("token should be gone after Delete")
	}
}

// TestConcurrentReadWrite verifies the -race detector is satisfied: one
// writer goroutine (event loop) registers tokens while many reader goroutines
// (frame-messaging handlers) call Lookup concurrently.
func TestConcurrentReadWrite(t *testing.T) {
	reg := framereg.New()
	const readers = 8
	const iters = 200

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Go(func() {
		for range iters {
			reg.Register("frame", "token")
		}
		close(stop)
	})

	for range readers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					reg.Lookup("token")
				}
			}
		})
	}
	wg.Wait()
}
