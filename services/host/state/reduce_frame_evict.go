package state

import "time"

// reduce_frame_evict.go holds helpers for removing frames from the state.
// Kept separate from reduce_helpers.go for file-size compliance.

// rootDisposition decides what happens to the session record when its root
// frame goes away.
//
// The distinction is load-bearing: a session that ran and then ended is
// retained so clients can observe its terminal lifecycle state
// (adr-20260814-session-terminal-state-retention), while a session whose very
// first spawn failed never became observable and must leave no row behind —
// showing "stopped" for something that never started would invent a terminal
// state the session never reached.
type rootDisposition int

const (
	// retainEndedSession keeps the record with a terminal timestamp: explicit
	// termination, a clean or signalled self-exit, a vanished backend frame.
	retainEndedSession rootDisposition = iota
	// dropUnstartedSession removes the record entirely: the session never came up.
	dropUnstartedSession
)

// evictFrame removes a frame and its cleanup effects.
//
// When frameID is the root frame (index 0), all sibling frames are also
// removed and the session either ends or disappears according to disposition —
// root death ends the session either way.
// When frameID is a child frame (index > 0), only that frame is removed;
// siblings are unaffected and disposition does not apply.
//
// killWindow controls whether EffKillFrame is emitted per removed
// frame. Pass true when the backend frame still exists (e.g. EvFrameCommandExited);
// pass false when the frame has already vanished (e.g. EvFrameVanished).
//
// Effect ordering: cleanup → persist → broadcast.
func evictFrame(s State, frameID FrameID, killWindow bool, disposition rootDisposition) (State, []Effect, bool) {
	sessID, sess, idx, ok := findFrame(s, frameID)
	if !ok {
		return s, nil, false
	}
	if idx == 0 {
		return evictRootFrame(s, sessID, sess, killWindow, disposition)
	}
	return evictChildFrame(s, sessID, sess, idx, frameID, killWindow)
}

// endSession strips a session down to its retained, observable remains: no
// frames, no head, no MRU, and a terminal timestamp. The record stays in the
// session map so every client sees the terminal lifecycle state the protocol
// already declares instead of a row vanishing
// (adr-20260814-session-terminal-state-retention). Frame and process teardown
// are unaffected — callers still emit frameTeardownEffects for what they
// removed.
func endSession(sess Session, now time.Time) Session {
	ended := now
	sess.Frames = nil
	sess.HeadFrameID = ""
	sess.MRUFrameIDs = nil
	sess.EndedAt = &ended
	return sess
}

func evictRootFrame(
	s State, sessID SessionID, sess Session, killWindow bool, disposition rootDisposition,
) (State, []Effect, bool) {
	allRemoved := truncateFrames(sess, 0)
	s.Sessions = cloneSessions(s.Sessions)
	if disposition == retainEndedSession {
		s.Sessions[sessID] = endSession(sess, s.Now)
	} else {
		delete(s.Sessions, sessID)
	}
	// Reap approval/question domain for the whole session (FR-P0-11 / NFR-06).
	var reapEffs []Effect
	s, reapEffs = cancelApprovalsForSession(s, sessID)
	var qReap []Effect
	s, qReap = cancelQuestionsForSession(s, sessID)
	effs := make([]Effect, 0, len(allRemoved)*4+2+len(reapEffs)+len(qReap))
	effs = append(effs, reapEffs...)
	effs = append(effs, qReap...)
	for _, frame := range allRemoved {
		effs = append(effs, frameTeardownEffects(frame.ID, killWindow)...)
	}
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

func evictChildFrame(s State, sessID SessionID, sess Session, idx int, frameID FrameID, killWindow bool) (State, []Effect, bool) {
	wasActive := sess.HeadFrameID == frameID
	sess, removed := removeFrameByIndex(sess, idx)
	sess = bumpFrameGeneration(sess)
	sess = pruneMRU(sess)
	if wasActive {
		fallback, next := popMRU(sess)
		sess = next
		if fallback == "" {
			fallback = sess.Frames[0].ID
		}
		sess.HeadFrameID = fallback
	}
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sessID] = sess

	// Child-frame teardown reaps only requests owned by that frame.
	var reapEffs []Effect
	s, reapEffs = cancelApprovalsForFrame(s, frameID)
	var qReap []Effect
	s, qReap = cancelQuestionsForFrame(s, frameID)
	effs := frameTeardownEffects(removed.ID, killWindow)
	effs = append(reapEffs, effs...)
	effs = append(effs, qReap...)
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

// frameTeardownEffects builds the canonical per-frame teardown effect set
// emitted whenever a frame leaves the session graph: backend frame kill
// (only when the frame is still alive — frame-vanished routes set
// killWindow=false), frame release (always — frame-vanished must still
// drop the container refcount), frame unregister, file unwatch.
//
// Centralising the set is load-bearing: any new reducer path that
// removes a frame must call this helper, otherwise the silent regression
// is "container survives forever even though no frame uses it" — exactly
// the bug reduceStopSession had until this helper existed.
func frameTeardownEffects(frameID FrameID, killWindow bool) []Effect {
	effs := make([]Effect, 0, 4)
	if killWindow {
		effs = append(effs, EffKillFrame{FrameID: frameID})
	}
	effs = append(effs,
		EffReleaseFrame{FrameID: frameID},
		EffUnregisterFrame{FrameID: frameID},
		EffUnwatchFile{FrameID: frameID},
	)
	return effs
}

// === ErrCode constants used by reducers ===
//
// These mirror proto.ErrCode but live in state pkg so reducers can
// emit them without importing proto. The runtime translates them to
// proto.ErrCode values when serializing the response.
const (
	ErrCodeNotFound          = "not_found"
	ErrCodeInvalidArgument   = "invalid_argument"
	ErrCodeInternal          = "internal"
	ErrCodeAlreadyExists     = "already_exists"
	ErrCodeUnsupported       = "unsupported"
	ErrCodeResourceExhausted = "resource_exhausted"
	ErrCodeFrameNotReady     = "frame_not_ready"
	ErrCodeUnavailable       = "unavailable"
)
