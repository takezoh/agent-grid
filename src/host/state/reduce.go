package state

import (
	"fmt"
)

// Reduce is the pure state transition function. Given a current State
// and an Event, it returns the next State and the side effects the
// runtime should execute.
//
// Reduce never panics in production: every Event variant must have a
// case below. The default branch panics so an unimplemented case
// fails fast in tests rather than silently dropping events.
//
// Reducer cases live in reduce_*.go files split by domain.
func Reduce(s State, ev Event) (State, []Effect) {
	if s.Lifecycle == LifecycleQuiescing {
		return reduceQuiescing(s, ev)
	}
	switch e := ev.(type) {
	// registered command event → dispatch by event type
	case EvEvent:
		return reduceEvent(s, e)
	// driver hook event → route to session's driver
	case EvDriverEvent:
		return reduceDriverHook(s, e)
	case EvSubsystem:
		return reduceSubsystem(s, e)

	// frame backend feedback
	case EvFrameSpawned:
		return reduceFrameSpawned(s, e)
	case EvSpawnFailed:
		return reduceSpawnFailed(s, e)
	case EvFrameVanished:
		return reduceFrameVanished(s, e)
	case EvFrameCommandExited:
		return reduceFrameCommandExited(s, e)

	// tick
	case EvTick:
		return reduceTick(s, e)

	// worker
	case EvJobResult:
		return reduceJobResult(s, e)

	// fsnotify
	case EvFileChanged:
		return reduceFileChanged(s, e)

	// frame tap
	case EvFrameOsc:
		return reduceFrameOsc(s, e)
	case EvFramePrompt:
		return reduceFramePrompt(s, e)
	case EvShutdownSaveBarrierSucceeded:
		return reduceShutdownSaveSucceeded(s, e)
	case EvShutdownSaveBarrierFailed:
		return reduceShutdownSaveFailed(s, e)
	case EvShutdownCleanupFinished:
		return reduceShutdownCleanupFinished(s, e)

	// connection lifecycle
	case EvConnOpened:
		return reduceConnOpened(s, e)
	case EvConnClosed:
		return reduceConnClosed(s, e)
	case EvCmdSubscribe:
		return reduceSubscribe(s, e)
	case EvCmdUnsubscribe:
		return reduceUnsubscribe(s, e)

	// surface.* / driver.* RPC commands
	case EvCmdSurfaceReadText:
		return reduceSurfaceReadText(s, e)
	case EvCmdSurfaceSendText:
		return reduceSurfaceSendText(s, e)
	case EvCmdSurfaceSendKey:
		return reduceSurfaceSendKey(s, e)
	case EvCmdDriverList:
		return reduceDriverList(s, e)
	case EvCmdSurfaceSubscribe:
		return reduceSurfaceSubscribe(s, e)
	case EvCmdSurfaceUnsubscribe:
		return reduceSurfaceUnsubscribe(s, e)
	case EvCmdSurfaceResize:
		return reduceSurfaceResize(s, e)
	case EvCmdSurfaceWriteRaw:
		return reduceSurfaceWriteRaw(s, e)
	}

	panic(fmt.Sprintf("state.Reduce: unhandled event type %T", ev))
}

// reduceQuiescing is the central admission boundary. Lifecycle results and
// connection-only bookkeeping remain live; session-mutating feedback is
// neutralized before any domain reducer can emit persistence/delete effects.
func reduceQuiescing(s State, ev Event) (State, []Effect) {
	switch e := ev.(type) {
	case EvShutdownSaveBarrierSucceeded:
		next, effects := reduceShutdownSaveSucceeded(s, e)
		return next, effects
	case EvShutdownSaveBarrierFailed:
		next, effects := reduceShutdownSaveFailed(s, e)
		return next, effects
	case EvShutdownCleanupFinished:
		next, effects := reduceShutdownCleanupFinished(s, e)
		return next, effects
	case EvConnOpened:
		next, effects := reduceConnOpened(s, e)
		return next, effects
	case EvConnClosed:
		next, effects := reduceConnClosed(s, e)
		return next, effects
	case EvCmdDriverList:
		next, effects := reduceDriverList(s, e)
		return next, effects
	case EvCmdSurfaceReadText:
		next, effects := reduceSurfaceReadText(s, e)
		return next, effects
	case EvCmdSubscribe:
		next, effects := reduceSubscribe(s, e)
		return next, effects
	case EvCmdUnsubscribe:
		next, effects := reduceUnsubscribe(s, e)
		return next, effects
	case EvCmdSurfaceUnsubscribe:
		next, effects := reduceSurfaceUnsubscribe(s, e)
		return next, effects
	case EvEvent:
		if e.Event == EventShutdown || e.Event == EventListSessions || e.Event == EventListSessionMessages {
			next, effects := reduceEvent(s, e)
			return next, effects
		}
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeUnavailable, "runtime is shutting down")}
	case EvCmdSurfaceSubscribe, EvCmdSurfaceSendText, EvCmdSurfaceSendKey,
		EvCmdSurfaceResize, EvCmdSurfaceWriteRaw:
		connID, reqID := requestIdentity(ev)
		return s, []Effect{errResp(connID, reqID, ErrCodeUnavailable, "runtime is shutting down")}
	case EvDriverEvent:
		if e.ConnID != 0 {
			return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeUnavailable, "runtime is shutting down")}
		}
		return s, nil
	case EvSubsystem:
		if e.ConnID != 0 {
			return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeUnavailable, "runtime is shutting down")}
		}
		return s, nil
	case EvFrameSpawned, EvSpawnFailed, EvFrameVanished,
		EvFrameCommandExited, EvTick, EvJobResult, EvFileChanged,
		EvFrameOsc, EvFramePrompt:
		return s, nil
	default:
		panic(fmt.Sprintf("state.reduceQuiescing: unclassified event type %T", ev))
	}
}

func requestIdentity(ev Event) (ConnID, string) {
	switch e := ev.(type) {
	case EvCmdSurfaceSubscribe:
		return e.ConnID, e.ReqID
	case EvCmdSurfaceSendText:
		return e.ConnID, e.ReqID
	case EvCmdSurfaceSendKey:
		return e.ConnID, e.ReqID
	case EvCmdSurfaceResize:
		return e.ConnID, e.ReqID
	case EvCmdSurfaceWriteRaw:
		return e.ConnID, e.ReqID
	default:
		panic(fmt.Sprintf("state.requestIdentity: unsupported event %T", ev))
	}
}

// Reducer cases live in reduce_event.go / reduce_session.go /
// reduce_tick.go / reduce_job.go / reduce_conn.go / reduce_lifecycle.go.
