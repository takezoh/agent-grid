package state

import (
	"testing"
	"time"
)

func approvalTestState(t *testing.T) (State, SessionID, FrameID) {
	t.Helper()
	s := New()
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	s.Now = now
	sid := SessionID("sess-1")
	fid := FrameID("frame-1")
	s.Sessions = map[SessionID]Session{
		sid: {
			ID:      sid,
			Project: "/tmp/proj",
			Frames: []SessionFrame{{
				ID:      fid,
				Project: "/tmp/proj",
				Command: "codex",
				Driver:  pushStubState{},
			}},
			HeadFrameID: fid,
			Driver:      pushStubState{},
		},
	}
	return s, sid, fid
}

func TestApprovalRequestCreatedAndEmittedInSameCycle(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	next, effs := Reduce(s, EvSubsystem{
		FrameID:   fid,
		Kind:      SubsystemApprovalRequested,
		Timestamp: now,
		Payload: SubsystemPayload{
			Approval: &SubsystemApproval{
				ID:      "ap-1",
				Kind:    "command",
				Command: "rm -rf /",
				Reason:  "destructive",
			},
		},
	})
	pending := PendingApprovalsForSession(next, sid)
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	r := pending[0]
	if r.ID != "ap-1" || r.Status != ApprovalPending {
		t.Fatalf("approval = %+v", r)
	}
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatalf("default_decision = %q, want deny", r.DefaultDecision)
	}
	if !r.ExpiresAt.Equal(now.Add(DefaultApprovalTTL)) {
		t.Fatalf("expires_at = %v, want %v", r.ExpiresAt, now.Add(DefaultApprovalTTL))
	}
	if _, ok := findEff[EffBroadcastEvent](effs); !ok {
		// may be multiple broadcast events; scan
		found := false
		for _, e := range effs {
			if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalRequested {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected approval-requested broadcast in same Reduce cycle")
		}
	}
}

func TestApprovalAndQuestionLifecycleCreation(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemQuestionRequested, Timestamp: now,
		Payload: SubsystemPayload{Question: &SubsystemQuestion{ID: "q-1", Prompt: "continue?"}},
	})
	if got := len(PendingApprovalsForSession(s, sid)); got != 1 {
		t.Fatalf("approvals = %d", got)
	}
	if got := len(PendingQuestionsForSession(s, sid)); got != 1 {
		t.Fatalf("questions = %d", got)
	}
	// Duplicate driver events must not create a second entry.
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	if got := len(s.PendingApprovals[sid]); got != 1 {
		t.Fatalf("after duplicate: approvals map size = %d", got)
	}
}

func TestApprovalSingleWriterFirstCommit(t *testing.T) {
	// witness-approve-roundtrip-normal + witness-other-client-normal:
	// first-writer-wins — exactly one held JSON-RPC reply, loser is tombstone-only.
	s, _, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, effsA := Reduce(s, EvCmdApprovalRespond{
		ConnID: 1, ReqID: "rA", ApprovalID: "ap-1",
		Decision: ApprovalDecisionAccept, ClientInstanceID: "ci-A",
	})
	if _, ok := findEff[EffSendError](effsA); ok {
		t.Fatal("winner must not receive error")
	}
	// Invariant #1 is "exactly one held-JSON-RPC reply per request", so count EVERY
	// reply for this id regardless of shape. Filtering by Decision/Error here would
	// miss a second reply of a different shape — e.g. the
	// {Decision: Deny, Error: "connection-lost"} form that reduce_approval.go emits
	// on cancel/reap — and the reducer could reply twice while the test stayed green.
	var replies []EffReplyHeldApproval
	resolvedN := 0
	for _, e := range effsA {
		switch ef := e.(type) {
		case EffReplyHeldApproval:
			if ef.ApprovalID == "ap-1" {
				replies = append(replies, ef)
			}
		case EffBroadcastEvent:
			if ef.Name == EvtNameApprovalResolved {
				resolvedN++
			}
		}
	}
	// rejects counterexample: double held-reply on single accept (would resume agent twice)
	if len(replies) != 1 {
		t.Fatalf("held replies for ap-1 = %d (%+v), want exactly 1", len(replies), replies)
	}
	// Then pin the shape of that single reply separately.
	if replies[0].Decision != ApprovalDecisionAccept || replies[0].Error != "" {
		t.Fatalf("held reply = %+v, want decision=accept error=\"\"", replies[0])
	}
	if resolvedN != 1 {
		t.Fatalf("resolved broadcasts = %d, want 1", resolvedN)
	}
	r, _, ok := FindApproval(s, "ap-1")
	if !ok || r.Status != ApprovalResolved || r.Decision != ApprovalDecisionAccept {
		t.Fatalf("winner state = %+v ok=%v", r, ok)
	}
	if r.ResolvingClientInstanceID != "ci-A" {
		t.Fatalf("resolving_client_instance_id = %q", r.ResolvingClientInstanceID)
	}
	// Loser path: decision must not flip accept → deny.
	beforeDecision := r.Decision
	sB, effsB := Reduce(s, EvCmdApprovalRespond{
		ConnID: 2, ReqID: "rB", ApprovalID: "ap-1",
		Decision: ApprovalDecisionDeny, ClientInstanceID: "ci-B",
	})
	errEff, ok := findEff[EffSendError](effsB)
	if !ok {
		t.Fatal("loser must receive resolved-by-other error")
	}
	if errEff.Code != ErrCodeResolvedByOther {
		t.Fatalf("code = %q, want %q", errEff.Code, ErrCodeResolvedByOther)
	}
	if errEff.Details["decision"] != "accept" {
		t.Fatalf("details.decision = %v", errEff.Details["decision"])
	}
	if errEff.Details["resolving_client_instance_id"] != "ci-A" {
		t.Fatalf("details.resolving = %v", errEff.Details["resolving_client_instance_id"])
	}
	// No second resolve broadcast / held reply for the loser path.
	for _, e := range effsB {
		switch ef := e.(type) {
		case EffBroadcastEvent:
			if ef.Name == EvtNameApprovalResolved {
				t.Fatal("loser path must not re-broadcast EvtApprovalResolved")
			}
		case EffReplyHeldApproval:
			t.Fatalf("loser path must not reply held approval: %+v", ef)
		}
	}
	// Read the state the loser Reduce actually produced. State is copy-on-write, so
	// re-reading the pre-Reduce `s` here would compare a value against itself and the
	// assertion could never fail.
	r, _, ok = FindApproval(sB, "ap-1")
	if !ok {
		t.Fatal("loser path dropped the approval record")
	}
	if r.Decision != beforeDecision {
		t.Fatalf("late respond overwrote decision: got %q want %q", r.Decision, beforeDecision)
	}
	if r.ResolvingClientInstanceID != "ci-A" {
		t.Fatalf("loser path overwrote resolver: %q", r.ResolvingClientInstanceID)
	}
}

func TestApprovalExpiryDecisionCaptureAtCreation(t *testing.T) {
	// witness-expiry-normal: tick applies creation-time default_decision only.
	s, _, fid := approvalTestState(t)
	created := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: created,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	// Mid-flight "policy mutation" cannot flip captured default_decision.
	// (DefaultDecision is already deny; we assert expiry uses it.)
	r, _, _ := FindApproval(s, "ap-1")
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatalf("captured default = %q", r.DefaultDecision)
	}
	// Tick past expires_at.
	s, effs := Reduce(s, EvTick{Now: created.Add(DefaultApprovalTTL + time.Second), N: 1})
	r, _, ok := FindApproval(s, "ap-1")
	if !ok || r.Status != ApprovalExpired {
		t.Fatalf("after expiry: %+v ok=%v", r, ok)
	}
	if r.Decision != ApprovalDecisionDeny {
		t.Fatalf("expired decision = %q, want deny", r.Decision)
	}
	if r.ResolutionReason != ApprovalReasonExpired {
		t.Fatalf("reason = %q", r.ResolutionReason)
	}
	// Count every reply for this id (see the shape-agnostic rationale in
	// TestApprovalSingleWriterFirstCommit), then assert the captured default applies.
	var expiryReplies []EffReplyHeldApproval
	foundBroadcast := false
	for _, e := range effs {
		if re, ok := e.(EffReplyHeldApproval); ok && re.ApprovalID == "ap-1" {
			expiryReplies = append(expiryReplies, re)
		}
		if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalResolved {
			foundBroadcast = true
		}
	}
	if len(expiryReplies) != 1 || !foundBroadcast {
		t.Fatalf("expiry replies for ap-1 = %d (%+v) broadcast=%v, want reply exactly 1",
			len(expiryReplies), expiryReplies, foundBroadcast)
	}
	if expiryReplies[0].Decision != ApprovalDecisionDeny {
		t.Fatalf("expiry reply decision = %q, want the creation-time default (deny)",
			expiryReplies[0].Decision)
	}

	// Forbidden: post-expiry client respond must not apply or re-reply.
	// rejects counterexample: accepting after TTL rewrites the expired decision
	sLate, lateEffs := Reduce(s, EvCmdApprovalRespond{
		ConnID: 1, ReqID: "late", ApprovalID: "ap-1",
		Decision: ApprovalDecisionAccept, ClientInstanceID: "ci-late",
	})
	lateErr, ok := findEff[EffSendError](lateEffs)
	if !ok || lateErr.Code != ErrCodeResolvedByOther {
		t.Fatalf("post-expiry respond error = %+v ok=%v, want resolved_by_other", lateErr, ok)
	}
	for _, e := range lateEffs {
		if _, isReply := e.(EffReplyHeldApproval); isReply {
			t.Fatal("post-expiry respond must not emit another held reply")
		}
	}
	// Read the post-Reduce state (see the copy-on-write note in
	// TestApprovalSingleWriterFirstCommit) — otherwise this re-asserts the value the
	// test already checked and cannot fail.
	r, _, ok = FindApproval(sLate, "ap-1")
	if !ok {
		t.Fatal("post-expiry respond dropped the tombstone")
	}
	if r.Decision != ApprovalDecisionDeny || r.Status != ApprovalExpired {
		t.Fatalf("post-expiry respond mutated tombstone: %+v", r)
	}
}

func TestApprovalRespondInvalidDecisionRejected(t *testing.T) {
	// witness-race-adversarial (malformed decision): pending stays, no held reply.
	// rejects counterexample: treating "maybe" as accept/deny or leaving agent stuck without error
	s, _, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, effs := Reduce(s, EvCmdApprovalRespond{
		ConnID: 1, ReqID: "bad", ApprovalID: "ap-1",
		Decision: ApprovalDecision("maybe"), ClientInstanceID: "ci-bad",
	})
	errEff, ok := findEff[EffSendError](effs)
	if !ok || errEff.Code != ErrCodeInvalidArgument {
		t.Fatalf("error = %+v ok=%v, want invalid_argument", errEff, ok)
	}
	for _, e := range effs {
		if _, isReply := e.(EffReplyHeldApproval); isReply {
			t.Fatal("invalid decision must not reply held approval")
		}
	}
	r, _, ok := FindApproval(s, "ap-1")
	if !ok || r.Status != ApprovalPending {
		t.Fatalf("pending must remain after invalid decision: %+v ok=%v", r, ok)
	}
}

func TestApprovalExpiryTOCTOU(t *testing.T) {
	// Same as capture-at-creation: even if we mutate a local policy flag,
	// the ApprovalRequest.DefaultDecision is what expiry applies.
	s, _, fid := approvalTestState(t)
	created := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: created,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{
			ID: "ap-1", Kind: "command", AutoApprove: false,
		}},
	})
	// Simulate "live policy flipped to accept" by only changing unrelated state.
	// DefaultDecision on the request remains deny.
	r, _, _ := FindApproval(s, "ap-1")
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatal("expected deny captured at creation")
	}
	s, _ = Reduce(s, EvTick{Now: created.Add(DefaultApprovalTTL + time.Millisecond), N: 2})
	r, _, _ = FindApproval(s, "ap-1")
	if r.Decision != ApprovalDecisionDeny {
		t.Fatalf("TOCTOU: decision flipped to %q", r.Decision)
	}
}

func TestApprovalLifecycleTeardownReap(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemQuestionRequested, Timestamp: now,
		Payload: SubsystemPayload{Question: &SubsystemQuestion{ID: "q-1", Prompt: "?"}},
	})
	// Root frame vanish evicts the session and reaps pending domain objects.
	s, effs := Reduce(s, EvFrameVanished{FrameID: fid})
	if len(PendingApprovalsForSession(s, sid)) != 0 {
		t.Fatalf("pending approvals remain: %v", PendingApprovalsForSession(s, sid))
	}
	if len(PendingQuestionsForSession(s, sid)) != 0 {
		t.Fatalf("pending questions remain: %v", PendingQuestionsForSession(s, sid))
	}
	if _, ok := s.PendingApprovals[sid]; ok {
		t.Fatal("session approval map should be fully reaped")
	}
	if _, ok := s.PendingQuestions[sid]; ok {
		t.Fatal("session question map should be fully reaped")
	}
	// Count every reply for these ids regardless of shape (Error/Decision), not just
	// the connection-lost form. A filtered counter here would miss a second reply of
	// a different shape emitted by another reap path — reduce_approval.go has three
	// EffReplyHeldApproval call sites (frame-scoped cancel, session-eviction reap,
	// TTL expiry), and this test only exercises the shared teardown path, so a
	// future session-eviction change that also replies (e.g. with the captured
	// default_decision) would double-reply while a shape-filtered count stayed at 1.
	var replies []EffReplyHeldApproval
	var qReplies []EffReplyHeldQuestion
	var cancelBroadcastN int
	for _, e := range effs {
		switch ef := e.(type) {
		case EffReplyHeldApproval:
			if ef.ApprovalID == "ap-1" {
				replies = append(replies, ef)
			}
		case EffReplyHeldQuestion:
			if ef.QuestionID == "q-1" {
				qReplies = append(qReplies, ef)
			}
		case EffBroadcastEvent:
			if ef.Name == EvtNameApprovalResolved || ef.Name == EvtNameQuestionResolved {
				cancelBroadcastN++
			}
		}
	}
	if len(replies) != 1 || len(qReplies) != 1 {
		t.Fatalf("drain replies for ap-1/q-1 = %d/%d (%+v / %+v), want exactly 1 each",
			len(replies), len(qReplies), replies, qReplies)
	}
	if replies[0].Error != "connection-lost" || qReplies[0].Error != "connection-lost" {
		t.Fatalf("teardown reply shape = %+v / %+v, want connection-lost", replies[0], qReplies[0])
	}
	// Exactly one pending approval and one pending question exist, so exactly 2
	// cancel broadcasts (not merely ">=2") is the precise, non-loose invariant.
	if cancelBroadcastN != 2 {
		t.Fatalf("cancel broadcasts = %d, want exactly 2 (1 approval + 1 question)", cancelBroadcastN)
	}
}

func TestApprovalAutoApproveResolvesInSameCycle(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, effs := Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{
			ID: "ap-auto", Kind: "command", AutoApprove: true,
		}},
	})
	if len(PendingApprovalsForSession(s, sid)) != 0 {
		t.Fatal("auto-approve must not leave pending")
	}
	r, _, ok := FindApproval(s, "ap-auto")
	if !ok || r.Status != ApprovalResolved || r.Decision != ApprovalDecisionAccept {
		t.Fatalf("auto tombstone = %+v ok=%v", r, ok)
	}
	if r.ResolvingClientInstanceID != AutoResolvingClientInstanceID {
		t.Fatalf("resolving = %q", r.ResolvingClientInstanceID)
	}
	var sawReply, sawResolved bool
	for _, e := range effs {
		if re, ok := e.(EffReplyHeldApproval); ok && re.Decision == ApprovalDecisionAccept {
			sawReply = true
		}
		if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalResolved {
			sawResolved = true
		}
	}
	if !sawReply || !sawResolved {
		t.Fatalf("reply=%v resolved=%v", sawReply, sawResolved)
	}
}

func TestSubscribeDoesNotReplayHumanInput(t *testing.T) {
	// UAC-043 / FR-008 at the layer that would plausibly implement replay.
	//
	// The gateway-side pin (server/api: TestGatewayLifecycle_ReconnectDoesNotReplay-
	// ResolvedApproval) fakes out the daemon subscribe path, so it can only prove that
	// AttachLifecycleWS holds no replay buffer. The reducer is where a "re-emit pending
	// state on resubscribe" implementation would live, so pin it here too.
	//
	// rejects counterexample: reconstructing already-resolved requests from state on
	// resubscribe and re-broadcasting them as ar/ax.
	s, _, fid := approvalTestState(t)
	now := s.Now

	// One still-pending approval and one already-resolved approval in state.
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-pending", Kind: "command"}},
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-resolved", Kind: "command"}},
	})
	s, _ = Reduce(s, EvCmdApprovalRespond{
		ConnID: 1, ReqID: "r1", ApprovalID: "ap-resolved",
		Decision: ApprovalDecisionAccept, ClientInstanceID: "ci-A",
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemQuestionRequested, Timestamp: now,
		Payload: SubsystemPayload{Question: &SubsystemQuestion{ID: "q-pending", Prompt: "Continue?"}},
	})

	// Preconditions: state really does hold the pending/resolved records, so the
	// scan below cannot pass by having nothing to scan.
	if r, _, ok := FindApproval(s, "ap-pending"); !ok || r.Status != ApprovalPending {
		t.Fatalf("precondition: ap-pending = %+v ok=%v", r, ok)
	}
	if r, _, ok := FindApproval(s, "ap-resolved"); !ok || r.Status != ApprovalResolved {
		t.Fatalf("precondition: ap-resolved = %+v ok=%v", r, ok)
	}

	// A fresh client subscribes (the reconnect step).
	_, effs := Reduce(s, EvCmdSubscribe{ConnID: 9, ReqID: "sub", Filters: nil})

	// Subscribe must still do its own job — otherwise the forbidden-effect scan
	// below would be vacuous.
	if len(effs) == 0 {
		t.Fatal("subscribe produced no effects; forbidden-effect scan would be vacuous")
	}
	sessionsChanged := false
	for _, e := range effs {
		if _, ok := e.(EffBroadcastSessionsChanged); ok {
			sessionsChanged = true
		}
	}
	if !sessionsChanged {
		t.Fatalf("subscribe must emit the initial sessions snapshot, got %+v", effs)
	}

	for _, e := range effs {
		switch ef := e.(type) {
		case EffBroadcastEvent:
			switch ef.Name {
			case EvtNameApprovalRequested, EvtNameApprovalResolved,
				EvtNameQuestionRequested, EvtNameQuestionResolved:
				t.Fatalf("subscribe must not replay human-input events, got %q (%+v)", ef.Name, ef)
			}
		// Forbidden regardless of transport shape: no held-reply side effects either.
		case EffReplyHeldApproval:
			t.Fatalf("subscribe must not touch held approvals, got %+v", ef)
		case EffReplyHeldQuestion:
			t.Fatalf("subscribe must not touch held questions, got %+v", ef)
		// The subscribe *response body* is the channel the protocol reserves for a
		// resubscribe payload (proto.RespPendingHumanInput is documented as "the
		// authoritative pending set returned on resubscribe"). A replay implemented
		// there would emit no broadcast at all, so assert the body stays empty.
		case EffSendResponse:
			if ef.ConnID == 9 && ef.Body != nil {
				t.Fatalf("subscribe response body must not carry a pending payload, got %#v", ef.Body)
			}
		case EffSendResponseSync:
			if ef.ConnID == 9 && ef.Body != nil {
				t.Fatalf("subscribe response body must not carry a pending payload, got %#v", ef.Body)
			}
		}
	}
}
