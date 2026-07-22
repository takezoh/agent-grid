---
id: adr-20260716-subsystem-termination-linearization
kind: adr
title: First terminal observation classifies subsystem termination
status: proposed
created: '2026-07-16'
updated: '2026-07-16'
summary: Subsystem Stop carries a typed cause, and Stop intent races process Wait through one linearization point whose first observation is canonical.
decision_makers:
- agent-grid-maintainers
consulted:
- runtime-maintainers
- Codex-driver-maintainers
informed:
- agent-grid-users
tags:
- subsystem
- codex
- process
- concurrency
relations:
- {type: partOf, target: change-20260716-codex-runtime-restart-continuity}
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
source_paths:
- src/client/runtime/subsystem/subsystem.go
- src/client/runtime/subsystem/stream/backend.go
- src/client/runtime/subsystem/stream/factory.go
- src/client/runtime/subsystem/cli/backend.go
- src/client/runtime/subsystem/cli/factory.go
- src/client/runtime/interpret.go
- src/client/runtime/subsystem_dispatch_test.go
consequences:
  positive:
  - Expected daemon or last-frame teardown cannot be misreported as a Codex app-server failure.
  - Unexpected process death remains observable for every still-bound frame without deleting durable session identity.
  - Mutex/once classification gives race tests one canonical winner and at-most-once failure emission.
  negative:
  - Every Subsystem implementation, Reaper caller, and test double must accept and test a StopCause.
  - The stream backend must coordinate Stop, Wait, parent cancellation, binding snapshots, and telemetry under a small critical section.
  - A different cause supplied after the winner is retained only as diagnostic mismatch, not as a behavior override.
  neutral:
  - StopCause has exactly RuntimeShutdown and LastFrameRelease; no general reason taxonomy or restart policy is introduced.
  - CLI may implement Stop as a typed no-op but remains compile-time subject to the boundary contract.
  - Codex protocol resume and observer subscription ownership are unchanged.
confirmation: >-
  Race-enabled table tests must cover Stop-before-Wait, Wait-before-Stop, parent cancel, duplicate same/different causes, last-frame zero bindings, and at-most-once failure; T2 conformance must compile every implementation and fake.
---

# First terminal observation classifies subsystem termination

## Context

{% context %}
`Subsystem.Stop(context.Context)` does not carry intent. The stream backend cancels its app-server and its waiter unconditionally reports process termination as `SubsystemFailed` to bound frames. The same app-server exit can then cause the Codex CLI frame to exit, where exit codes otherwise used for user termination can evict the session.

A typed cause alone is insufficient: daemon shutdown, last-frame Reaper removal, parent cancellation, process Wait, and duplicate Stop calls race. Classification needs one linearization point, otherwise two goroutines can independently decide expected and unexpected outcomes.
{% /context %}

## Decision

{% decision %}
`Subsystem.Stop` shall require a typed `StopCause`, limited to `RuntimeShutdown` and `LastFrameRelease`. The stream backend shall route Stop intent and process Wait completion through one mutex/once (or equivalent CAS) terminal classifier. The first terminal observation is canonical.

Stop records its expected cause through the classifier before cancelling the process. If it wins, later Wait, parent cancel, duplicate Stop, or a different StopCause cannot change the classification; no `SubsystemFailed` is emitted. If process Wait wins before an expected Stop intent is recorded, the exit is unexpected and each still-bound frame in the atomically captured binding snapshot receives one failure. A later Stop only performs idempotent cleanup and cannot erase the failure. Parent context cancellation without a recorded StopCause does not invent expected intent.

Duplicate Stop with the same cause is a no-op. Duplicate Stop with a different cause retains the first cause and emits a diagnostic mismatch. Last-frame Reaper passes `LastFrameRelease`; daemon cleanup passes `RuntimeShutdown`. All implementations and fakes expose the same compile-time method.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Intentional termination and real process failure have a single race-safe decision owner.
- Expected teardown emits reason telemetry but no failure, while unexpected Wait preserves failure observability.
- The concurrency contract can be exhaustively exercised with a deterministic Wait seam and race detector.
{% /consequence %}

{% consequence kind="negative" %}
- Interface migration touches CLI, stream, factories/Reaper, interpreter, and test doubles.
- The terminal classifier must snapshot still-bound frames consistently with release, increasing mutex discipline.
- First-cause-wins means a late higher-level cause cannot rewrite telemetry; mismatches require separate diagnostics.
{% /consequence %}

{% consequence kind="neutral" %}
- This decision classifies termination; it does not auto-restart an app-server or resume a turn.
- Runtime Quiescing remains the defense that prevents any separate frame-exit event from mutating the pre-teardown session set guarded by the Save barrier.
- Existing frame and observer lifecycle ownership remains intact.
{% /consequence %}

Error triage: expected StopCause is removed from the error domain; unexpected Wait is a recoverable external failure; an unknown StopCause or double failure emission is an internal contract violation.

## Alternatives

- **Infer intent from `context.Canceled`** — rejected. Parent cancellation and explicit expected stop share the same error and race ordering.
- **Infer intent from exit code** — rejected. Codes 0/129/130/137/143 can represent user exit, daemon teardown, or external kill; they do not carry owner intent.
- **Add a stream-only `Quiesce` method** — rejected. It creates two termination entrypoints and leaves Reaper/CLI/test doubles outside one contract.
- **Last caller wins** — rejected. Outcome would depend on scheduling and could retract an already emitted failure.

## Confirmation

The termination race matrix, race detector, invariant-named subsystem conformance, and expected/unexpected telemetry assertions in the verification member confirm this decision.
