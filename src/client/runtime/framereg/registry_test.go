package framereg_test

import (
	"sync"
	"testing"

	"github.com/takezoh/agent-roost/client/runtime/framereg"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/pathmap"
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
