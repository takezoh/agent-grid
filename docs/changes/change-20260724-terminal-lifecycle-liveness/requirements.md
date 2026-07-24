---
change: change-20260724-terminal-lifecycle-liveness
role: requirements
functional_requirements:
- id: FR-TERM-1
  type: ubiquitous
  statement: The simultaneously implemented lifecycle v2 system shall recover terminal presentation from stalled input, resize, status delivery, or obsolete work without a browser page reload.
  priority: must
- id: FR-TERM-2
  type: ubiquitous
  statement: The daemon TerminalLifecycleActor shall be the sole writer of owner lease, lease expiry, accepted desired revision, applied projection, and authoritative RevisionOutcome; effect workers and the browser shall not author RevisionOutcome.
  priority: must
- id: FR-TERM-3
  type: event_driven
  statement: When a desired command arrives, the daemon shall atomically accept a greater revision, replay an equal identical value idempotently, reject an equal different value as conflict, and reject a lower revision as stale.
  priority: must
- id: FR-TERM-4
  type: unwanted
  statement: The system shall not allow completion, status, output, release, or severance from an obsolete connection generation, private owner, client revision, lease epoch, effect epoch, relay epoch, or output sequence to mutate or render the current terminal.
  priority: must
- id: FR-TERM-5
  type: event_driven
  statement: When a lifecycle connection is established, the browser shall publish only the public correlation tuple {clientInstanceID, connectionGeneration, clientRevision}; the gateway shall map that tuple to a server-only owner key derived from the consumed single-use ticket, fresh socket nonce, and daemon IPC generation, and shall never expose that owner key.
  priority: must
- id: FR-TERM-6
  type: state_driven
  statement: While a browser publication is observing delivery, accepted and waiting statuses shall remain non-terminal and shall not clear its watchdog; only a matching authoritative terminal RevisionOutcome shall transition it to observed_remote.
  priority: must
- id: FR-TERM-7
  type: event_driven
  statement: When a desired value names a session, it shall contain valid geometry as one complete revision; daemon reconciliation shall release the old SurfaceLease before acquiring the current surface at that geometry and shall not expose a separate imperative resize lifecycle command.
  priority: must
- id: FR-TERM-8
  type: event_driven
  statement: When exact release arrives or a lease expires, TerminalLifecycleActor shall remove the ledger entry and command release of its epoch-fenced SurfaceLease; SurfaceLease.Release shall be idempotent and physically remove the termvt subscriber map entry, channel, and fanout goroutine within 100 milliseconds without terminating the PTY session.
  priority: must
- id: FR-TERM-9
  type: unwanted
  statement: The gateway shall not implement imperative subscribe/unsubscribe reconciliation, speculative subscriber sets, targeted cleanup, or absence queries.
  priority: must
- id: FR-TERM-10
  type: ubiquitous
  statement: The system shall preserve one physical daemon IPC, per-owner isolation, typed severance, geometry semantics, and shared-hop priority lanes.
  priority: must
- id: FR-TERM-11A
  type: event_driven
  statement: When the daemon admits a revision, TerminalLifecycleActor shall assign at most one authoritative terminal RevisionOutcome under its absolute four-second apply deadline; accepted and waiting shall not be terminal outcomes.
  priority: must
- id: FR-TERM-11B
  type: event_driven
  statement: When the browser replaces an observing publication with a newer desired value, it shall first transition the old TransportObservation to local publication_replaced and cancel its watchdog before enqueuing the new publication; when no matching authoritative terminal RevisionOutcome is received by an observing publication's absolute four-second delivery deadline, the browser shall atomically record delivery_timeout, close the socket, increment connectionGeneration, acquire a fresh ticket and private owner mapping, allocate a fresh clientRevision, and replay the complete desired value; failed ticket or socket attempts shall remain visibly reconnecting and retry without page reload.
  priority: must
- id: FR-TERM-11C
  type: unwanted
  statement: Browser TransportObservation and reconnecting projection shall not overwrite, synthesize, or contradict any authoritative RevisionOutcome, and status or output from a replaced connection generation or revision shall not settle or render the fresh publication.
  priority: must
- id: FR-TERM-12A
  type: event_driven
  statement: When a producer stage watermark changes, that producer shall atomically replace its latest value and mark its capacity-one slot dirty; a 250-millisecond monotonic tick shall atomically snapshot and clear dirty and enqueue that latest snapshot, while any concurrent or later change remains dirty for the next tick and burst intermediates may be coalesced.
  priority: must
- id: FR-TERM-12B
  type: event_driven
  statement: Before the gateway forwards a terminal or close status for a relay epoch, it shall have forwarded every admitted output sequence through finalSequence or have attributed each missing sequence as delivery_gap; the immediate status marker shall include final watermark and diagnostic drop counters, and an unsatisfied barrier or failed status lane shall close the socket as transport unknown.
  priority: must
- id: FR-TERM-12C
  type: state_driven
  statement: While diagnosing a lifecycle v2 revision, accepted, applied, output, forwarded, received, and rendered watermarks shall be authored by their actual daemon, gateway, and browser stages; no_output shall be derived only from an authoritative applied watermark with outputSeq=0 and shall be mutually exclusive with diagnostic unknown or drop.
  priority: must
non_functional_requirements:
- id: NFR-TERM-1
  type: reliability
  criteria: Browser delivery observation uses one absolute four-second monotonic deadline per publication; daemon application uses a separate absolute four-second monotonic deadline per admission; neither deadline resets on accepted, waiting, or retry.
  measurement: Fake-clock boundary tests, including applied at 3.9 seconds with delayed delivery and equality precedence.
- id: NFR-TERM-2
  type: performance
  criteria: Each stage producer stores latest one and emits at most 4 Hz; the first dirty value is enqueued within 250 milliseconds and bursts coalesce without losing the final latest value.
  measurement: Low-rate single-event and sustained-burst deterministic clock tests.
- id: NFR-TERM-3
  type: scalability
  criteria: Diagnostic state is bounded to active owner 8, tombstone 16, 60-second retention, and one dirty/latest slot plus one diagnostic latest-one lane per producer.
  measurement: Capacity and eviction tests with bounded local replace/drop counters.
- id: NFR-TERM-4
  type: security
  criteria: Diagnostics record no bearer, ticket, nonce, private owner key, or terminal bytes; public correlation uses clientInstanceID, connectionGeneration, and clientRevision only.
  measurement: Codec/schema assertions and log capture tests.
acceptance:
- id: AC-1
  given: Input, resize, Runtime.dispatch, or one surface effect is stalled
  when: A later complete desired revision is published
  then: Lifecycle admission progresses and browser observes an authoritative terminal outcome or enters typed reconnecting transport recovery without page reload.
  requirement_refs:
  - FR-TERM-1
  - FR-TERM-11B
- id: AC-2
  given: A new connection generation, private owner and client revision are current
  when: Old completion, status, output, release, severance or relay event arrives
  then: Current state and rendered bytes do not change.
  requirement_refs:
  - FR-TERM-4
  - FR-TERM-11C
- id: AC-3
  given: Actor commits applied at 3.9 seconds but delivery is delayed beyond the browser deadline
  when: The delivery watchdog fires
  then: Applied remains the only old RevisionOutcome; browser records delivery_timeout, closes, and replays under fresh generation/ticket/owner/revision.
  requirement_refs:
  - FR-TERM-2
  - FR-TERM-6
  - FR-TERM-11A
  - FR-TERM-11B
  - FR-TERM-11C
- id: AC-4
  given: Browser receives accepted or waiting for its current publication
  when: The delivery deadline has not yet produced a terminal outcome
  then: The watchdog remains active and TransportObservation stays observing.
  requirement_refs:
  - FR-TERM-6
  - FR-TERM-11B
- id: AC-5
  given: Timer and socket close race at the delivery deadline
  when: The browser reducer processes them
  then: Exactly one of delivery_timeout or socket_closed wins; a timeout-triggered close cannot overwrite delivery_timeout.
  requirement_refs:
  - FR-TERM-11B
  - FR-TERM-11C
- id: AC-6
  given: Fresh ticket acquisition or socket open fails during recovery
  when: The attempt terminates
  then: UI stays reconnecting, desired state is preserved, and another bounded-resource attempt is scheduled without reload.
  requirement_refs:
  - FR-TERM-1
  - FR-TERM-11B
- id: AC-7
  given: One stage watermark changes once and no further output occurs
  when: The next 250ms monotonic tick runs
  then: That producer enqueues the latest snapshot; no sequence threshold delays it.
  requirement_refs:
  - FR-TERM-12A
- id: AC-8
  given: Stage watermarks change continuously in a burst including during flush
  when: Multiple 250ms ticks run
  then: Emission stays at most 4Hz, intermediate values may coalesce, and the final changed latest value is eventually emitted.
  requirement_refs:
  - FR-TERM-12A
- id: AC-9
  given: The diagnostic lane drops or replaces a watermark
  when: A terminal or close marker is emitted
  then: The immediate status marker includes the final watermark and local drop counter; if status delivery also fails, the socket closes as transport unknown.
  requirement_refs:
  - FR-TERM-12B
  - FR-TERM-12C
- id: AC-10
  given: A terminal RevisionOutcome has finalSequence N and output <=N is admitted
  when: Gateway prepares the terminal marker
  then: It forwards all <=N output or attributes every missing sequence as delivery_gap before the marker; otherwise it closes on barrier timeout.
  requirement_refs:
  - FR-TERM-12B
- id: AC-11
  given: Actor applied reports outputSeq=0 with no diagnostic loss
  when: Operators inspect the revision
  then: It is classified no_output; any drop, missing evidence or positive output makes no_output impossible and yields unknown/conflicting as appropriate.
  requirement_refs:
  - FR-TERM-12C
- id: AC-12
  given: Current lifecycle v2 browser and server are built from the same change
  when: Desired, status, output and diagnostic frames round-trip
  then: All required public correlation, terminality and finalSequence fields are preserved and private owner material is absent.
  requirement_refs:
  - FR-TERM-3
  - FR-TERM-5
  - FR-TERM-6
  - FR-TERM-12B
- id: AC-13
  given: The new ADRs are proposed and the accepted desired-reconcile ADR remains effective
  when: Readiness is evaluated without explicit approval
  then: The change remains draft; acceptance must atomically accept new ADRs, supersede the old ADR and pass docs conformance.
  requirement_refs:
  - FR-TERM-2
  - FR-TERM-9
- id: AC-14
  given: A later blank incident has only bounded stage evidence
  when: Diagnosis is reported
  then: The report may attribute a first missing stage only when evidence is complete; otherwise it states unknown and does not generalize the proven gateway head-of-line mechanism.
  requirement_refs:
  - FR-TERM-12C
- id: AC-15
  given: Publication N is observing and occupies the gateway latest-one pending slot before daemon admission
  when: Browser intent changes and publishes N+1
  then: Browser first transitions N to local publication_replaced and cancels N's watchdog; gateway may discard N without terminal status or RevisionOutcome; N cannot later close the socket during N+1.
  requirement_refs:
  - FR-TERM-1
  - FR-TERM-6
  - FR-TERM-11B
  - FR-TERM-11C
---

# Terminal lifecycle liveness requirements

## Invariants

- `TerminalLifecycleActor` is the only authority that writes lease, expiry, accepted desired state, applied projection, and `RevisionOutcome`.
- `accepted` and `waiting` are non-terminal. They do not clear the browser delivery watchdog.
- Browser state `TransportObservation` belongs to a connection attempt and is disjoint from daemon `RevisionOutcome`.
- Browser-visible correlation is exactly `{clientInstanceID, connectionGeneration, clientRevision}`. Private owner keys and server epochs never cross that boundary.
- Reconnect replaces connection generation, ticket, private owner mapping, and client revision. Status and output from the replaced namespace cannot settle or render the replacement.
- Each telemetry producer owns one single-writer `{latest, dirtyGeneration, flushedGeneration, nextFlush}` slot.
- A terminal or close status cannot overtake admitted output through `finalSequence`.
- `no_output` is authoritative only when actor-applied evidence has `outputSeq=0`; missing or dropped diagnostics are `unknown`, never `no_output`.
- The gateway owns no speculative subscriber set and performs no targeted unsubscribe or absence query.

## State partitions

### Authoritative RevisionOutcome

The daemon actor alone assigns a terminal outcome for a daemon-admitted revision. `applied`, `rejected`, `superseded`, `released`, and typed daemon degradation are single-assignment. Browser delivery failure cannot create or replace one of these outcomes.

### Local TransportObservation

For one browser connection attempt:

```text
observing -> observed_remote
          -> publication_replaced
          -> delivery_timeout
          -> socket_closed
```

Only a matching authoritative terminal `RevisionOutcome` can select `observed_remote`. Publishing N+1 first selects local `publication_replaced` for observing N and cancels N's watchdog before enqueue; it keeps the socket open and creates no `RevisionOutcome`. A single reducer performs every transition with compare-and-set semantics. At deadline equality the deadline event wins. `delivery_timeout` is stored before it initiates socket close, so the resulting close event cannot overwrite it.

### Recovery projection

`reconnecting` is a UI projection over connection replacement, ticket acquisition, socket open, and desired replay. Failure to obtain a ticket or open a socket preserves desired state, retains `reconnecting`, and schedules another attempt at 250, 500, 1000, then at most 2000 milliseconds. This is not a remote lifecycle outcome.

## Fixed bounds

- Browser delivery deadline: absolute monotonic `publishedAt + 4s`.
- Daemon apply deadline: independently absolute monotonic `acceptedAt + 4s`.
- Owner renewal/expiry: 4 seconds / 12 seconds.
- SurfaceLease physical release: 100 milliseconds.
- Actor mailbox: FIFO 16; browser/gateway desired: latest one; input: FIFO 64.
- Owners per daemon IPC generation: 8.
- Telemetry: first dirty enqueue within 250 milliseconds, at most 4 Hz per producer, latest one per producer/lane, active 8, tombstone 16, retention 60 seconds.

## Acceptance scenarios

The canonical Given/When/Then scenarios are in frontmatter. The following counterexamples must fail:

- `accepted` clears the browser watchdog.
- applied at 3.9 seconds plus delayed delivery causes the browser to author a degraded `RevisionOutcome`.
- timeout-triggered close changes `delivery_timeout` into `socket_closed`.
- gateway pending replacement authors `superseded`, or old N's watchdog closes the socket after N+1 publication.
- a retry reuses the replaced connection generation, owner mapping, or client revision.
- one low-rate watermark waits for an output-count threshold.
- clearing dirty during a concurrent update loses the final latest value.
- a diagnostic-lane failure is reported only through that same failed lane.
- terminal status passes output that has neither been forwarded nor attributed as `delivery_gap`.
- missing telemetry is classified `no_output`.

## Scope boundary

This change specifies the simultaneously implemented current lifecycle v2 endpoints. Endpoint-version compatibility, negotiation, staggered deployment, and rollback behavior are not product contracts or readiness criteria for this change.

## Causal boundary

Debug evidence proves gateway reader head-of-line blocking. It does not prove that every terminal blank, or the previously hypothesized browser promise edge, has that cause. Stage evidence may support a later attribution only when all required producer watermarks are present; otherwise the result is explicitly `unknown`.
