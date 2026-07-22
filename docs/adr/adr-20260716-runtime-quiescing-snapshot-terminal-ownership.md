---
id: adr-20260716-runtime-quiescing-snapshot-terminal-ownership
kind: adr
title: Runtime owns quiescing, the all-session Save barrier, and terminal transition
status: proposed
created: '2026-07-16'
updated: '2026-07-16'
summary: Shutdown is a reducer-owned result-driven transaction; Save and deadline-aware cleanup outcomes gate acknowledgement and runtime termination.
decision_makers:
- agent-grid-maintainers
consulted:
- runtime-maintainers
- Web-client-maintainers
informed:
- agent-grid-users
tags:
- runtime
- shutdown
- persistence
- codex
relations:
- {type: partOf, target: change-20260716-codex-runtime-restart-continuity}
- {type: references, target: design-client}
- {type: references, target: adr-20260716-codex-observer-subscription-ready-ownership}
source_paths:
- src/client/state/state.go
- src/client/state/event.go
- src/client/state/effect.go
- src/client/state/reduce.go
- src/client/state/reduce_lifecycle.go
- src/client/runtime/runtime.go
- src/client/runtime/interpret.go
- src/client/runtime/persist.go
- src/cmd/server/coordinator.go
consequences:
  positive:
  - A successful all-session Save barrier proves every pre-teardown session upsert reached its per-file atomic rename before teardown; late teardown events cannot erase or stop sessions.
  - Signal and IPC ingress converge on one Runtime-owned terminal action and cannot leave a quiescing daemon alive without a coordinator cancel.
  - Pure admission and lifecycle reducers make every event class and persistence failure testable without process I/O.
  negative:
  - Shutdown gains Save and cleanup result events, transaction deadlines, joined waiters, and an explicit rollback path.
  - Save barrier completion must re-enter the event loop, so stale transaction results and terminal ordering require tests.
  - A partial Save can leave valid new prefix versions mixed with valid last-successful suffix versions; graph-wide point-in-time atomicity is not provided.
  - Quiescing rejects mutations that previously could race with shutdown, which may expose unavailable errors to callers.
  neutral:
  - LifecyclePhase and transaction metadata are process-local and do not change sessions store or Web wire formats.
  - Existing normal persistence effects remain usable outside shutdown; only shutdown requires acknowledged commit.
  - Coordinator cancellation remains a deadline fallback rather than the successful terminal owner.
confirmation: >-
  T0 exhaustive admission/projection tests and T1 runtimetest Harness cases for all-session success, multi-session failure-after-prefix, duplicate requests, typed IPC/signal outcomes, permanently blocked cleanup, bounded Runtime.Done, and Runtime-owned termination must pass.
---

# Runtime owns quiescing, the all-session Save barrier, and terminal transition

## Context

{% context %}
The current shutdown reducer emits persist, synchronous response, and resource release in one effect list. Persistence failure is only logged, while release stops Codex's app-server and can enqueue subsystem or frame-exit events before the event loop ends. Signal ingress later cancels the runtime, but IPC shutdown has no equivalent terminal owner.

Persistence is an upsert-only session-per-file store. `Save` writes each `<dataDir>/sessions/<id>.json` through a temp file and atomic rename, but the collection has no manifest transaction: failure after a prefix leaves valid new prefix versions and valid last-successful suffix versions. The shutdown contract must acknowledge this mixed-store behavior rather than promise graph-wide atomicity.

Durable state has one writer: the reducer. A shutdown freeze therefore needs to be a reducer state and an exhaustive event-admission decision, not a collection of backend checks. At the same time, that process lifecycle must never be restored from disk or shown as session state.
{% /context %}

## Decision

{% decision %}
The runtime reducer shall be the sole writer of a process-local `LifecyclePhase` and shall execute shutdown as a result-driven transaction with an acknowledged all-session Save barrier followed by deadline-aware cleanup.

`Running → Quiescing` occurs once when the first shutdown request is accepted. The transition registers a transaction and emits only an acknowledged all-session Save effect. Success means every pre-teardown session upsert completed its individual atomic rename and returns as a transaction-scoped barrier-success event.

Barrier success authorizes only `EffReleaseFrameSandboxes{Cause: RuntimeShutdown, Deadline}`. Runtime ingress normalizes the signal timeout or IPC default into one absolute transaction deadline; the first accepted request owns it and duplicate waiters cannot extend it. The interpreter runs subsystem Stop and sandbox cleanup under that deadline and emits exactly one transaction-scoped cleanup result: `completed` when every attempt returns, or `deadline_exceeded` when the deadline wins. Cooperative workers receive cancellation; non-cooperative workers are detached for process exit so the interpreter itself returns. Late worker completion emits no lifecycle event.

The matching cleanup result alone authorizes caller response/ack followed by `EffTerminateRuntime`, once. `completed` returns `committed`; `deadline_exceeded` returns the degraded outcome and still advances to Runtime-owned termination. The terminal effect closes the runtime's own done/result channel and ends the event loop for both IPC and signal ingress. Coordinator cancel remains an explicit fallback, not the only way to escape a blocked cleanup.

Barrier failure authorizes none of cleanup, success response, or normal termination. The upsert-only store is not rolled back: completed prefix files remain new and valid, while suffix files retain each session's valid last-successful version; lifecycle teardown deletes none. IPC receives an error and the reducer rolls back to retryable Running. `RequestShutdown(timeout) ShutdownResult` returns `committed`, `commit_failed`, or `deadline_exceeded`; signal handling treats `committed` as normal completion and explicitly logs then calls fallback `cancel()` for the other outcomes. Duplicate shutdown requests join the active transaction.

Before dispatching to individual reducers, a closed, exhaustive event classifier shall admit shutdown transaction events and side-effect-free read/connection bookkeeping, reject external mutation requests with unavailable, and neutralize every internal session-mutating late event in both state and effect. Unknown event types have no permissive default. Neutralization emits reason-labelled debug telemetry outside the durable projection.

LifecyclePhase and transaction metadata are excluded from `SessionSnapshot`, sessions store, published state, and Web projections. Every New/Bootstrap/LoadSnapshot begins Running.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Completion of every requested per-session upsert becomes a testable teardown barrier rather than a best-effort log.
- IPC and signal shutdown have the same terminal owner and ordering.
- An exhaustive pure admission matrix prevents any late subsystem, frame, spawn, driver, job, tick, file, or prompt event from mutating the restart snapshot.
{% /consequence %}

{% consequence kind="negative" %}
- The reducer and interpreter gain a transaction/result protocol, explicit IPC rollback, and typed signal fallback semantics.
- Collection-wide rollback is not attempted; partial failure intentionally exposes a valid mixed last-successful-per-session store.
- Read-only versus mutation event classification becomes a maintained closed contract whenever a new Event type is introduced.
- Clients may observe unavailable during Quiescing instead of a mutation racing to apparent success.
{% /consequence %}

{% consequence kind="neutral" %}
- The sessions store and Codex locator format do not change.
- Ordinary non-shutdown persistence retains its existing behavior.
- Sandbox preserve versus destroy remains a policy supplied after Save barrier success and is not decided here.
{% /consequence %}

Error triage: expected late mutation is eliminated from state semantics and counted diagnostically; Save partial failure is a recoverable shutdown error with mixed-store limited recovery; cleanup error or deadline after barrier success is a degraded result that does not roll back identity safety and cannot block terminal progress; an illegal lifecycle transition, stale result accepted as current, or missing event classification is an internal contract violation.

## Alternatives

- **Keep persist → response → release and merely suppress `context.Canceled`** — rejected. Save failure still looks successful, CLI exit can still delete a session, and IPC terminal ownership remains absent.
- **Add a generation/manifest for graph-wide point-in-time atomicity** — rejected for this change. It changes the store wire/read protocol and migration surface, while the required identity safety is achieved by the upsert-only all-session barrier plus mutation freeze.
- **Stop the event loop before all teardown** — rejected. It removes some late events but loses reducer ownership of cleanup result/ack and complicates resource-drain observability. The two-phase protocol supplies a smaller explicit boundary.
- **Put a quiescing flag only in the runtime loop** — rejected. It splits mutation authority between runtime and reducer and makes the pure state transition accept events production would drop.
- **Freeze only `EvSubsystem` and `EvFrameCommandExited`** — rejected. Spawn, vanish, driver hook, job, tick, file, OSC, prompt, and mutation RPC paths can also produce durable state/effects.

## Confirmation

The lifecycle, admission, projection, all-session barrier, multi-session failure-after-prefix, joined-waiter, deadline-aware cleanup, late-completion idempotency, typed shutdown outcome, terminal-owner, and restart tests in the change verification member are the fitness function for this decision.
