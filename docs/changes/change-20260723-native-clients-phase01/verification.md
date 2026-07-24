---
change: change-20260723-native-clients-phase01
role: verification
---

# Verification

## Success conditions

- Phase 0 exit: a fake agent raises an approval; two subscribed clients under distinct client-instance-ids observe the pending request; one answers; both observe the resolution with `resolving_client_instance_id` set; a reconnecting third client's resubscribe snapshot converges with the still-connected clients.
- Phase 0 lifecycle exit: an in-flight ApprovalRequest cancels cleanly on frame/session teardown without leaking a goroutine or map entry; an expired ApprovalRequest applies deny-by-default from the value captured at creation regardless of any mid-flight driver-policy mutation.
- Phase 1 exit: every generated SDK (C# / Swift / Kotlin / TS) observes the identical typed event/command sequence when replayed against the simulator's recorded scenario; the compatibility CI gate fails closed on undeclared SDK surface, inconclusive scans, and new-SDK targets that skip the shared recorded-scenario suite.
- Docs: `docs lint` is zero-error; all 12 accepted ADRs (plus 1 rejected) originatedFrom this change; `validate_plan.py` passes with the source draft attached.

## Verification matrix (per-contract)

### `contract-approval-question-envelope` (integration_contract)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-envelope-conflict-e2e` | T1 | FR-P0-03, FR-P0-04, FR-P0-05, FR-P0-12 | server/api scenario test with two subscribed WS clients under distinct client-instance-ids racing CmdApprovalRespond | `cd src && go test ./server/api -run TestApprovalConflict` |
| `verify-envelope-fixture-round-trip` | T0 | FR-P1-01, FR-P1-12 | wire-fixtures pipeline (adr-20260705-wire-fixtures-pipeline extended) round-trips the new Evt* frames against protocol/events.schema.json. | `cd src && go test ./server/api -run TestWireFixturesApproval` |
| `verify-envelope-expiry-broadcast` | T0 | FR-P0-06 | reduce_expiry_test.go simulates tick past expires_at with mutated per-session policy. | `cd src && go test ./host/state -run TestApprovalExpiryDecisionCaptureAtCreation` |
| `verify-envelope-question-round-trip` | T0 | FR-P0-07 | Fixture round-trip for a QuestionRequest carrying a single free-text answer field on the kind-discriminated HumanInputRequest envelope. | `cd src && go test ./host/proto -run TestWireFixturesQuestion` |
| `verify-envelope-lifecycle-creation` | T0 | FR-P0-01, FR-P0-02, FR-P1-01 | reduce_event_test.go: driver approval-requested DEv arrives; assert exactly one pending ApprovalRequest with expected fields is created and EvtApprovalRequested is emitted in the same cycle; assert wire round-trip validates against protocol/events.schema.json when it lands. | `cd src && go test ./host/state -run TestApprovalRequestCreatedAndEmittedInSameCycle` |

**Witnesses**

- `witness-envelope-normal-round-trip` [normal] risks: (normal)
    - **precondition**: One client is subscribed to session S under client-instance-id ci-A; one pending ApprovalRequest ID r1 exists in state.
    - **stimulus**: Client A submits CmdApprovalRespond(r1, decision=accept).
    - **expected**: ApprovalRequest.status transitions pending->resolved with resolving_client_instance_id=ci-A and decision=accept.; Every subscriber observes exactly one EvtApprovalResolved(r1, decision=accept, resolving_client_instance_id=ci-A).
    - **forbidden**: EvtApprovalResolved is broadcast more than once for r1.; ApprovalRequest.decided_by is empty on the emitted event.
    - **verifies**: `verify-envelope-conflict-e2e`
- `witness-envelope-conflict-adversarial` [adversarial] risks: concurrency, boundary, conflicting_evidence
    - **precondition**: Two clients A (ci-A) and B (ci-B) are subscribed; a pending ApprovalRequest r1 exists.
    - **stimulus**: Both submit CmdApprovalRespond(r1) with opposing decisions inside the same event-loop tick window.
    - **expected**: Exactly one of A/B receives success; the other receives resolved-by-other carrying (winning_decision, winning_ci).; Both A and B observe exactly one EvtApprovalResolved with the winning decision.
    - **forbidden**: Both clients receive success.; The loser silently succeeds or receives a generic error without the authoritative decision.; Two EvtApprovalResolved frames are broadcast for r1.
    - **verifies**: `verify-envelope-conflict-e2e`
- `witness-envelope-expiry-normal` [normal] risks: (normal)
    - **precondition**: ApprovalRequest r1 was created with expires_at=t0+30s and default policy captured at creation as deny.
    - **stimulus**: Reduce is invoked with a tick value at t0+31s.
    - **expected**: r1.status transitions pending->expired.; EvtApprovalResolved(r1, decision=deny, resolution_reason=expired) is broadcast to every subscriber.
    - **forbidden**: expires_at is extended by an agent-side request after creation.; The mutated per-session policy at expiry time flips decision to accept.
    - **verifies**: `verify-envelope-expiry-broadcast`
- `witness-envelope-broadcast-shape-adversarial` [adversarial] risks: boundary, unsupported_environment
    - **precondition**: A pre-Phase-0 browser client (ADR-0021 hand-written TS) is connected and only understands existing k in {v, tt, et, n} frames.
    - **stimulus**: The daemon broadcasts an EvtApprovalRequested using a new k=... value on the shared subscription surface.
    - **expected**: The new frame is delivered on the same subscription surface as existing frames.; The browser's socket decoder skips unknown k values without disconnecting or crashing.
    - **forbidden**: The new frame is delivered on a separate subscription surface a legacy client would need to opt into.; A legacy client crashes on the unknown k value.
    - **verifies**: `verify-envelope-fixture-round-trip`
- `witness-envelope-creation-normal` [normal] risks: (normal)
    - **precondition**: A session S is subscribed by one client A.
    - **stimulus**: Driver emits an approval-requested driver event for a new ID r1.
    - **expected**: state.PendingApprovals[S] contains r1 with status=pending, kind, command/path, reason, created_at, expires_at populated.; A observes EvtApprovalRequested(r1) in the same Reduce cycle.
    - **forbidden**: The Evt* frame arrives in a later Reduce cycle.; A duplicate driver event creates a second pending entry.
    - **verifies**: `verify-envelope-lifecycle-creation`
- `witness-envelope-question-answer-normal` [normal] risks: (normal)
    - **precondition**: A driver emits a request-user-input event for a QuestionRequest q1.
    - **stimulus**: The QuestionRequest is serialized onto the wire.
    - **expected**: The envelope's answer field is a single free-text string; kind discriminator identifies it as HumanInputRequest.free_text.; The SDK exposes a typed free-text answer accessor.
    - **forbidden**: Answer field is a structured object.; Kind discriminator is absent.
    - **verifies**: `verify-envelope-question-round-trip`
- `witness-envelope-question-answer-adversarial` [adversarial] risks: malformed, boundary
    - **precondition**: A malformed CmdQuestionRespond payload carries a structured object where a free-text string is required.
    - **stimulus**: The REST/WS layer processes the payload.
    - **expected**: Request is rejected with 400 at the wire layer; QuestionRequest state is unchanged.
    - **forbidden**: The structured object is silently coerced to a string.; QuestionRequest state is corrupted.
    - **verifies**: `verify-envelope-question-round-trip`
- `witness-envelope-broadcast-shape-normal` [normal] risks: (normal)
    - **precondition**: A subscribed client is connected.
    - **stimulus**: Daemon broadcasts EvtApprovalRequested.
    - **expected**: Frame arrives on the same subscription surface as existing k in {v, tt, et, n}.; k value identifies it as an approval-requested frame.
    - **forbidden**: Frame arrives on a separate subscription negotiation.; k discriminator is absent.
    - **verifies**: `verify-envelope-fixture-round-trip`, `verify-envelope-lifecycle-creation`
- `witness-envelope-expiry-adversarial` [adversarial] risks: stale, boundary
    - **precondition**: ApprovalRequest r1 has expires_at at t0+30s; a client attempts an out-of-order CmdApprovalRespond at t0+31s (after Reduce fired the expiry).
    - **stimulus**: Reduce processes the late CmdApprovalRespond.
    - **expected**: The command receives a resolved-by-other-or-expired error carrying decision=deny.; No second EvtApprovalResolved is broadcast.
    - **forbidden**: The late command mutates r1's decision.; A second EvtApprovalResolved is broadcast.
    - **verifies**: `verify-envelope-expiry-broadcast`, `verify-envelope-conflict-e2e`
- `witness-envelope-decided-by-normal` [normal] risks: (normal)
    - **precondition**: Client A with ci-A is subscribed.
    - **stimulus**: A submits CmdApprovalRespond and the daemon broadcasts EvtApprovalResolved.
    - **expected**: The broadcast frame carries resolving_client_instance_id=ci-A.
    - **forbidden**: resolving_client_instance_id is empty or missing on the broadcast.
    - **verifies**: `verify-envelope-conflict-e2e`

### `contract-approval-question-lifecycle` (state_lifecycle)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-lifecycle-teardown` | T0 | FR-P0-11, NFR-06 | reduce_teardown_test.go creates a session with one pending ApprovalRequest and one pending QuestionRequest, then processes a teardown DEv. | `cd src && go test ./host/state -run TestApprovalLifecycleTeardownReap` |
| `verify-lifecycle-toctou-expiry` | T0 | FR-P0-06, NFR-02 | reduce_expiry_test.go creates r with captured default_decision=deny, mutates the driver's live policy to accept, then ticks past expires_at. | `cd src && go test ./host/state -run TestApprovalExpiryTOCTOU` |
| `verify-lifecycle-monotonic` | T0 | FR-P0-02 | reduce_fuzz_test.go extended with a duplicate approval-requested DEv trace. | `cd src && go test ./host/state -run FuzzApprovalLifecycleMonotonic` |
| `verify-lifecycle-creation-and-question` | T0 | FR-P0-01, FR-P0-02, FR-P0-07 | reduce_event_test.go: driver approval-requested + question-requested DEvs; assert exactly one pending ApprovalRequest and one pending QuestionRequest per underlying id, both visible in state before the next Reduce call. | `cd src && go test ./host/state -run TestApprovalAndQuestionLifecycleCreation` |

**Witnesses**

- `witness-lifecycle-teardown-normal` [normal] risks: (normal)
    - **precondition**: Session S has one pending ApprovalRequest r1 and one pending QuestionRequest q1.
    - **stimulus**: A frame-teardown DEv for S is delivered to Reduce.
    - **expected**: r1 and q1 both transition to cancelled in the same Reduce cycle.; state.PendingApprovals[S] and PendingQuestions[S] are empty.; The held JSON-RPC ids for r1 and q1 receive a connection-lost error.
    - **forbidden**: A pending entry outlives the teardown.; A goroutine remains waiting on r1's or q1's reply.
    - **verifies**: `verify-lifecycle-teardown`
- `witness-lifecycle-expiry-toctou` [adversarial] risks: stale, boundary
    - **precondition**: r was created at t0 with default_decision=deny captured onto r.
    - **stimulus**: Driver-side policy is mutated to accept at t0+15s; Reduce tick fires at t0+31s (past expires_at).
    - **expected**: r.status transitions to expired with decision=deny (captured), not accept.
    - **forbidden**: decision flips to accept because the expiry reducer re-read the driver policy at expiry time.
    - **verifies**: `verify-lifecycle-toctou-expiry`
- `witness-lifecycle-creation-normal` [normal] risks: (normal)
    - **precondition**: Session S exists.
    - **stimulus**: Driver emits an approval-requested DEv for r1 and a question-requested DEv for q1.
    - **expected**: state.PendingApprovals[S] contains exactly one r1 with status=pending.; state.PendingQuestions[S] contains exactly one q1 with status=pending.
    - **forbidden**: A duplicate DEv creates a shadow entry.; The pending entry is not visible in the next State snapshot.
    - **verifies**: `verify-lifecycle-creation-and-question`, `verify-lifecycle-monotonic`
- `witness-lifecycle-monotonic-adversarial` [adversarial] risks: repeated_usage, boundary
    - **precondition**: r1 already exists in state (status=pending or resolved).
    - **stimulus**: Driver re-emits an approval-requested DEv for r1.
    - **expected**: No second ApprovalRequest is created.; r1's status is not moved backward.
    - **forbidden**: A duplicate DEv creates a second pending entry.; r1 transitions backward from resolved to pending.
    - **verifies**: `verify-lifecycle-monotonic`
- `witness-lifecycle-teardown-adversarial` [adversarial] risks: recovery, boundary, concurrency
    - **precondition**: Session S has r1 pending and a goroutine is blocked waiting for its answer.
    - **stimulus**: A teardown DEv for S races with an in-flight CmdApprovalRespond for r1.
    - **expected**: Whichever event Reduce processes first commits (resolved by respond, or cancelled by teardown).; The other event is a no-op.; No goroutine or map entry outlives Reduce's teardown cycle.
    - **forbidden**: Both events mutate state.; A goroutine remains blocked on r1's reply after teardown.
    - **verifies**: `verify-lifecycle-teardown`
- `witness-lifecycle-expiry-normal` [normal] risks: (normal)
    - **precondition**: r1 was created with default_decision=deny captured onto r1.
    - **stimulus**: Reduce tick fires at t0+expires_at.
    - **expected**: r1.status transitions pending->expired with decision=deny.
    - **forbidden**: r1 remains pending past expires_at.
    - **verifies**: `verify-lifecycle-toctou-expiry`

### `contract-approval-resolution-single-writer` (concurrency)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-single-writer-race` | T0 | FR-P0-04, FR-P0-05 | reduce_event_test.go enqueues two CmdApprovalRespond events for the same ID and asserts the second Reduce call observes status=resolved from the first. | `cd src && go test ./host/state -run TestApprovalSingleWriterFirstCommit` |
| `verify-single-writer-fuzz` | T0 | FR-P0-05 | reduce_fuzz_test.go extended with a two-response fuzzer. | `cd src && go test ./host/state -run FuzzApprovalSingleWriter` |

**Witnesses**

- `witness-single-writer-normal` [normal] risks: (normal)
    - **precondition**: Pending r exists; single client A submits one CmdApprovalRespond.
    - **stimulus**: Reduce processes A's event.
    - **expected**: r.status transitions pending->resolved with A's decision.; No rejection effect is produced.
    - **forbidden**: A second implicit commit occurs.; A rejection effect is emitted for A.
    - **verifies**: `verify-single-writer-race`
- `witness-single-writer-race-adversarial` [adversarial] risks: concurrency, boundary
    - **precondition**: Pending r exists; both A and B submit CmdApprovalRespond and both events are enqueued.
    - **stimulus**: Reduce processes them sequentially.
    - **expected**: The first-processed event commits r.decision.; The second-processed event returns state unchanged plus a resolved-by-other rejection effect carrying the winner's decision.
    - **forbidden**: Both events mutate state.; The second event silently succeeds.; The atomic State snapshot shows two different decisions across consecutive reads.
    - **verifies**: `verify-single-writer-race`, `verify-single-writer-fuzz`

### `contract-reconnect-authoritative-resubscribe` (failure_recovery)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-reconnect-e2e` | T1 | FR-P0-08 | gateway_inbound_test.go: subscribe A, request approval, disconnect A, resolve via B, reconnect A, assert A's resubscribe snapshot matches state. | `cd src && go test ./server/api -run TestReconnectAuthoritativeResubscribe` |
| `verify-reconnect-scope-consistency` | T1 | FR-P0-08 | gateway_inbound_test.go: two clients A and B; race B's disconnect+reconnect against A's CmdApprovalRespond so B's resubscribe snapshot and A's post-resolution broadcast are the two sources of truth. | `cd src && go test ./server/api -run TestReconnectSnapshotBroadcastConvergence` |
| `verify-reconnect-old-client-fixture` | T0 | FR-P1-11, NFR-05 | wire-fixtures pipeline: pre-Phase-0 hello fixture decoded by pre-Phase-1 TS decoder against the new server payload. | `cd clients/ui && npm run test:unit -- wire/reconnect-back-compat` |

**Witnesses**

- `witness-reconnect-normal` [normal] risks: (normal)
    - **precondition**: S has one pending r1; client A was subscribed and is now disconnected.
    - **stimulus**: A reconnects and resubscribes to S while r1 is still pending.
    - **expected**: A's resubscribe response includes r1 in status=pending.; A's local view now shows r1 pending.
    - **forbidden**: A's resubscribe response omits r1.; A's view shows r1 in a status different from state's.
    - **verifies**: `verify-reconnect-e2e`
- `witness-reconnect-scope-consistency` [adversarial] risks: stale, concurrency, recovery, partial_data
    - **precondition**: S has one pending r1; A and B are both subscribed; B disconnects.
    - **stimulus**: A submits CmdApprovalRespond(r1) at t0; B reconnects and resubscribes at t0+dt with the two orderings (snapshot before A's commit vs after).
    - **expected**: Order 1 (snapshot before commit): B's snapshot has r1 pending; the subsequent EvtApprovalResolved broadcast converges B's view to r1 resolved matching A's view.; Order 2 (snapshot after commit): B's snapshot omits r1; no divergence with A.
    - **forbidden**: A's and B's final views diverge on r1's status.
    - **verifies**: `verify-reconnect-scope-consistency`
- `witness-reconnect-old-client-decodes-adversarial` [adversarial] risks: boundary, stale, unsupported_environment
    - **precondition**: A pre-Phase-1 hand-written TS wire decoder is deployed at a browser client.
    - **stimulus**: The daemon (Phase-1) emits a resubscribe/hello payload with the additive pending-approval fields.
    - **expected**: Decoding succeeds; the additive fields are ignored without disconnecting.; Existing fields (session list, viewUpdate) still populate the view.
    - **forbidden**: Decoding fails on unknown fields.; Existing fields are dropped or corrupted.
    - **verifies**: `verify-reconnect-old-client-fixture`

### `contract-capability-negotiation-policy` (migration_compatibility)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-capability-bundled-no-extra-round-trip` | T1 | FR-P1-03, NFR-04 | server/api handshake test with bundled version constant equal on both sides. | `cd src && go test ./server/api -run TestCapabilityBundledSingleRoundTrip` |
| `verify-capability-old-client-vs-new-daemon` | T1 | FR-P1-04 | component-simulator drives a fixed-old-version SDK against a fixed-new-version simulated daemon. | `cd src && go test ./server/api -run TestCapabilityOldClientNewDaemon` |
| `verify-capability-new-client-vs-old-daemon` | T1 | FR-P1-04 | component-simulator drives a fixed-new-version SDK against a fixed-old-version simulated daemon. | `cd src && go test ./server/api -run TestCapabilityNewClientOldDaemon` |

**Witnesses**

- `witness-capability-bundled-normal` [normal] risks: (normal)
    - **precondition**: Bundled shell and its co-shipped daemon share build commit sha1=X.
    - **stimulus**: The shell connects and the handshake completes.
    - **expected**: Handshake trace contains exactly one hello request/response.; No capability probe frames follow.
    - **forbidden**: An additional round-trip for capability negotiation is observed.
    - **verifies**: `verify-capability-bundled-no-extra-round-trip`
- `witness-capability-remote-adversarial` [adversarial] risks: unsupported_environment, boundary, stale, unknown
    - **precondition**: A remote client compiled against protocol version 2 connects to a daemon compiled against version 1; capability C exists only in v2.
    - **stimulus**: Handshake completes with the daemon's capabilities.schema.json response omitting C.
    - **expected**: The client disables C in its UI.; The client emits no command targeting C for the lifetime of the connection.
    - **forbidden**: The client speculatively invokes C.; The client crashes or hangs on the daemon's undefined-capability error.
    - **verifies**: `verify-capability-old-client-vs-new-daemon`, `verify-capability-new-client-vs-old-daemon`
- `witness-capability-repeated-connections-adversarial` [adversarial] risks: repeated_usage, scale
    - **precondition**: A shell repeatedly reconnects (e.g. 10 reconnect cycles under a bundled-axis daemon).
    - **stimulus**: Each reconnect performs the handshake.
    - **expected**: Each reconnect's handshake trace contains exactly one hello request/response.; Total network round-trips scale linearly (one per reconnect), never quadratic.
    - **forbidden**: Any reconnect performs multiple round-trips.
    - **verifies**: `verify-capability-bundled-no-extra-round-trip`
- `witness-capability-remote-normal` [normal] risks: (normal)
    - **precondition**: Remote client compiled against protocol v1; daemon compiled against protocol v1; no capability gap.
    - **stimulus**: Handshake completes.
    - **expected**: Every capability the client compiled against is available; no degrade is triggered.; The client's commands issue normally.
    - **forbidden**: A capability is degraded despite the daemon declaring it.
    - **verifies**: `verify-capability-old-client-vs-new-daemon`

### `contract-sdk-generation-determinism` (integration_contract)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-sdk-determinism-diff` | T2 | FR-P1-02, NFR-01 | compatibility profile runs generation twice on a clean checkout and diffs the outputs. | `scripts/run-verification-profile.sh pr compatibility` |

**Witnesses**

- `witness-sdk-determinism-normal` [normal] risks: (normal)
    - **precondition**: protocol/ HEAD is commit-X; pinned quicktype version-Y (npm lockfile).
    - **stimulus**: Run the generation twice from a clean workspace.
    - **expected**: The two runs produce byte-identical file trees for all four SDK targets.
    - **forbidden**: A timestamp or UUID appears in output.; A transitive dep version differs between runs.
    - **verifies**: `verify-sdk-determinism-diff`
- `witness-sdk-determinism-adversarial` [adversarial] risks: unsupported_environment, boundary
    - **precondition**: A PR bumps a transitive template dep without pinning it.
    - **stimulus**: compatibility profile runs generation twice.
    - **expected**: Diff is non-zero and CI fails; report names the drifting file.
    - **forbidden**: CI passes despite non-empty diff.
    - **verifies**: `verify-sdk-determinism-diff`

### `contract-simulator-recorded-scenario-replay` (user_observability)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-simulator-per-sdk-e2e` | T1 | FR-P1-07, FR-P1-08 | each generated SDK's e2e suite drives the sim server against the same recorded scenario and diffs observed sequence vs fixture. | `scripts/run-verification-profile.sh pr compatibility` |
| `verify-simulator-cross-sdk-parallel` | T1 | FR-P1-07 | run C# and TS suites concurrently against a shared sim server instance. | `scripts/run-verification-profile.sh pr compatibility` |

**Witnesses**

- `witness-simulator-normal` [normal] risks: (normal)
    - **precondition**: Fixture protocol/simulator/fixtures/approval-round-trip.jsonl exists.
    - **stimulus**: TS SDK drives the sim server against this fixture.
    - **expected**: TS SDK's observed sequence equals the fixture's canonical sequence.
    - **forbidden**: SDK sees an out-of-order frame or missing frame.; SDK falls back to a live agent.
    - **verifies**: `verify-simulator-per-sdk-e2e`
- `witness-simulator-parallel-replay-adversarial` [adversarial] risks: concurrency, scale, partial_data
    - **precondition**: C# and TS suites both drive the same sim server instance.
    - **stimulus**: Both start replay of the same fixture concurrently.
    - **expected**: Each SDK's observed sequence equals the fixture; no cross-SDK frame appears in either sequence.
    - **forbidden**: A C# session receives a TS-directed frame (or vice versa).
    - **verifies**: `verify-simulator-cross-sdk-parallel`, `verify-simulator-per-sdk-e2e`

### `contract-compatibility-ci-drift-gate` (failure_recovery)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-ci-outcome-partition` | T2 | FR-P1-05, FR-P1-06 | unit test of the scan tool covering all four decision rules with synthetic inputs. | `scripts/run-verification-profile.sh pr compatibility` |
| `verify-ci-undeclared-blocks-merge` | T2 | FR-P1-05 | integration test that runs the compatibility profile against a synthetic PR referencing an undeclared field. | `scripts/run-verification-profile.sh pr compatibility` |

**Witnesses**

- `witness-ci-declared-normal` [normal] risks: (normal)
    - **precondition**: PR only touches fields already declared in protocol/*.schema.json.
    - **stimulus**: compatibility profile runs.
    - **expected**: profile exits zero; PR is mergeable.
    - **forbidden**: profile fails despite only declared usage.
    - **verifies**: `verify-ci-outcome-partition`
- `witness-ci-undeclared-adversarial` [adversarial] risks: boundary, malformed, unknown, inconclusive
    - **precondition**: PR touches a dynamically constructed field name in a generated SDK.
    - **stimulus**: compatibility profile scans.
    - **expected**: Scan returns inconclusive; profile exits non-zero; artifact reports the SDK and the unclassifiable call site.
    - **forbidden**: profile passes despite inconclusive classification.; profile silently treats unknown usage as safe.
    - **verifies**: `verify-ci-undeclared-blocks-merge`, `verify-ci-outcome-partition`
- `witness-ci-conflicting-scan-adversarial` [adversarial] risks: conflicting_evidence, unknown, recovery, unsupported_environment
    - **precondition**: A PR touches an SDK in a way that two scan passes classify differently, or references a wire kind whose schema mapping is missing.
    - **stimulus**: compatibility profile runs both scan passes and cross-checks classifications.
    - **expected**: profile exits non-zero.; artifact names the divergent classification or the missing schema mapping.; A retry with an unchanged tree yields the same failure (deterministic).
    - **forbidden**: profile treats one scan pass as authoritative and passes.; profile treats missing-schema as declared.
    - **verifies**: `verify-ci-outcome-partition`, `verify-ci-undeclared-blocks-merge`

### `contract-auth-trust-boundary-approval-answering` (security_boundary)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-auth-decided-by` | T1 | FR-P0-03, FR-P0-12 | server/api scenario test mints two WS tickets (two ids ci-A, ci-B), submits CmdApprovalRespond from A, asserts EvtApprovalResolved carries resolving_client_instance_id=ci-A. | `cd src && go test ./server/api -run TestApprovalDecidedByClientInstance` |
| `verify-auth-instance-id-not-reused` | T0 | FR-P0-12 | ticket_test.go: mint id_1 from client A, close A's WS, reconnect A and mint id_2. | `cd src && go test ./server/api -run TestTicketClientInstanceIDNotReused` |
| `verify-auth-401-preserves-state` | T1 | FR-P0-10 | mux_scenario_test.go: submit CmdApprovalRespond without a bearer token. | `cd src && go test ./server/api -run TestApprovalRespond401` |

**Witnesses**

- `witness-auth-decided-by-populated-normal` [normal] risks: (normal)
    - **precondition**: Client A minted ticket -> (t_A, ci-A) and opened a WS with t_A.
    - **stimulus**: A submits CmdApprovalRespond(r1).
    - **expected**: EvtApprovalResolved(r1) carries resolving_client_instance_id=ci-A.
    - **forbidden**: resolving_client_instance_id is empty on the emitted event.
    - **verifies**: `verify-auth-decided-by`
- `witness-auth-unauthenticated-adversarial` [adversarial] risks: boundary, malformed, unsupported_environment
    - **precondition**: A pending r1 exists.
    - **stimulus**: A request with no bearer / expired bearer / consumed WS ticket submits CmdApprovalRespond(r1).
    - **expected**: Request rejected with 401.; r1 remains pending; no EvtApprovalResolved is broadcast.
    - **forbidden**: r1 is implicitly resolved by the rejected request.; A broadcast fires despite auth failure.
    - **verifies**: `verify-auth-401-preserves-state`
- `witness-auth-instance-id-not-reused-adversarial` [adversarial] risks: stale, boundary, repeated_usage
    - **precondition**: Client A minted (t_1, ci_1), closed the WS, and reconnected with a fresh ticket.
    - **stimulus**: A REST CmdApprovalRespond presents ci_1.
    - **expected**: Request is rejected as unknown-identity; ApprovalRequest is unchanged.; The new WS's ticket returned a different id ci_2 != ci_1.
    - **forbidden**: ci_1 is accepted after its WS closed.; ci_1 == ci_2 (id reuse across connections).
    - **verifies**: `verify-auth-instance-id-not-reused`
- `witness-auth-401-normal` [normal] risks: (normal)
    - **precondition**: A pending r1 exists.
    - **stimulus**: A well-formed CmdApprovalRespond with a valid bearer + fresh WS ticket is admitted.
    - **expected**: Request is admitted through TokenAuth/ticketStore; r1 is processed; no 401 is returned.
    - **forbidden**: A valid request is rejected with 401.
    - **verifies**: `verify-auth-401-preserves-state`

### `contract-adr0021-supersede-stdlib-preservation` (migration_compatibility)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-adr0021-lint-gate` | T2 | FR-P1-10 | make lint runs the existing stdlib-only depguard rule against every Go package including any generated Go helper. | `make lint` |

**Witnesses**

- `witness-adr0021-normal` [normal] risks: (normal)
    - **precondition**: The Phase 1 pipeline emits no Go artifact, or only a stdlib-importing one.
    - **stimulus**: make lint runs.
    - **expected**: make lint exits zero.
    - **forbidden**: make lint fails on a stdlib-only tree.
    - **verifies**: `verify-adr0021-lint-gate`
- `witness-adr0021-drift-adversarial` [adversarial] risks: boundary, unsupported_environment
    - **precondition**: A pipeline change accidentally emits a Go file importing a third-party JSON validator.
    - **stimulus**: make lint runs.
    - **expected**: make lint fails naming the offending file and import.
    - **forbidden**: The lint gate silently allows the third-party import.
    - **verifies**: `verify-adr0021-lint-gate`
- `witness-adr0021-hand-written-ts-migration-stale` [adversarial] risks: stale, recovery, boundary
    - **precondition**: The hand-written TS wire layer (clients/ui/src/wire) still ships alongside a generated TS SDK during the incremental migration.
    - **stimulus**: A schema change lands in protocol/*.schema.json; the hand-written TS layer is stale for one release cycle.
    - **expected**: make lint remains green (no Go-side third-party import is introduced).; The stale hand-written TS layer's fixture round-trip either succeeds (additive change) or fails deterministically (breaking change), never silently drifts.
    - **forbidden**: The Go stdlib-only gate silently allows a non-stdlib import because generation introduced it.; The hand-written TS layer decodes malformed data as if it were valid.
    - **verifies**: `verify-adr0021-lint-gate`

### `contract-deep-link-uri-shape` (integration_contract)

| Verification | Tier | Requirements | Method | Command |
|---|---|---|---|---|
| `verify-deep-link-parse-construct` | T0 | FR-P1-09 | SDK unit tests parse and construct the two shapes. | `scripts/run-verification-profile.sh pr compatibility` |

**Witnesses**

- `witness-deep-link-normal` [normal] risks: (normal)
    - **precondition**: SDK parse helper is available.
    - **stimulus**: Parse agent-grid://approval/<id>.
    - **expected**: Helper returns {kind: 'approval', id: <id>}.
    - **forbidden**: Helper returns a raw string component.
    - **verifies**: `verify-deep-link-parse-construct`
- `witness-deep-link-malformed-adversarial` [adversarial] risks: malformed, boundary
    - **precondition**: A shell receives a URI agent-grid://banana/xyz.
    - **stimulus**: Parse helper is invoked.
    - **expected**: Helper returns a typed error; the shell surfaces an unrecognized-link message.
    - **forbidden**: Helper returns a partial or best-effort parse.; Shell hand-parses the URI to recover.
    - **verifies**: `verify-deep-link-parse-construct`

## Resolved critique blockers

| Issue | Resolution |
|---|---|
| `issue-d2-answerer-identity-gap` | Added FR-P0-12 (per-WS-connection ephemeral client-instance-id issuance), decision-input-approval-answerer-identity, ADR adr-20260724-approval-answerer-identity-per-ws-instance (accepted), and contract-auth-trust-boundar… |
| `issue-d2-lifecycle-cancel-teardown-missing` | Extended contract-approval-question-lifecycle state machine to pending -> {resolved, expired, cancelled}, added FR-P0-11 (teardown transitions to cancelled + drain), added NFR-06 (no leak), added ADR adr-20260724-approva… |
| `issue-d2-simulator-grounding-contradicts-scaffold` | Replaced 'invented' text with an explicit ADR-tracked disposition: ADR adr-20260724-simulator-under-protocol-directory (accepted) names protocol/simulator/ as a governed extension of plan-20260723-repo-structure.md's pla… |
| `issue-d2-adr-0023-omission-for-approval-broadcast` | Added both ADRs to component-approval-question-proto-wire.accepted_adr_refs and component-server-api-approval-gateway.accepted_adr_refs; added FR-P1-12 pinning the coexistence rule; added ADR adr-20260724-approval-broadc… |
| `issue-d2-question-wire-classified-implementation-detail` | Reclassified to design_choice in decision_dispositions; folded the free-text-answer rule into contract-approval-question-envelope.decision_rules (decision-envelope-question-answer-shape) with a normal-case observable (ob… |
| `issue-d2-remote-control-deep-link-decision-input-drop` | Added decision-input-deep-link-uri-shape sourcing plans/remote-control-mobile-session-deep-link.md, ADR adr-20260724-deep-link-shape-adopts-remote-control-plan (accepted) adopting agent-grid://session/<id> and agent-grid… |
| `issue-d2-reconnect-scope-mislabelled-as-local` | Changed contract-reconnect-authoritative-resubscribe.observable_effects[observable-reconnect-snapshot-matches-state].scope to 'global'; added semantic_profile profile-reconnect-scope-consistency (kind=scope_consistency, … |
| `issue-d2-expiry-default-source-underspecified` | Added input-lifecycle-expiry-policy (owner=component-approval-question-state-domain, producer=component-approval-question-state-domain, source=driver per-session policy captured onto ApprovalRequest at creation as immuta… |

## Runtime commands

- Go tests: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...`
- Race-sensitive subtrees: `make test-race`
- Lint: `GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint`
- Web unit: `cd clients/ui && npm run test:unit`
- Compatibility CI profile: `scripts/run-verification-profile.sh pr compatibility`
- docs skill: `python3 <docs> lint`
