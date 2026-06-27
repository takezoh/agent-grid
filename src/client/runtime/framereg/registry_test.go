package framereg_test

import (
	"sync"
	"testing"

	"github.com/takezoh/agent-reactor/client/runtime/framereg"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
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

// TestRegisterWithMountsDetectsTokenRebind mirrors the F8 fix for the atomic
// register path. Same shape as TestRegisterDetectsTokenRebindAcrossFrames
// but exercises RegisterWithMounts since that's the prod call site after the
// 029 F6 rework.
func TestRegisterWithMountsDetectsTokenRebind(t *testing.T) {
	reg := framereg.New()
	msA := pathmap.Mounts{{Host: "/a", Container: "/work"}}
	msB := pathmap.Mounts{{Host: "/b", Container: "/work"}}
	reg.RegisterWithMounts("frame-a", "shared-tok", msA)
	reg.RegisterWithMounts("frame-b", "shared-tok", msB)

	if got, ok := reg.Lookup("shared-tok"); !ok || got != "frame-b" {
		t.Errorf("Lookup(shared-tok) = (%q, %v), want (frame-b, true)", got, ok)
	}
	if ms, ok := reg.GetMounts("frame-b"); !ok || ms[0].Host != "/b" {
		t.Errorf("frame-b mounts = (%v, %v), want host=/b", ms, ok)
	}
	// frame-a should have been cleared from frameToToken when shared-tok rebound.
	// Re-registering frame-a with a fresh token must not collide with leftovers.
	reg.RegisterWithMounts("frame-a", "fresh-tok", msA)
	if got, ok := reg.Lookup("fresh-tok"); !ok || got != "frame-a" {
		t.Errorf("Lookup(fresh-tok) = (%q, %v), want (frame-a, true) after re-register", got, ok)
	}
}

func TestDeleteRemovesTokenAndMounts(t *testing.T) {
	reg := framereg.New()
	reg.Register("frame1", "tok1")
	reg.StoreMounts("frame1", pathmap.Mounts{{Host: "/h", Container: "/c"}})
	reg.Delete("frame1")

	if _, ok := reg.Lookup("tok1"); ok {
		t.Error("token should be gone after Delete")
	}
	if _, ok := reg.GetMounts("frame1"); ok {
		t.Error("mounts should be gone after Delete")
	}
}

func TestGetMounts(t *testing.T) {
	reg := framereg.New()
	if _, ok := reg.GetMounts("f"); ok {
		t.Fatal("GetMounts on empty reg should return false")
	}
	reg.StoreMounts("f", pathmap.Mounts{{Host: "/h", Container: "/c"}})
	if _, ok := reg.GetMounts("f"); !ok {
		t.Fatal("GetMounts after StoreMounts should return true")
	}
}

func TestRegisterWithMountsAtomic(t *testing.T) {
	reg := framereg.New()
	mounts := pathmap.Mounts{{Host: "/h", Container: "/c"}}
	reg.RegisterWithMounts("f1", "tok1", mounts)

	got, ok := reg.Lookup("tok1")
	if !ok || got != "f1" {
		t.Fatalf("Lookup after RegisterWithMounts: got (%q, %v)", got, ok)
	}
	ms, ok := reg.GetMounts("f1")
	if !ok || len(ms) == 0 {
		t.Fatalf("GetMounts after RegisterWithMounts: got (%v, %v)", ms, ok)
	}

	// Replace with new token — old token should be invalidated.
	reg.RegisterWithMounts("f1", "tok2", nil)
	if _, ok := reg.Lookup("tok1"); ok {
		t.Error("old token should be invalidated after re-register")
	}
	if _, ok := reg.Lookup("tok2"); !ok {
		t.Error("new token should be valid")
	}
}

// TestRegisterWithMountsClearsStaleMounts verifies that re-registering a frame
// with an empty mount set drops the previously stored mounts, so GetMounts does
// not return a stale bind-mount table that would mistranslate hook paths.
func TestRegisterWithMountsClearsStaleMounts(t *testing.T) {
	reg := framereg.New()
	reg.RegisterWithMounts("f1", "tok1", pathmap.Mounts{{Host: "/h", Container: "/c"}})
	if _, ok := reg.GetMounts("f1"); !ok {
		t.Fatal("mounts should be present after first register")
	}

	reg.RegisterWithMounts("f1", "tok2", nil)
	if ms, ok := reg.GetMounts("f1"); ok {
		t.Errorf("stale mounts retained after empty re-register: got %v", ms)
	}
}

// TestConcurrentReadWrite verifies the -race detector is satisfied: one
// writer goroutine (event loop) registers tokens while many reader goroutines
// (container endpoint handlers) call Lookup and GetMounts concurrently.
func TestConcurrentReadWrite(t *testing.T) {
	reg := framereg.New()
	const readers = 8
	const iters = 200

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Go(func() {
		for range iters {
			reg.Register("frame", "token")
			reg.StoreMounts("frame", pathmap.Mounts{{Host: "/h", Container: "/c"}})
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
					reg.GetMounts("frame")
				}
			}
		})
	}
	wg.Wait()
}

// TestRegisterWithMountsNoTornRead proves the TOCTOU fix: while a frame is
// being (re)registered with mounts, a reader that successfully looks up its
// token must always also observe the mounts for that frame. RegisterWithMounts
// writes token and mounts under one lock, so no intermediate token-without-mounts
// state is ever observable — the exact window the old Generate-then-Store split
// left open. The writer never deletes or registers without mounts, so the
// invariant "token visible ⟹ mounts visible" holds at every instant.
func TestRegisterWithMountsNoTornRead(t *testing.T) {
	const frameID = state.FrameID("f1")
	mounts := pathmap.Mounts{{Host: "/host/work", Container: "/work"}}

	reg := framereg.New()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	tokens := []string{"tok-a", "tok-b", "tok-c"}

	wg.Go(func() {
		for i := range 9000 {
			reg.RegisterWithMounts(frameID, tokens[i%len(tokens)], mounts)
		}
		close(stop)
	})

	for range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					for _, tok := range tokens {
						if _, ok := reg.Lookup(tok); ok {
							if _, mok := reg.GetMounts(frameID); !mok {
								t.Errorf("torn read: token %q visible but mounts absent", tok)
								return
							}
						}
					}
				}
			}
		})
	}
	wg.Wait()
}
