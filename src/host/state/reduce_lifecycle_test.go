package state

import "testing"

func TestReduceShutdownUsesAcknowledgedBarrier(t *testing.T) {
	next, effects := reduceShutdown(New(), 1, "req-1", struct{}{})
	if next.Lifecycle != LifecycleQuiescing || next.Shutdown == nil {
		t.Fatal("shutdown request must enter Quiescing with an active transaction")
	}
	if len(effects) != 1 {
		t.Fatalf("request effects = %#v, want only Save barrier", effects)
	}
	barrier, ok := effects[0].(EffCommitShutdownSessions)
	if !ok || barrier.TransactionID != next.Shutdown.ID {
		t.Fatalf("request effect = %#v, want matching Save barrier", effects[0])
	}
}

func TestReduceShutdownOrdersBarrierCleanupAckAndTerminal(t *testing.T) {
	s, _ := reduceShutdown(New(), 1, "req-1", struct{}{})
	txID := s.Shutdown.ID
	s, effects := Reduce(s, EvShutdownSaveBarrierSucceeded{TransactionID: txID})
	if len(effects) != 1 {
		t.Fatalf("barrier success effects = %#v", effects)
	}
	release, ok := effects[0].(EffReleaseFrameSandboxes)
	if !ok || release.TransactionID != txID {
		t.Fatalf("barrier success effect = %#v", effects[0])
	}

	_, effects = Reduce(s, EvShutdownCleanupFinished{TransactionID: txID, Outcome: ShutdownCleanupCompleted})
	if len(effects) != 3 {
		t.Fatalf("cleanup effects = %#v", effects)
	}
	if _, ok := effects[0].(EffSendResponseSync); !ok {
		t.Fatalf("first cleanup effect = %T, want response", effects[0])
	}
	if _, ok := effects[1].(EffCompleteShutdown); !ok {
		t.Fatalf("second cleanup effect = %T, want result", effects[1])
	}
	if _, ok := effects[2].(EffTerminateRuntime); !ok {
		t.Fatalf("third cleanup effect = %T, want terminal", effects[2])
	}
}

func TestReduceShutdownConsumesCleanupResultExactlyOnce(t *testing.T) {
	s, _ := reduceShutdown(New(), 1, "req-1", struct{}{})
	txID := s.Shutdown.ID
	s, _ = Reduce(s, EvShutdownSaveBarrierSucceeded{TransactionID: txID})
	s, effects := Reduce(s, EvShutdownCleanupFinished{TransactionID: txID, Outcome: ShutdownCleanupCompleted})
	if s.Shutdown != nil {
		t.Fatal("cleanup result must consume the shutdown transaction")
	}

	_, duplicateEffects := Reduce(s, EvShutdownCleanupFinished{TransactionID: txID, Outcome: ShutdownCleanupCompleted})
	if len(duplicateEffects) != 0 {
		t.Fatalf("duplicate cleanup result emitted effects: %#v (first=%#v)", duplicateEffects, effects)
	}
}

func TestReduceShutdownSaveFailureRollsBackWithoutCleanup(t *testing.T) {
	s, _ := reduceShutdown(New(), 1, "req-1", struct{}{})
	next, effects := Reduce(s, EvShutdownSaveBarrierFailed{TransactionID: s.Shutdown.ID, Err: "disk full"})
	if next.Lifecycle != LifecycleRunning || next.Shutdown != nil {
		t.Fatal("Save failure must roll back to Running")
	}
	for _, effect := range effects {
		if _, ok := effect.(EffReleaseFrameSandboxes); ok {
			t.Fatal("Save failure must not start cleanup")
		}
		if _, ok := effect.(EffTerminateRuntime); ok {
			t.Fatal("Save failure must not terminate runtime")
		}
	}
}

func TestQuiescingAdmissionAllowsReadOnlyAndFreezesMutation(t *testing.T) {
	s, _ := reduceShutdown(New(), 0, "", struct{}{})
	next, effects := Reduce(s, EvEvent{ConnID: 1, ReqID: "list", Event: EventListSessions})
	if _, ok := findEff[EffSendResponse](effects); !ok {
		t.Fatal("list-sessions must remain available while Quiescing")
	}
	if next.Lifecycle != LifecycleQuiescing {
		t.Fatal("read-only request changed lifecycle")
	}

	next, effects = Reduce(s, EvFrameCommandExited{FrameID: "late", ExitCode: 0})
	if len(effects) != 0 || next.Lifecycle != LifecycleQuiescing {
		t.Fatalf("late mutation was not neutralized: next=%#v effects=%#v", next, effects)
	}
}

func TestQuiescingAdmissionRejectsSurfaceSubscribeThatStartsBackendIO(t *testing.T) {
	s := New()
	s.Sessions["sess"] = Session{
		ID:     "sess",
		Frames: []SessionFrame{{ID: "frame"}},
	}
	s, _ = reduceShutdown(s, 0, "", struct{}{})

	next, effects := Reduce(s, EvCmdSurfaceSubscribe{
		ConnID:       1,
		ReqID:        "subscribe",
		SessionID:    "sess",
		SubscriberID: "browser",
	})

	if len(next.SurfaceSubs) != 0 {
		t.Fatalf("quiescing subscribe mutated SurfaceSubs: %#v", next.SurfaceSubs)
	}
	if _, ok := findEff[EffSurfaceSubscribeStart](effects); ok {
		t.Fatalf("quiescing subscribe started backend I/O: %#v", effects)
	}
	errEffect, ok := findEff[EffSendError](effects)
	if !ok || errEffect.Code != ErrCodeUnavailable {
		t.Fatalf("quiescing subscribe effects = %#v, want unavailable", effects)
	}
}

func TestQuiescingAdmissionDropsLateInternalDriverHookWithoutResponseIO(t *testing.T) {
	s, _ := reduceShutdown(New(), 0, "", struct{}{})

	next, effects := Reduce(s, EvDriverEvent{Event: "SessionEnd", SenderID: "late-frame"})

	if next.Lifecycle != LifecycleQuiescing {
		t.Fatalf("late internal hook changed lifecycle: %v", next.Lifecycle)
	}
	if len(effects) != 0 {
		t.Fatalf("late internal hook emitted response/backend effects: %#v", effects)
	}
}
