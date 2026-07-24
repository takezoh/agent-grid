package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

const lifecycleApplyDeadline = 4 * time.Second
const lifecycleLeaseTTL = 12 * time.Second

type lifecycleBinding struct {
	connID       state.ConnID
	sessionID    state.SessionID
	subscriberID state.SubscriberID
	correlation  proto.PublicCorrelation
}

func lifecycleBindingKey(connID state.ConnID, c proto.PublicCorrelation) string {
	return fmt.Sprintf("%d/%s/%d", connID, c.ClientInstanceID, c.ConnectionGeneration)
}

func (r *Runtime) handleLifecycleDesired(e internalLifecycleDesired) {
	c := e.cmd.Correlation
	if c.ClientInstanceID == "" || c.ConnectionGeneration == 0 || c.ClientRevision == 0 || e.cmd.SessionID == "" {
		r.sendLifecycleError(e.connID, e.reqID, proto.ErrInvalidArgument, "lifecycle desired requires public correlation and session")
		return
	}
	frameID := ""
	if e.cmd.Desired {
		if e.cmd.Cols == 0 || e.cmd.Rows == 0 {
			r.sendLifecycleError(e.connID, e.reqID, proto.ErrInvalidArgument, "lifecycle desired requires non-zero geometry")
			return
		}
		if reason := state.SizeHintRejectReason(int(e.cmd.Cols), int(e.cmd.Rows)); reason != "" {
			r.sendLifecycleError(e.connID, e.reqID, proto.ErrInvalidArgument, reason)
			return
		}
		sess, ok := r.state.Sessions[state.SessionID(e.cmd.SessionID)]
		if !ok {
			r.sendLifecycleError(e.connID, e.reqID, proto.ErrNotFound, "session not found: "+e.cmd.SessionID)
			return
		}
		frame, ok := sessionHeadFrame(sess)
		if !ok {
			r.sendLifecycleError(e.connID, e.reqID, proto.ErrFrameNotReady, "frame-not-ready: "+e.cmd.SessionID)
			return
		}
		frameID = string(frame.ID)
	}
	if e.cmd.SubscriberID == "" {
		e.cmd.SubscriberID = fmt.Sprintf("lifecycle-%d", e.connID)
	}
	key := lifecycleBindingKey(e.connID, c)
	if e.cmd.Desired {
		if _, exists := r.lifecycleBindings[key]; !exists && len(r.lifecycleBindings) >= 8 {
			r.sendLifecycleError(e.connID, e.reqID, proto.ErrResourceExhausted, "lifecycle owner cap (8) exceeded")
			return
		}
	}

	_, alreadyAdmitted := r.lifecycleActor.Lookup(c)
	out := r.lifecycleActor.Admit(c)
	r.emitLifecycleOutcome(out)
	if out.Status == proto.RevisionRejected || alreadyAdmitted {
		r.sendLifecycleResponse(e.connID, e.reqID)
		return
	}
	if waiting, ok := r.lifecycleActor.MarkWaiting(c); ok {
		r.emitLifecycleOutcome(waiting)
	}
	r.sendLifecycleResponse(e.connID, e.reqID)

	if old, ok := r.lifecycleBindings[key]; ok && old.correlation.ClientRevision < c.ClientRevision {
		if old.sessionID != state.SessionID(e.cmd.SessionID) || old.subscriberID != state.SubscriberID(e.cmd.SubscriberID) {
			if r.terminalRelay != nil {
				r.terminalRelay.UnsubscribeOwned(old.connID, old.sessionID, old.subscriberID)
			}
		}
		delete(r.lifecycleBindings, key)
	}
	if e.cmd.Desired {
		r.lifecycleBindings[key] = lifecycleBinding{
			connID: e.connID, sessionID: state.SessionID(e.cmd.SessionID),
			subscriberID: state.SubscriberID(e.cmd.SubscriberID), correlation: c,
		}
	} else {
		delete(r.lifecycleBindings, key)
	}

	go r.runLifecycleEffect(e.connID, e.cmd, frameID)
	go r.enqueueLifecycleDeadline(e.connID, c)
	if e.cmd.Desired {
		go r.enqueueLifecycleExpiry(e.connID, c)
	}
}

func (r *Runtime) runLifecycleEffect(connID state.ConnID, cmd proto.CmdLifecycleDesired, frameID string) {
	ctx, cancel := context.WithTimeout(r.baseContext(), lifecycleApplyDeadline)
	defer cancel()
	var err error
	r.lifecycleEffectMu.Lock()
	defer r.lifecycleEffectMu.Unlock()
	switch {
	case r.terminalRelay == nil:
		err = fmt.Errorf("terminal surface relay unavailable")
	case cmd.Desired:
		err = r.rebindLifecycleWithRetry(ctx, connID, cmd, frameID)
	default:
		r.terminalRelay.UnsubscribeOwned(connID, state.SessionID(cmd.SessionID), state.SubscriberID(cmd.SubscriberID))
	}
	select {
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
	default:
	}
	r.sendInternalNow(internalLifecycleEffectResult{connID: connID, cmd: cmd, err: err})
}

func (r *Runtime) rebindLifecycleWithRetry(ctx context.Context, connID state.ConnID, cmd proto.CmdLifecycleDesired, frameID string) error {
	delays := []time.Duration{0, 250 * time.Millisecond, 500 * time.Millisecond, time.Second}
	var err error
	for _, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
		if err = r.terminalRelay.RebindOwned(
			connID, state.SessionID(cmd.SessionID), state.SubscriberID(cmd.SubscriberID),
			frameID, int(cmd.Cols), int(cmd.Rows),
		); err == nil {
			return nil
		}
	}
	return err
}

func (r *Runtime) enqueueLifecycleDeadline(connID state.ConnID, c proto.PublicCorrelation) {
	t := time.NewTimer(lifecycleApplyDeadline)
	defer t.Stop()
	select {
	case <-t.C:
		r.sendInternalNow(internalLifecycleDeadline{connID: connID, correlation: c})
	case <-r.done:
	}
}

func (r *Runtime) handleLifecycleEffectResult(e internalLifecycleEffectResult) {
	status := proto.RevisionApplied
	if !e.cmd.Desired {
		status = proto.RevisionReleased
	}
	if e.err != nil {
		status = proto.RevisionDegraded
	}
	out, err := r.lifecycleActor.Complete(e.cmd.Correlation, status, 0, 0, lifecycleReason(e.err))
	if err == nil {
		r.emitLifecycleOutcome(out)
	}
}

func (r *Runtime) handleLifecycleDeadline(e internalLifecycleDeadline) {
	out, err := r.lifecycleActor.Complete(e.correlation, proto.RevisionDegraded, 0, 0, "apply_deadline_exceeded")
	if err == nil {
		r.emitLifecycleOutcome(out)
	}
}

func (r *Runtime) enqueueLifecycleExpiry(connID state.ConnID, c proto.PublicCorrelation) {
	t := time.NewTimer(lifecycleLeaseTTL)
	defer t.Stop()
	select {
	case <-t.C:
		r.sendInternalNow(internalLifecycleExpiry{connID: connID, correlation: c})
	case <-r.done:
	}
}

func (r *Runtime) handleLifecycleExpiry(e internalLifecycleExpiry) {
	key := lifecycleBindingKey(e.connID, e.correlation)
	binding, ok := r.lifecycleBindings[key]
	if !ok || binding.correlation.ClientRevision != e.correlation.ClientRevision {
		return
	}
	delete(r.lifecycleBindings, key)
	if r.terminalRelay != nil {
		r.terminalRelay.UnsubscribeOwned(binding.connID, binding.sessionID, binding.subscriberID)
	}
	out, err := r.lifecycleActor.Complete(e.correlation, proto.RevisionReleased, 0, 0, "lease_expired")
	if err == nil {
		r.emitLifecycleOutcome(out)
	}
}

func lifecycleReason(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *Runtime) sendLifecycleResponse(connID state.ConnID, reqID string) {
	cc, ok := r.conns[connID]
	if !ok {
		return
	}
	wire, err := proto.EncodeResponse(reqID, proto.RespOK{})
	if err == nil {
		r.queueWire(cc, wire)
	}
}

func (r *Runtime) sendLifecycleError(connID state.ConnID, reqID string, code proto.ErrCode, message string) {
	cc, ok := r.conns[connID]
	if !ok {
		return
	}
	wire, err := proto.EncodeError(reqID, code, message, nil)
	if err == nil {
		r.queueWire(cc, wire)
	}
}

func (r *Runtime) emitLifecycleOutcome(out proto.RevisionOutcome) {
	wire, err := proto.EncodeEvent(proto.EvtLifecycleOutcome{RevisionOutcome: out})
	if err == nil {
		r.broadcastWire(wire, proto.EvtNameLifecycleOutcome)
	}
}

func (r *Runtime) releaseLifecycleBindings(connID state.ConnID) {
	for key, binding := range r.lifecycleBindings {
		if binding.connID != connID {
			continue
		}
		delete(r.lifecycleBindings, key)
		if r.terminalRelay != nil {
			r.terminalRelay.UnsubscribeOwned(binding.connID, binding.sessionID, binding.subscriberID)
		}
	}
}
