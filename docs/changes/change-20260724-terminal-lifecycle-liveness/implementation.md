---
change: change-20260724-terminal-lifecycle-liveness
role: implementation
contracts:
- contract-owner-revision
- contract-control-admission
- contract-browser-publication
- contract-capability-lease
- contract-gateway-lanes
- contract-owner-reconcile
- contract-lease-teardown
- contract-resource-bounds
- contract-authoritative-revision-outcome
- contract-browser-transport-recovery
- contract-cross-namespace-fencing
- contract-current-v2-wire
- contract-stage-observability
contract_projections:
- id: contract-owner-revision
  decision_rules:
  - decision-owner-revision
  observable_effects:
  - observable-owner-record
  operational_inputs:
  - input-desired-command
  semantic_profiles: []
  failures:
  - failure-owner-conflict
  verifications:
  - verify-owner-reducer
  witnesses:
  - witness-owner-normal
  - witness-owner-stale
- id: contract-control-admission
  decision_rules:
  - decision-control-admit
  observable_effects:
  - observable-control-progress
  operational_inputs:
  - input-control-frame
  semantic_profiles:
  - profile-control-cost
  failures:
  - failure-control-capacity
  verifications:
  - verify-control-blocking
  witnesses:
  - witness-control-normal
  - witness-control-blocked
- id: contract-browser-publication
  decision_rules:
  - decision-browser-publish
  observable_effects:
  - observable-browser-publish
  operational_inputs:
  - input-browser-intent
  semantic_profiles: []
  failures:
  - failure-browser-geometry
  verifications:
  - verify-browser-wired
  witnesses:
  - witness-browser-normal
  - witness-browser-race
- id: contract-capability-lease
  decision_rules:
  - decision-capability-bind
  observable_effects:
  - observable-capability-boundary
  operational_inputs:
  - input-authenticated-upgrade
  semantic_profiles: []
  failures:
  - failure-capability-invalid
  verifications:
  - verify-capability
  witnesses:
  - witness-cap-normal
  - witness-cap-theft
- id: contract-gateway-lanes
  decision_rules:
  - decision-lane-route
  observable_effects:
  - observable-lane-progress
  operational_inputs:
  - input-lifecycle-frame
  semantic_profiles: []
  failures:
  - failure-input-overflow
  verifications:
  - verify-lanes
  witnesses:
  - witness-lane-normal
  - witness-lane-stall
- id: contract-owner-reconcile
  decision_rules:
  - decision-owner-apply
  observable_effects:
  - observable-owner-apply
  operational_inputs:
  - input-latest-desired
  semantic_profiles: []
  failures:
  - failure-reconcile-backend
  verifications:
  - verify-reconcile
  witnesses:
  - witness-reconcile-normal
  - witness-reconcile-stale
- id: contract-lease-teardown
  decision_rules:
  - decision-lease-expire
  observable_effects:
  - observable-lease-clean
  operational_inputs:
  - input-lease-clock
  semantic_profiles:
  - profile-lease-cost
  failures:
  - failure-lease-renewal
  verifications:
  - verify-lease
  witnesses:
  - witness-lease-normal
  - witness-lease-missing-release
- id: contract-resource-bounds
  decision_rules:
  - decision-fixed-bounds
  observable_effects:
  - observable-fixed-bounds
  operational_inputs: []
  semantic_profiles:
  - profile-resource-cost
  failures:
  - failure-resource-overflow
  verifications:
  - verify-resource-bounds
  witnesses:
  - witness-bounds-normal
  - witness-bounds-burst
- id: contract-authoritative-revision-outcome
  decision_rules:
  - decision-authoritative-applied
  - decision-authoritative-nonapplied
  observable_effects:
  - observable-authoritative-outcome
  operational_inputs:
  - input-actor-stamped-event
  semantic_profiles: []
  failures:
  - failure-authoritative-deadline
  verifications:
  - verify-authoritative-outcome
  witnesses:
  - witness-authoritative-normal
  - witness-authoritative-delayed-delivery
- id: contract-browser-transport-recovery
  decision_rules:
  - decision-transport-observed
  - decision-transport-timeout
  - decision-transport-closed
  - decision-transport-publication-replaced
  observable_effects:
  - observable-transport-recovery
  operational_inputs:
  - input-browser-publication-attempt
  semantic_profiles:
  - profile-transport-observation
  failures:
  - failure-transport-delivery
  - failure-transport-reconnect
  verifications:
  - verify-transport-recovery
  witnesses:
  - witness-transport-normal
  - witness-transport-delay
  - witness-transport-replaced
- id: contract-cross-namespace-fencing
  decision_rules:
  - decision-public-private-map
  - decision-fence-old-event
  observable_effects:
  - observable-current-correlation
  operational_inputs:
  - input-public-correlation
  - input-private-owner-map
  semantic_profiles: []
  failures:
  - failure-correlation-invalid
  verifications:
  - verify-cross-namespace-fencing
  witnesses:
  - witness-fencing-normal
  - witness-fencing-old
- id: contract-current-v2-wire
  decision_rules:
  - decision-current-v2-codec
  observable_effects:
  - observable-current-v2-roundtrip
  operational_inputs:
  - input-current-v2-browser-command
  - input-current-v2-daemon-event
  - input-current-v2-gateway-forward
  semantic_profiles: []
  failures:
  - failure-current-v2-malformed
  verifications:
  - verify-current-v2-wire
  witnesses:
  - witness-current-v2-normal
  - witness-current-v2-malformed
- id: contract-stage-observability
  decision_rules:
  - decision-stage-determinate
  - decision-stage-unknown
  - decision-stage-inconclusive
  - decision-stage-conflicting
  observable_effects:
  - observable-stage-evidence
  operational_inputs:
  - input-daemon-stage
  - input-gateway-stage
  - input-browser-stage
  semantic_profiles:
  - profile-stage-cost
  - profile-stage-scope
  - profile-stage-outcomes
  failures:
  - failure-diagnostic-lane
  - failure-status-barrier
  - failure-stage-contract
  verifications:
  - verify-stage-observability
  witnesses:
  - witness-stage-low-rate
  - witness-stage-loss-barrier
adrs:
- adr-20260724-terminal-lifecycle-actor-boundary
- adr-20260724-terminal-lifecycle-bounds
decision_dispositions:
- decision_input_ref: decision-input-user-structural-correctness
  disposition: adopted
  rationale: Atomic daemon authority replaces the minimal gateway patch.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-owner-revision
- decision_input_ref: decision-input-user-reloadless-terminal
  disposition: adopted
  rationale: Fresh transport recovery and actor-only authoritative outcomes preserve reloadless recovery without dual authority.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-authoritative-revision-outcome
  - contract-browser-transport-recovery
- decision_input_ref: decision-input-existing-desired-owner
  disposition: adopted
  rationale: Browser remains sole user-intent publisher; the proposed authority ADR supersedes its incompatible imperative/effective-state ownership at acceptance.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-browser-publication
  - contract-authoritative-revision-outcome
- decision_input_ref: decision-input-single-ipc
  disposition: adopted
  rationale: Isolation is logical within one physical IPC.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-owner-reconcile
- decision_input_ref: decision-input-existing-severance
  disposition: adopted
  rationale: Typed severance is public-correlation fenced and projects as transport recovery without authoring RevisionOutcome.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-browser-transport-recovery
  - contract-cross-namespace-fencing
- decision_input_ref: decision-input-existing-sever-isolation
  disposition: adopted
  rationale: Failures remain owner-scoped.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-owner-reconcile
- decision_input_ref: decision-input-existing-priority-lanes
  disposition: adopted
  rationale: Existing shared-hop priority is preserved; lifecycle control is cut at IPC decode into mailbox16 while input remains FIFO64 and geometry is complete desired state.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-gateway-lanes
- decision_input_ref: decision-input-geometry-attach
  disposition: adopted
  rationale: Geometry is mandatory in complete desired state.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-browser-publication
- decision_input_ref: decision-input-repository-testing
  disposition: adopted
  rationale: The external SurfaceLease triple and wired recovery/telemetry tests remain mandatory.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-stage-observability
- decision_input_ref: decision-input-atomic-desired-candidate
  disposition: adopted
  rationale: Chosen as the structural authority model, not asserted as the only RCA repair.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-owner-revision
- decision_input_ref: decision-input-user-structural-contract
  disposition: adopted
  rationale: The plan changes ownership and protocol rather than applying deadline-only recovery.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-control-admission
- decision_input_ref: decision-input-repository-test-policy
  disposition: adopted
  rationale: The external SurfaceLease triple and wired recovery/telemetry tests remain mandatory.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-stage-observability
- decision_input_ref: decision-input-adr-desired-reconcile
  disposition: adopted
  rationale: At explicit acceptance, the authority ADR supersedes only incompatible imperative wire/retry/effective-state ownership while retaining browser intent publication.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-browser-publication
  - contract-authoritative-revision-outcome
- decision_input_ref: decision-input-adr-single-ipc
  disposition: adopted
  rationale: Single IPC is retained.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-owner-reconcile
- decision_input_ref: decision-input-adr-severance
  disposition: adopted
  rationale: Server severance remains typed and is fenced in the public correlation namespace.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-browser-transport-recovery
  - contract-cross-namespace-fencing
- decision_input_ref: decision-input-adr-per-subscriber-isolation
  disposition: adopted
  rationale: Owner/revision is the new isolation attribution.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-owner-reconcile
- decision_input_ref: decision-input-adr-priority-lanes
  disposition: adopted
  rationale: Existing shared-hop priority is preserved; lifecycle control is cut at IPC decode into mailbox16 while input remains FIFO64 and geometry is complete desired state.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-gateway-lanes
- decision_input_ref: decision-input-adr-geometry
  disposition: adopted
  rationale: AttachAtGeometry and latest resize semantics are preserved.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-browser-publication
- decision_input_ref: decision-input-new-new-only
  disposition: adopted
  rationale: The implementation defines one current lifecycle v2 contract for the simultaneously changed browser and server; compatibility behavior is outside this change and creates no contract or acceptance obligation.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  contract_refs:
  - contract-current-v2-wire
- decision_input_ref: decision-input-single-outcome-authority
  disposition: adopted
  rationale: TerminalLifecycleActor exclusively writes RevisionOutcome; browser writes only TransportObservation.
  adr_refs:
  - adr-20260724-terminal-lifecycle-actor-boundary
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-authoritative-revision-outcome
  - contract-browser-transport-recovery
- decision_input_ref: decision-input-fresh-recovery-identities
  disposition: adopted
  rationale: Connection replacement after delivery_timeout or socket_closed uses a fresh connection generation, ticket, private owner mapping and client revision, with old status/output fenced; local publication_replaced keeps the socket, ticket, owner and connection generation and allocates only the next client revision.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-browser-transport-recovery
  - contract-cross-namespace-fencing
- decision_input_ref: decision-input-watermark-liveness
  disposition: adopted
  rationale: Single-writer dirty/latest slots flush every 250ms without a sequence threshold and enforce finalSequence ordering.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-stage-observability
- decision_input_ref: decision-input-priority-lanes
  disposition: adopted
  rationale: Immediate status/final evidence remains isolated from latest-one diagnostics; status failure closes the transport.
  adr_refs:
  - adr-20260724-terminal-lifecycle-bounds
  contract_refs:
  - contract-stage-observability
- decision_input_ref: decision-input-test-triple
  disposition: subsumed
  rationale: This duplicates the earlier repository testing policy and is closed by that input's verification contract.
  adr_refs: []
  contract_refs: []
  subsumed_by: decision-input-repository-testing
milestones:
- id: chunk-1-authority
- id: chunk-2-runtime
- id: chunk-3-gateway-browser
- id: chunk-4-observability-verification
reference_algorithms:
- id: alg-owner-reconcile
- id: alg-stage-flush-barrier
contract_lineage:
- prior_contract_ref: contract-owner-revision
  disposition: retained
  successor_contract_refs:
  - contract-owner-revision
  verification_refs:
  - verify-owner-reducer
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-control-admission
  disposition: retained
  successor_contract_refs:
  - contract-control-admission
  verification_refs:
  - verify-control-blocking
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-browser-publication
  disposition: retained
  successor_contract_refs:
  - contract-browser-publication
  verification_refs:
  - verify-browser-wired
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-capability-lease
  disposition: retained
  successor_contract_refs:
  - contract-capability-lease
  verification_refs:
  - verify-capability
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-gateway-lanes
  disposition: retained_and_strengthened
  successor_contract_refs:
  - contract-gateway-lanes
  verification_refs:
  - verify-lanes
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-owner-reconcile
  disposition: retained
  successor_contract_refs:
  - contract-owner-reconcile
  verification_refs:
  - verify-reconcile
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-lease-teardown
  disposition: retained
  successor_contract_refs:
  - contract-lease-teardown
  verification_refs:
  - verify-lease
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-resource-bounds
  disposition: retained
  successor_contract_refs:
  - contract-resource-bounds
  verification_refs:
  - verify-resource-bounds
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
- prior_contract_ref: contract-convergence-outcomes
  disposition: replaced
  successor_contract_refs:
  - contract-authoritative-revision-outcome
  - contract-browser-transport-recovery
  - contract-cross-namespace-fencing
  verification_refs:
  - verify-authoritative-outcome
  - verify-transport-recovery
  - verify-cross-namespace-fencing
  rationale: The prior contract conflated daemon outcome and browser delivery timeout. Its authority, delivery-recovery and stale-fencing obligations are now separated.
- prior_contract_ref: contract-protocol-evolution
  disposition: retired
  successor_contract_refs:
  - contract-current-v2-wire
  verification_refs:
  - verify-current-v2-wire
  rationale: Compatibility evolution is outside the selected scope; only the simultaneously implemented current wire schema/producer contract remains.
- prior_contract_ref: contract-stage-observability
  disposition: retained_and_strengthened
  successor_contract_refs:
  - contract-stage-observability
  verification_refs:
  - verify-stage-observability
  rationale: Stable contract identity remains projected; current requirements and witnesses strengthen it without changing its owner.
---

# Terminal lifecycle liveness implementation

## Delivered foundation

The first implementation slice is now wired across the current browser/server
surface:

- `src/server/api/gateway_lifecycle_inbound.go` separates blocking input/resize
  work into a bounded data lane while lifecycle desired/subscribe/unsubscribe
  control requests retain prompt acknowledgements and serialized WebSocket
  responses.
- `src/host/proto/lifecycle.go` and `src/server/api/wire.go` define current-v2
  public correlation, daemon-authored outcomes, output metadata, diagnostics,
  and complete desired publication frames.
- `src/host/runtime/terminal_lifecycle_actor.go` is the reducer-side sole
  writer for admitted revision outcomes; accepted/waiting are non-terminal.
- `clients/ui/src/socket/transportObservation.ts` owns the browser transport
  namespace. A matching terminal outcome is distinct from local
  `publication_replaced`; delivery timeout requests connection recovery.
- `src/host/runtime/telemetry_slot.go` provides a capacity-one latest/dirty
  slot with a 250ms flush bound, and `terminal_barrier.go` prevents terminal
  markers from overtaking unforwarded output.
- `src/host/runtime/terminal_lifecycle_runtime.go` admits v2 commands on the
  runtime control lane, emits accepted/waiting/terminal outcomes, fences late
  effect results, and binds the current revision to a `TerminalRelay` lease.
- `src/server/api/gateway.go` forwards v2 desired commands over the existing
  daemon IPC and subscribes lifecycle outcome events without creating a second
  IPC or an imperative gateway reconciler.
- `clients/ui/src/socket/connection.ts` keeps the delivery watchdog active
  after the gateway acknowledgement and settles it only on a matching `lo`
  authoritative terminal frame.

All implementation chunks in this package are now wired and verified. The
supported PTY surface has the required fake, invariant contract, and
FakeVsReal release backstop; the browser smoke suite and repository gates are
green.

## Ownership and boundaries

1. Browser publishes complete desired intent and owns only per-attempt `TransportObservation`.
2. Gateway authenticates, maps the public correlation tuple to a private owner, transports frames, enforces relay ordering, and owns no effective subscription replica.
3. `TerminalLifecycleActor` linearizes lifecycle state and is the sole `RevisionOutcome` author.
4. Per-owner workers perform cancellable release/acquire effects and return fully stamped results; they write no lifecycle state.
5. `SurfaceLease` owns physical terminal subscriber cleanup without ending the PTY session.

## Current lifecycle v2 wire

The current browser, gateway, and daemon codecs are changed together. Browser produces desired commands; daemon produces accepted/waiting, authoritative outcomes, output and daemon diagnostics; gateway produces forwarding, delivery-gap, final-marker and local-counter evidence without rewriting embedded daemon authorship. Frames carry:

- public `clientInstanceID`, `connectionGeneration`, and `clientRevision`;
- complete desired value (`none` or session plus geometry);
- non-terminal `accepted`/`waiting` or authoritative terminal `RevisionOutcome`;
- relay epoch, output sequence, `finalSequence`, `delivery_gap`, final watermark, and local diagnostic drop counters.

The gateway stores the public-to-private owner mapping internally. Private owner keys, lease/effect epochs, tickets, nonces, and daemon IPC authority are never browser fields. Missing required fields, terminal `accepted`, or malformed correlation fail closed before lifecycle mutation.

## Browser recovery reducer

One production `Connection` reducer owns status, deadline, and close events. It compares public correlation before state mutation.

- Before enqueuing N+1 while N observes: `N: observing -> publication_replaced`, cancel N's watchdog, keep the socket open, then enqueue N+1. This is local cleanup, not a `RevisionOutcome`.
- Matching terminal outcome before the deadline: `observing -> observed_remote`.
- Deadline while observing: `observing -> delivery_timeout`, then close.
- External close while observing: `observing -> socket_closed`.
- Accepted or waiting: remain `observing`.
- Any later event for the same attempt: no transition.

Gateway latest-one replacement only discards an unadmitted transport frame; it emits no terminal status. If N was admitted, only the daemon actor may supersede it.

After timeout/close, increment `connectionGeneration`, acquire a fresh single-use ticket and private owner mapping, allocate a fresh `clientRevision`, and replay current complete desired. Ticket/socket failure retains reconnecting and retries with capped intervals. Old status/output is rejected before promise settlement, store projection, or rendering.

## Producer telemetry and relay barrier

Daemon produces accepted/applied/output/finalSequence/no-output evidence. Gateway produces forwarded/delivery-gap and its local replace/drop counters. Browser produces received/rendered evidence.

Each producer has one single-writer dirty slot. A stage change replaces `latest` and increments `dirtyGeneration`. Every 250 milliseconds, if dirty, the producer atomically snapshots latest and generation, advances `flushedGeneration` only to that snapshot, and non-blockingly replaces the diagnostic latest-one lane. A concurrent or later change has a greater generation and therefore stays dirty for the next tick.

For each owner/revision relay epoch, a terminal/close status waits until every admitted output sequence through `finalSequence` is forwarded or explicitly attributed as `delivery_gap`. The immediate status marker includes final watermark and local drop counters. Diagnostic failure therefore remains visible through a different lane. Barrier timeout or status-lane failure closes the socket, leaving the browser to report transport uncertainty.

## Resource and teardown contracts

- Lifecycle commands are decoded into the capacity-16 actor mailbox before blocking `Runtime.dispatch/eventCh`.
- Input remains ordered FIFO 64; desired state remains latest-one.
- Each owner has one worker and one effective `SurfaceLease`.
- `AcquireSurface(ctx)` must honor cancellation.
- Idempotent `SurfaceLease.Release` physically removes subscriber map entry, channel, and fanout within 100 milliseconds.
- One physical daemon IPC is retained; owner-local failure does not recycle healthy shared IPC.

## Delivery order

1. Define the current lifecycle v2 wire and pure actor `RevisionOutcome` reducer.
2. Add daemon decoder cut point, actor mailbox, immutable public/private correlation and stale fences.
3. Implement context-aware `SurfaceLease`, effect-only reconciler, and repository-required fake/contract/FakeVsReal triple.
4. Implement gateway lanes, owner lease, relay epoch, `finalSequence` barrier, and status evidence.
5. Replace browser imperative reconciliation with the production `Connection` observation/recovery reducer.
6. Add producer dirty slots and cross-layer verification. (completed)

## Governance

Both new ADRs are accepted, `adr-20260711-terminal-subscription-desired-reconcile` is
superseded, and `docs lint --conformance` is green under the recorded user
approval.

## Prior contract lineage

- The nine stable IDs `contract-owner-revision`, `contract-control-admission`, `contract-browser-publication`, `contract-capability-lease`, `contract-gateway-lanes`, `contract-owner-reconcile`, `contract-lease-teardown`, `contract-resource-bounds`, and `contract-stage-observability` are retained; gateway lanes and stage observability are strengthened.
- `contract-convergence-outcomes` is replaced by `contract-authoritative-revision-outcome`, `contract-browser-transport-recovery`, and `contract-cross-namespace-fencing`.
- `contract-protocol-evolution` is retired; `contract-current-v2-wire` owns only the scoped current message schema and producer attribution.
