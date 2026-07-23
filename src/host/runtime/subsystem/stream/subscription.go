package stream

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agent/codexclient"
)

// ActivateFrame implements subsystem.Subsystem. It commits the runtime-owned
// side of readiness only after the runtime has registered all frame handles.
func (b *Backend) ActivateFrame(frameID state.FrameID) {
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.runtimeActivated = true
	}
	payload, ready := commitReady(binding)
	b.mu.Unlock()
	if ready {
		b.emit(frameID, state.SubsystemSessionReady, payload)
	}
}

// startObserverSubscription begins the backend connection's subscription
// without blocking the notification read loop that discovered a fresh thread.
func (b *Backend) startObserverSubscription(frameID state.FrameID) {
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil || binding.observerSubscribeStarted || (b.conn == nil && b.resumeObserver == nil) {
		b.mu.Unlock()
		return
	}
	binding.observerSubscribeStarted = true
	b.mu.Unlock()

	go func() {
		if err := b.subscribeObserver(frameID); err != nil {
			b.failFrame(frameID, err)
			b.ReleaseFrame(frameID)
		}
	}()
}

// subscribeObserver issues thread/resume on the Backend's own connection.
// A resume is also the app-server's connection-scoped subscribe operation.
func (b *Backend) subscribeObserver(frameID state.FrameID) error {
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return fmt.Errorf("stream backend: frame %q disappeared before observer subscription", frameID)
	}
	binding.observerSubscribeStarted = true
	wantID := strings.TrimSpace(binding.threadID)
	generation := binding.generation
	opts := codexclient.ResumeOptions{
		ThreadID:    wantID,
		RolloutPath: binding.rolloutPath,
		Cwd:         binding.startDir,
	}
	b.mu.Unlock()
	if wantID == "" {
		return fmt.Errorf("stream backend: frame %q has no thread identity to subscribe", frameID)
	}

	session, err := b.resumeObserverThread(opts)
	if err != nil {
		return fmt.Errorf("stream backend: subscribe observer for thread %q: %w", wantID, err)
	}
	canonicalID := strings.TrimSpace(session.ThreadID)
	if canonicalID == "" || canonicalID != wantID {
		if canonicalID != "" {
			b.bestEffortUnsubscribe(frameID, canonicalID)
		}
		return fmt.Errorf("stream backend: observer resume identity mismatch: requested %q, observed %q", wantID, canonicalID)
	}

	b.mu.Lock()
	binding = b.frames[frameID]
	if binding == nil || binding.generation != generation || binding.threadID != wantID {
		b.mu.Unlock()
		b.bestEffortUnsubscribe(frameID, canonicalID)
		return nil // released or rebound while the request was in flight
	}
	binding.observerSubscribed = true
	binding.canonicalIdentityValidated = true
	binding.observedID = canonicalID
	binding.resumePhase = resumePhaseAttached
	if session.SessionID != "" {
		binding.sessionID = session.SessionID
	}
	if session.RolloutPath != "" {
		if _, hostPath, pathErr := translateRolloutPath(session.RolloutPath, b.mounts); pathErr == nil {
			binding.rolloutPath = hostPath
		}
	}
	payload, ready := commitReady(binding)
	b.mu.Unlock()
	if ready {
		b.emit(frameID, state.SubsystemSessionReady, payload)
	}
	return nil
}

func (b *Backend) resumeObserverThread(opts codexclient.ResumeOptions) (codexclient.ThreadSession, error) {
	if b.resumeObserver != nil {
		return b.resumeObserver(opts)
	}
	if b.conn == nil {
		return codexclient.ThreadSession{}, fmt.Errorf("stream backend: observer connection is not started")
	}
	return codexclient.ResumeThread(b.conn, opts)
}

func (b *Backend) bestEffortUnsubscribe(frameID state.FrameID, threadID string) {
	var err error
	if b.unsubscribeObserver != nil {
		_, err = b.unsubscribeObserver(threadID)
	} else if b.conn != nil {
		_, err = codexclient.UnsubscribeThread(b.conn, threadID)
	}
	if err != nil {
		slog.Debug("stream backend: thread unsubscribe failed",
			"subsystem", b.subsystemID, "frame", frameID, "thread", threadID, "err", err)
	}
}

func commitReady(binding *frameBinding) (state.SubsystemPayload, bool) {
	if binding == nil || binding.readyCommitted || !binding.runtimeActivated ||
		!binding.observerSubscribed || !binding.canonicalIdentityValidated {
		return state.SubsystemPayload{}, false
	}
	binding.readyCommitted = true
	return payloadFromBinding(binding), true
}
