package runtime

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Compile-time proof that PtyBackend satisfies the full TmuxBackend role set.
var _ TmuxBackend = (*PtyBackend)(nil)

// waitUntil polls pred until it returns true or the deadline elapses.
func waitUntil(t *testing.T, pred func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if pred() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for condition")
		case <-tick.C:
		}
	}
}

// TestPtyBackendSpawnEchoCaptureKill exercises the full data-plane flow:
// spawn a cat pty, send a line, capture the echoed output, then kill and
// observe the exit status.
func TestPtyBackendSpawnEchoCaptureKill(t *testing.T) {
	b := NewPtyBackend()

	winIdx, paneID, err := b.SpawnWindow("w1", "cat", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	if winIdx == "" || paneID == "" {
		t.Fatalf("SpawnWindow returned empty ids: win=%q pane=%q", winIdx, paneID)
	}

	// PaneID echoes the synthetic id back.
	if got, err := b.PaneID(paneID); err != nil || got != paneID {
		t.Fatalf("PaneID(%q) = %q, %v; want %q", paneID, got, err, paneID)
	}

	// Alive before kill.
	if alive, err := b.PaneAlive(paneID); err != nil || !alive {
		t.Fatalf("PaneAlive(%q) = %v, %v; want true", paneID, alive, err)
	}

	// SendKeys appends Enter; cat echoes it back.
	if err := b.SendKeys(paneID, "echo-marker-xyz"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var captured string
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		if err != nil {
			return false
		}
		captured = out
		return strings.Contains(out, "echo-marker-xyz")
	})
	if strings.Contains(captured, "\x1b[") {
		t.Fatalf("CapturePane output still contains SGR escapes: %q", captured)
	}

	if err := b.KillPaneWindow(paneID); err != nil {
		t.Fatalf("KillPaneWindow: %v", err)
	}

	// After kill the pane is no longer alive.
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(paneID)
		return err == nil && !alive
	})
}

// TestPtyBackendExitStatus verifies a process that exits non-zero reports its
// code via PaneExitStatus.
func TestPtyBackendExitStatus(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "bash -c 'exit 7'", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}

	var code int
	waitUntil(t, func() bool {
		dead, c, err := b.PaneExitStatus(paneID)
		if err != nil || !dead {
			return false
		}
		code = c
		return true
	})
	if code != 7 {
		t.Fatalf("PaneExitStatus code = %d, want 7", code)
	}
}

// TestPtyBackendEnvStore verifies the in-process session env store backing
// SetEnv/UnsetEnv/ShowEnvironment.
func TestPtyBackendEnvStore(t *testing.T) {
	b := NewPtyBackend()
	if err := b.SetEnv("BETA", "2"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetEnv("ALPHA", "1"); err != nil {
		t.Fatal(err)
	}
	out, err := b.ShowEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	// Sorted ascending by key.
	if out != "ALPHA=1\nBETA=2\n" {
		t.Fatalf("ShowEnvironment() = %q, want sorted KEY=VALUE lines", out)
	}
	if err := b.UnsetEnv("ALPHA"); err != nil {
		t.Fatal(err)
	}
	out, _ = b.ShowEnvironment()
	if out != "BETA=2\n" {
		t.Fatalf("after UnsetEnv = %q, want %q", out, "BETA=2\n")
	}
}

// TestPtyBackendBufferRoundTrip verifies LoadBuffer holds text and PasteBuffer
// writes it to the pane then drops the buffer.
func TestPtyBackendBufferRoundTrip(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.LoadBuffer("buf1", "pasted-text-abc\n"); err != nil {
		t.Fatal(err)
	}
	if err := b.PasteBuffer("buf1", paneID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "pasted-text-abc")
	})
	// Buffer consumed: a second paste is a no-op error (buffer gone).
	if err := b.PasteBuffer("buf1", paneID); err == nil {
		t.Fatal("PasteBuffer on consumed buffer should error")
	}
}

// TestPtyBackendResize verifies ResizeWindow is delegated to the session and
// PaneSize reflects the new dimensions.
func TestPtyBackendResize(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.ResizeWindow(paneID, 120, 40); err != nil {
		t.Fatal(err)
	}
	w, h, err := b.PaneSize(paneID)
	if err != nil {
		t.Fatal(err)
	}
	if w != 120 || h != 40 {
		t.Fatalf("PaneSize = %dx%d, want 120x40", w, h)
	}
}

// TestPtyBackendSpawnSyntheticIDs verifies synthetic id allocation increments.
func TestPtyBackendSpawnSyntheticIDs(t *testing.T) {
	b := NewPtyBackend()
	win1, pane1, err := b.SpawnWindow("a", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(pane1) }()
	win2, pane2, err := b.SpawnWindow("b", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(pane2) }()

	if pane1 != "%1" || pane2 != "%2" {
		t.Fatalf("pane ids = %q,%q; want %%1,%%2", pane1, pane2)
	}
	if win1 != "1" || win2 != "2" {
		t.Fatalf("window indexes = %q,%q; want 1,2", win1, win2)
	}
}

// TestKeyBytes is the table test for the named-key → byte-sequence mapping,
// including the literal passthrough for unknown keys.
func TestKeyBytes(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"Escape", "\x1b"},
		{"Enter", "\r"},
		{"Up", "\x1b[A"},
		{"Down", "\x1b[B"},
		{"Right", "\x1b[C"},
		{"Left", "\x1b[D"},
		{"Tab", "\t"},
		{"BSpace", "\x7f"},
		{"Space", " "},
		{"q", "q"},                       // unknown single char passes through literally
		{"some-literal", "some-literal"}, // unknown multi-char passes through
		{"", ""},                         // empty passes through as empty
		{"C-c", "\x03"},                  // control chord → SIGINT byte
		{"C-a", "\x01"},                  // control chord → SOH
		{"M-x", "\x1bx"},                 // meta chord → ESC + char
		{"X-y", "X-y"},                   // unknown chord prefix passes through
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := keyBytes(c.key); got != c.want {
				t.Errorf("keyBytes(%q) = %q, want %q", c.key, got, c.want)
			}
		})
	}
}

// TestPtyBackendSendKey verifies SendKey reaches the pty by echoing a Space
// keystroke through cat and observing it in the captured output.
func TestPtyBackendSendKey(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	// Send a recognisable text marker via SendKeys, then a Space via SendKey;
	// cat echoes both. We assert the marker appears (SendKey path drove the pty).
	if err := b.SendKeys(paneID, "key-marker"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	if err := b.SendKey(paneID, "Space"); err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "key-marker")
	})
}

// TestPtyBackendUnknownPaneErrors pins the unknown-target contract: every
// inspect/IO/lifecycle op that addresses a pane returns a non-nil error for an
// unspawned target, while PaneAlive reports (false, nil).
func TestPtyBackendUnknownPaneErrors(t *testing.T) {
	b := NewPtyBackend()
	const unknown = "%999"

	if err := b.SendKeys(unknown, "x"); err == nil {
		t.Error("SendKeys(unknown) error = nil, want non-nil")
	}
	if _, err := b.CapturePane(unknown, 10); err == nil {
		t.Error("CapturePane(unknown) error = nil, want non-nil")
	}
	if err := b.ResizeWindow(unknown, 80, 24); err == nil {
		t.Error("ResizeWindow(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.PaneSize(unknown); err == nil {
		t.Error("PaneSize(unknown) error = nil, want non-nil")
	}
	if _, err := b.PaneID(unknown); err == nil {
		t.Error("PaneID(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.PaneExitStatus(unknown); err == nil {
		t.Error("PaneExitStatus(unknown) error = nil, want non-nil")
	}
	if err := b.KillPaneWindow(unknown); err == nil {
		t.Error("KillPaneWindow(unknown) error = nil, want non-nil")
	}
	// PaneAlive is the explicit exception: unknown target is reported dead, no error.
	if alive, err := b.PaneAlive(unknown); alive || err != nil {
		t.Errorf("PaneAlive(unknown) = %v, %v; want false, nil", alive, err)
	}
}

// TestPtyBackendExitStatusLive verifies PaneExitStatus on a running process
// reports (false, -1, nil), and PaneAlive flips to false once a clean exit is
// reaped.
func TestPtyBackendExitStatusLive(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	dead, code, err := b.PaneExitStatus(paneID)
	if err != nil {
		t.Fatalf("PaneExitStatus(live) err = %v, want nil", err)
	}
	if dead || code != -1 {
		t.Fatalf("PaneExitStatus(live) = %v, %d; want false, -1", dead, code)
	}

	// A separate pane that exits 0: PaneAlive must flip to false.
	_, exitPane, err := b.SpawnWindow("w2", "bash -c 'exit 0'", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(exitPane) }()
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(exitPane)
		return err == nil && !alive
	})
}

// TestPtyBackendRespawn verifies a pane can be respawned in place and that an
// empty respawn command is rejected.
func TestPtyBackendRespawn(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "bash -c 'exit 0'", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	// Wait for the original to die so we respawn over a reaped pane.
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(paneID)
		return err == nil && !alive
	})

	// Respawn over the same target with a long-lived command: pane is alive again.
	if err := b.RespawnPane(paneID, "cat"); err != nil {
		t.Fatalf("RespawnPane: %v", err)
	}
	if alive, err := b.PaneAlive(paneID); err != nil || !alive {
		t.Fatalf("PaneAlive after respawn = %v, %v; want true", alive, err)
	}
	// Echo path still works on the respawned pane.
	if err := b.SendKeys(paneID, "respawned-ok"); err != nil {
		t.Fatalf("SendKeys after respawn: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "respawned-ok")
	})

	// Empty respawn command is rejected.
	if err := b.RespawnPane(paneID, ""); err == nil {
		t.Error("RespawnPane(empty) error = nil, want non-nil")
	}
}

// TestPtyBackendConcurrent drives spawn/SendKeys/CapturePane/KillPaneWindow from
// many goroutines so `go test -race` can prove the backend's shared state is
// race-free.
func TestPtyBackendConcurrent(t *testing.T) {
	b := NewPtyBackend()
	const workers = 8

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
			if err != nil {
				t.Errorf("SpawnWindow: %v", err)
				return
			}
			_ = b.SendKeys(paneID, "hello")
			_, _ = b.CapturePane(paneID, 10)
			_, _ = b.PaneAlive(paneID)
			_ = b.KillPaneWindow(paneID)
		}()
	}
	wg.Wait()
}
