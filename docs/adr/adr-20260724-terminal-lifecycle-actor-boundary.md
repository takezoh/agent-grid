---
id: adr-20260724-terminal-lifecycle-actor-boundary
kind: adr
title: Make TerminalLifecycleActor the sole terminal lifecycle and RevisionOutcome
  authority
status: accepted
created: '2026-07-24'
decision_makers:
- unknown
tags:
- terminal
- lifecycle
- authority
- protocol
owners:
- host/state
- host/runtime
- server/api
- clients/ui
relations:
- {type: supersedes, target: adr-20260711-terminal-subscription-desired-reconcile}
- {type: references, target: adr-20260711-keep-single-ipc-connection-topology}
- {type: references, target: adr-20260715-geometry-bearing-terminal-attach}
- {type: originatedFrom, target: change-20260724-terminal-lifecycle-liveness}
source_paths:
- clients/ui/src/socket/terminalSubscription.ts
- src/server/api
- src/host/proto
- src/host/state
- src/host/runtime
summary: Preserve browser intent publication while making TerminalLifecycleActor the
  only lifecycle ledger and RevisionOutcome writer.
consequences:
  positive:
  - One authoritative lifecycle ledger and RevisionOutcome writer removes remote absence
    inference and dual terminal authority.
  negative:
  - Browser, gateway and daemon current lifecycle v2 types must change together.
  neutral:
  - Single IPC and geometry-bearing intent remain.
confirmation: Reducer, blocked-runtime, gateway, current-v2 codec, and production
  Connection tests enforce one intent publisher, one lifecycle/RevisionOutcome writer,
  accepted/waiting nonterminality, and no imperative reconciliation.
updated: '2026-07-24'
---

# Context

The proven gateway reader head-of-line failure is one symptom of a larger authority defect. Browser imperative history, gateway cleanup inference, and daemon effective state can disagree, while only the daemon can author what surface is applied. Treating a browser delivery timeout as the same kind of outcome would add another authority instead of removing the split.

# Decision

The browser remains the sole publisher of complete monotonic user intent. The gateway authenticates and transports that value but owns no effective subscription replica, targeted unsubscribe, or absence query.

Lifecycle commands are classified at daemon IPC decode before blocking runtime effects and enter a capacity-16 `TerminalLifecycleActor` mailbox. The actor is the only writer of owner lease, expiry, accepted desired, applied projection, effect epoch, and authoritative `RevisionOutcome`. Per-owner reconcilers execute immutable effect snapshots and return stamped results; they write no lifecycle state.

`accepted` and `waiting` are non-terminal. Only the actor can single-assign terminal applied, rejected, superseded, released, or typed degradation for a daemon-admitted revision. Publishing N+1 locally cancels observing N as `publication_replaced` before enqueue; the gateway may discard unadmitted pending N but authors no status. Browser replacement and delivery timeout belong to a separate transport namespace and cannot change an actor assignment.

The current lifecycle v2 wire carries public correlation, complete desired state, terminality, output ordering, and diagnostics. At explicit acceptance this ADR supersedes the imperative wire/retry/effective-state decisions of `adr-20260711-terminal-subscription-desired-reconcile`; its browser user-intent single-writer principle remains.

# Alternatives

**Gateway actor around imperative commands.** Rejected because it retains compensation, effective-state replication, and remote absence inference.

**Browser timeout as degraded RevisionOutcome.** Rejected because it creates a second writer and conflicts with a late actor outcome.

**Blocking runtime dispatcher as lifecycle actor.** Rejected because it preserves the demonstrated progress dependency.

# Consequences

{% consequence kind="positive" %}One daemon linearization point owns lifecycle truth and authoritative outcomes.{% /consequence %}

{% consequence kind="negative" %}Current browser, gateway, daemon protocol and state code change together.{% /consequence %}

{% consequence kind="neutral" %}One physical daemon IPC and geometry-bearing browser intent remain.{% /consequence %}


{% transition from="proposed" to="accepted" date="2026-07-24" %}
ユーザーが全設計を承認し、RevisionOutcome authority boundary を実装開始
{% /transition %}
