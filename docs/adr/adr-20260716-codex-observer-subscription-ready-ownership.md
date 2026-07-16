---
id: adr-20260716-codex-observer-subscription-ready-ownership
kind: adr
title: Codex observer subscription and ready commit ownership
status: accepted
created: '2026-07-16'
decision_makers:
- unknown
tags:
- codex
- app-server
- driver
owners: []
relations:
- {type: partOf, target: plan-20260716-codex-thread-event-subscription}
- {type: supersedes, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: references, target: spec-20260716-codex-thread-event-subscription}
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: references, target: adr-20260707-shim-bytes-preserving-id-proxy}
source_paths:
- src/client/runtime/subsystem/subsystem.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/subsystem/stream/backend.go
- src/client/runtime/subsystem/stream/event.go
- src/platform/agent/codexclient/client.go
- src/client/runtime/subsystem/stream/fake/appserver.go
methodology: sdd
consequences:
  positive:
  - Backend observer subscription and SessionReady have one mutex-protected lifecycle
    transaction owner, closing the identity-known-but-unsubscribed gap.
  - The required ActivateFrame contract makes runtime resource commit an explicit,
    compile-time checked input for every subsystem.
  - Connection-scoped fake semantics and FakeVsReal coverage make status loss observable
    before production.
  negative:
  - Fresh and recovery attach each perform an additional backend ResumeThread, so
    attach latency and cleanup paths gain one external RPC.
  - Subsystem interface expansion requires explicit no-op implementations in CLI and
    all test fakes.
  - Release must classify asynchronous unsubscribe outcomes without holding the stream
    mutex across I/O.
  neutral:
  - ADR-0001 routing isolation remains unchanged; only its passive lifecycle ownership
    update is replaced.
  - Shim notification transparency remains unchanged because no targeted relay or
    broker is introduced.
  - The existing fake-versus-real ADR is extended by a connection-scoped subscription
    invariant rather than superseded.
confirmation: Required activation, one-shot ready, connection-scoped fake, release
  matrix, gateway status, and T3 fidelity tests listed in plan-20260716-codex-thread-event-subscription
  must pass.
summary: Codex frame observer subscription and SessionReady commit are explicit connection-scoped
  lifecycle responsibilities owned by one stream attach coordinator.
updated: '2026-07-16'
---

# Codex observer subscription and ready commit ownership

## Context

{% context %}
Codex app-server subscriptions are connection scoped. The current stream backend learns a fresh thread identity from a cross-connection `thread/started` broadcast or pre-registers a persisted recovery ID, then emits `SubsystemSessionReady` without making its own connection a subscriber. The TUI connection can therefore receive lifecycle events while the backend connection that owns driver routing receives none, leaving the Web status permanently Idle.

ADR-0081 correctly established the single `initState` pending slot and prevented ambiguous fresh identity adoption, but its broader “backend is a passive router and never resumes” decision conflates identity discovery with lifecycle observation. ADR-0001's one-thread-to-one-frame routing isolation remains necessary. The shim's bytes-preserving notification transparency also remains necessary and does not provide frame-correlated subscription ownership.

Runtime spawn completion is a second independent fact. Backend subscription may finish before wrap/spawn succeeds, or spawn may finish before identity/subscription. Publishing Ready from either side alone exposes a partially attached frame. Subscription ownership and ready ownership must therefore be one lifecycle transaction even though their inputs arrive from different boundaries.
{% /context %}

## Decision

{% decision %}
The stream attach coordinator shall own both the backend observer subscription and the one-shot `SessionReady` commit for each Codex frame.

`Subsystem` gains a required `ActivateFrame(frameID)` method. `Runtime.handleSpawnComplete` calls it only after loop-owned subsystem/frame maps, cleanup handle, token, and container resources are committed and before dispatching `EvFrameSpawned`. Every subsystem and test fake implements the method explicitly; non-stream implementations are no-op. Optional interfaces and type assertions are forbidden.

The stream binding records `runtimeActivated`, `observerSubscribed`, `canonicalIdentityValidated`, `readyCommitted`, and release/tombstone state under the existing backend mutex. The coordinator emits `SessionReady` exactly once when and only when:

`runtimeActivated && observerSubscribed && canonicalIdentityValidated`

Fresh identity discovery retains ADR-0081's existing `initState` pending slot and cross-connection `thread/started` observation. That broadcast is used only to bind the unique pending frame to an identity. Immediately afterward the backend connection calls `ResumeThread`, verifies the returned canonical ID, and commits its own subscription. The identity-only broadcast assumption is pinned against real Codex in T3; it is not promoted into a general lifecycle broadcast contract.

The notification handler only records the locator and schedules a generation-scoped subscription operation; it must return to `Conn.Run` before `ResumeThread` waits for its response. No app-server RPC executes while the backend mutex is held. The completion path reacquires the mutex and commits only if the same frame generation is still live. If release tombstones the generation while resume is in flight, a later successful response is stale and triggers compensating best-effort `UnsubscribeThread` instead of subscription or Ready commit.

Recovery uses the persisted thread ID to call backend `ResumeThread` and validate the canonical ID. The TUI separately executes `codex resume` on its own connection. This intentional double resume establishes one observer subscription and one interactive subscription without creating a second thread.

All post-bind wrap/spawn failures continue through the existing `ReleaseFrame` entrypoint. Release atomically commits a local tombstone, stops routing, performs best-effort typed `UnsubscribeThread` outside the mutex, and continues binding/worktree/launch cleanup. `unsubscribed`, `notSubscribed`, and `notLoaded` are idempotent success; timeout and closed transport are diagnostic external failure but do not block local cleanup; duplicate release is no-op.

The fake app-server stores introduced/subscribed thread registries per connection. `thread/start` and `thread/resume` subscribe the requester, `thread/status/*` targets subscribers, and `turn/*` targets the initiating connection. Bootstrap `thread/started` behavior is bounded by the real fidelity test rather than treated as a general fake broadcast rule.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Identity, subscription, activation, and Ready have one SSOT and one decision point, so an identity-known but unsubscribed frame cannot be published as ready.
- `ActivateFrame` is compile-time mandatory, making the runtime-to-subsystem lifecycle boundary explicit and testable.
- The external dependency triple—connection-scoped fake, invariant-naming contract, and FakeVsReal T3 backstop—detects fake drift and real protocol drift.
{% /consequence %}

{% consequence kind="negative" %}
- Backend `ResumeThread` adds an external RPC on fresh and recovery attach, and release gains best-effort unsubscribe latency and diagnostics.
- All production implementations and test doubles of `Subsystem` must add an explicit method, even when it is a no-op.
- The coordinator must avoid holding its mutex during ResumeThread/UnsubscribeThread I/O while preserving one-shot transition semantics, increasing concurrency test obligations.
- An in-flight resume can outlive frame release, so the generation check and compensating unsubscribe path are mandatory rather than optional cleanup.
{% /consequence %}

{% consequence kind="neutral" %}
- This ADR supersedes ADR-0081's passive-router/never-resume lifecycle decision while retaining its single pending-slot fresh identity mechanism.
- ADR-0001 routing isolation and collision rejection remain authoritative; only the later passive lifecycle update is replaced.
- Shim notification transparency is unchanged. No frame-aware relay, request broker, or notification fan-out responsibility is added to the shim.
- ADR-0002's fake-versus-real posture remains authoritative and is extended with the connection-scoped subscription invariant.
{% /consequence %}

Error triage: canonical ID mismatch is an internal contract violation and fails fast; resume/transport failure is external and fails the frame observably; unsubscribe notSubscribed/notLoaded is eliminated as an error by idempotent semantics; unsubscribe timeout/close is an external diagnostic failure with local degradation.

## Alternatives

- **Recovery-only backend resume patch** — rejected. It repairs one entry path but leaves fresh frames identity-known and unsubscribed, preserves split Ready ownership, and permits the same Web status freeze after fresh start.
- **Targeted shim relay or full JSON-RPC broker** — rejected. The existing pending slot already correlates fresh identity. A relay invents a frame/generation wire contract in a session-scoped shim; a full broker additionally assumes request-ID, server-request, approval, and fan-out ownership. Neither is required by any FR.
- **Passive broadcast lifecycle observation** — rejected. `thread/started` is retained only for fresh identity discovery because that observed cross-connection behavior is needed before an ID exists. Treating status/turn notifications as broadcast repeats the connection-scope mistake and lets the fake hide missing subscriptions.
- **Commit Ready from identity discovery or spawn independently** — rejected. Both orders admit a partially attached public frame. The conjunction is the smallest contract that covers asynchronous arrival without an optional callback or polling layer.

## Confirmation

The T0 predicate table, T1 runtime/stream and Go gateway scenarios, T2 protocol/fake contracts, minimum Web smoke, and T3 real Codex identity-only fidelity profile in `plan-20260716-codex-thread-event-subscription` collectively confirm this decision.


{% transition from="proposed" to="accepted" date="2026-07-16" %}
構造設計に基づく実装着手をユーザーが承認したため。
{% /transition %}
