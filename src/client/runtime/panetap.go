package runtime

import "context"

// FrameTap is a source of raw terminal byte streams from a backend pane.
// The event loop starts one tap per frame when the pane is registered
// and stops it when the pane is unregistered. The tap byte stream is fed
// into a VT emulator; OSC callbacks enqueue EvFrameOsc and EvFramePrompt events.
//
// PtyFrameTap (see pty_tap.go) is the current implementation, subscribing
// directly to the termvt.Manager that PtyBackend owns.
type FrameTap interface {
	// Start begins delivering raw bytes for pane into the returned channel.
	// The channel is closed when the tap is stopped or ctx is cancelled.
	Start(ctx context.Context, pane string) (<-chan []byte, error)
	// Stop ends delivery for pane and releases all resources.
	Stop(pane string) error
}
