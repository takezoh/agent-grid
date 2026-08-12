// Package framereg provides a single-writer/multi-reader registry that maps
// frame bearer tokens to frame IDs.
//
// The event loop is the sole writer (Register, Delete). Frame-messaging
// handlers read concurrently (Lookup) from their own per-connection
// goroutines. A single RWMutex owned by this package is the only lock in the
// runtime root call path, keeping the "no mutexes outside sources" principle
// intact while still serving off-loop readers safely.
package framereg

import (
	"log/slog"
	"sync"

	"github.com/takezoh/agent-grid/services/host/state"
)

// Registry maps frame bearer tokens to frame IDs. The event loop is the single
// writer; frame-messaging handlers read concurrently.
type Registry struct {
	mu           sync.RWMutex
	frameToToken map[state.FrameID]string
	tokenToFrame map[string]state.FrameID
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		frameToToken: make(map[state.FrameID]string),
		tokenToFrame: make(map[string]state.FrameID),
	}
}

// Register stores token for frameID, replacing any prior token.
// Must be called from the single writer (event loop or bootstrap pre-Run).
func (reg *Registry) Register(frameID state.FrameID, token string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if old, ok := reg.frameToToken[frameID]; ok {
		delete(reg.tokenToFrame, old)
	}
	reg.warnIfTokenRebind(token, frameID)
	reg.frameToToken[frameID] = token
	reg.tokenToFrame[token] = frameID
}

// warnIfTokenRebind logs when a token is being rebound from one frame to
// another and fully evicts the previous holder so the rebind matches the
// post-Delete shape. 32-byte random tokens make this practically unreachable
// except for token reuse bugs (issues/029 F8). Caller must hold reg.mu.
func (reg *Registry) warnIfTokenRebind(token string, newFrameID state.FrameID) {
	prev, ok := reg.tokenToFrame[token]
	if !ok || prev == newFrameID {
		return
	}
	slog.Warn("framereg: token rebound to a different frame; previous binding evicted",
		"token-prefix", tokenPrefix(token), "prev-frame", prev, "new-frame", newFrameID)
	delete(reg.frameToToken, prev)
}

// tokenPrefix returns the first 8 chars of token for log diagnostics without
// leaking the full bearer.
func tokenPrefix(token string) string {
	const prefixLen = 8
	if len(token) <= prefixLen {
		return token
	}
	return token[:prefixLen] + "…"
}

// Lookup returns the FrameID associated with token, or ("", false).
// Safe for concurrent reads.
func (reg *Registry) Lookup(token string) (state.FrameID, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	id, ok := reg.tokenToFrame[token]
	return id, ok
}

// Delete removes the token associated with frameID.
// Must be called from the single writer (event loop or bootstrap pre-Run).
func (reg *Registry) Delete(frameID state.FrameID) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if token, ok := reg.frameToToken[frameID]; ok {
		delete(reg.tokenToFrame, token)
		delete(reg.frameToToken, frameID)
	}
}
