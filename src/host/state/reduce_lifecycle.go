package state

func init() {
	RegisterEvent[struct{}](EventShutdown, reduceShutdown)
}

// reduceShutdown starts (or joins) the reducer-owned shutdown transaction.
// Persistence acknowledgement and cleanup completion return as explicit
// result events; no teardown is authorized by the request itself.
func reduceShutdown(s State, connID ConnID, reqID string, _ struct{}) (State, []Effect) {
	waiter := ShutdownWaiter{ConnID: connID, ReqID: reqID}
	if s.Lifecycle == LifecycleQuiescing {
		if s.Shutdown != nil && connID != 0 {
			tx := *s.Shutdown
			tx.Waiters = append(append([]ShutdownWaiter(nil), tx.Waiters...), waiter)
			s.Shutdown = &tx
		}
		return s, nil
	}
	s.NextShutdownID++
	tx := &ShutdownTransaction{ID: s.NextShutdownID}
	if connID != 0 {
		tx.Waiters = []ShutdownWaiter{waiter}
	}
	s.Lifecycle = LifecycleQuiescing
	s.Shutdown = tx
	return s, []Effect{EffCommitShutdownSessions{TransactionID: tx.ID}}
}

func reduceShutdownSaveSucceeded(s State, e EvShutdownSaveBarrierSucceeded) (State, []Effect) {
	if !currentShutdown(s, e.TransactionID) {
		return s, nil
	}
	return s, []Effect{EffReleaseFrameSandboxes(e)}
}

func reduceShutdownSaveFailed(s State, e EvShutdownSaveBarrierFailed) (State, []Effect) {
	if !currentShutdown(s, e.TransactionID) {
		return s, nil
	}
	effects := make([]Effect, 0, len(s.Shutdown.Waiters)+1)
	for _, waiter := range s.Shutdown.Waiters {
		effects = append(effects, errResp(waiter.ConnID, waiter.ReqID, ErrCodeInternal, "shutdown persistence failed: "+e.Err))
	}
	effects = append(effects, EffCompleteShutdown{Result: ShutdownCommitFailed})
	s.Lifecycle = LifecycleRunning
	s.Shutdown = nil
	return s, effects
}

func reduceShutdownCleanupFinished(s State, e EvShutdownCleanupFinished) (State, []Effect) {
	if !currentShutdown(s, e.TransactionID) {
		return s, nil
	}
	result := ShutdownCommitted
	if e.Outcome == ShutdownCleanupDeadlineExceeded {
		result = ShutdownDeadlineExceeded
	}
	effects := make([]Effect, 0, len(s.Shutdown.Waiters)+2)
	for _, waiter := range s.Shutdown.Waiters {
		if result == ShutdownDeadlineExceeded {
			effects = append(effects, errResp(waiter.ConnID, waiter.ReqID, ErrCodeUnavailable, "shutdown cleanup deadline exceeded"))
		} else {
			effects = append(effects, EffSendResponseSync{ConnID: waiter.ConnID, ReqID: waiter.ReqID, Body: nil})
		}
	}
	effects = append(effects, EffCompleteShutdown{Result: result}, EffTerminateRuntime{})
	// Consume the transaction before the terminal effect is interpreted. The
	// event loop may select an already-queued duplicate cleanup result before it
	// selects terminateCh; clearing it here makes that duplicate stale.
	s.Shutdown = nil
	return s, effects
}

func currentShutdown(s State, id uint64) bool {
	return s.Lifecycle == LifecycleQuiescing && s.Shutdown != nil && s.Shutdown.ID == id
}
