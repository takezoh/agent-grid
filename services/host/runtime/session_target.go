package runtime

import "github.com/takezoh/agent-grid/services/host/state"

// sessionHeadFrameTarget returns the frame target (string(FrameID)) for the
// head frame of the given session. Returns "" if the session has no head
// frame. After the B2 indirection removal termvt keys sessions on
// string(FrameID) directly, so the lookup collapses to a HeadFrameID string
// conversion.
func (r *Runtime) sessionHeadFrameTarget(sid state.SessionID) string {
	sess, ok := r.state.Sessions[sid]
	if !ok {
		return ""
	}
	frame, ok := sessionHeadFrame(sess)
	if !ok {
		return ""
	}
	return string(frame.ID)
}
