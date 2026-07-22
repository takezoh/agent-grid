---
id: adr-20260716-restart-continuity-compatibility-axes
kind: adr
title: Restart continuity is specified on three compatibility axes
status: proposed
created: '2026-07-16'
updated: '2026-07-16'
summary: The requested Claude-like restart behavior is split into identity/status, container adoption, and PTY/turn continuity; this change guarantees only the first plus conversation locator.
decision_makers:
- agent-grid-maintainers
- product-owner
consulted:
- runtime-maintainers
- Web-client-maintainers
- Codex-driver-maintainers
informed:
- agent-grid-users
tags:
- compatibility
- restart
- codex
- claude
relations:
- {type: partOf, target: change-20260716-codex-runtime-restart-continuity}
- {type: references, target: design-client}
- {type: references, target: adr-20260716-codex-observer-subscription-ready-ownership}
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
source_paths:
- docs/design/design-client.md
- src/cmd/server/coordinator.go
- src/client/runtime/bootstrap.go
- src/client/runtime/bootstrap_coldstart.go
- src/client/runtime/warm_state.go
- src/client/driver/codex.go
- src/client/driver/codex_persist.go
- src/client/driver/codex_resume.go
- src/client/runtime/subsystem/stream/resume.go
- src/client/runtime/subsystem/stream/subscription.go
- src/server/web/mux.go
consequences:
  positive:
  - Web-visible success is precise and testable as same session/frame/locator plus nonStopped after observer Ready.
  - Container policy can be decided independently without weakening the session continuity fix.
  - PTY and in-flight turn exactly-once claims are not implied by the word warm.
  negative:
  - The user-visible phrase Claude-like Cold/Warm start requires documentation explaining the three axes.
  - Signal-time container behavior remains unresolved until maintainers and the product owner choose between preserve/adopt and graceful destroy/recreate.
  - Identity continuity may succeed while a live terminal/turn does not continue, which the UI and release notes must not overstate.
  neutral:
  - Existing sessions store, ThreadID/RolloutPath, LoadSnapshot/RecreateAll/PrepareLaunch, and observer Ready contracts remain the recovery mechanism.
  - Both container alternatives must satisfy the same identity/status acceptance tests.
  - Promotion to design-client waits for user consultation and ADR acceptance.
confirmation: >-
  Gateway restart tests must prove identity/status continuity without asserting PTY or turn exactly-once, and the selected container policy must later add an explicit adoption-or-recreation scenario before design-client promotion.
---

# Restart continuity is specified on three compatibility axes

## Context

{% context %}
The requirement asks Codex Driver sessions to Cold/Warm start like Claude and reports a Web observation: sessions disappear or become Stopped after runtime restart. “Warm” can mean identity/status continuity, reuse of a preserved container, or continuation of a live PTY/in-flight turn. Those are distinct contracts.

The current stable client design describes signal-time container preservation, while the current coordinator follows `RequestShutdown` through resource release before cancellation. The available evidence establishes a conflict, not which side is authoritative. Declaring the document drift solely because code differs would turn implementation history into an unreviewed compatibility decision.
{% /context %}

## Decision

{% decision %}
Restart continuity shall be specified on three independent axes:

1. identity/status continuity: same SessionID, FrameID, persisted ThreadID/RolloutPath and nonStopped status after canonical observer Ready;
2. container adoption continuity: preserve/adopt an existing container versus destroy/recreate it;
3. PTY/turn continuity: reuse a live PTY and continue an in-flight turn exactly once.

This change shall guarantee axis 1 and conversation-locator continuity by reusing sessions store, `LoadSnapshot/RecreateAll`, `PrepareLaunch`, stream resume, canonical identity validation, and observer Ready. Axis 3 is explicitly a non-goal. Axis 2 remains a proposed compatibility choice: either documented signal preserve/adopt or current graceful destroy/recreate may be selected, but both must satisfy the same axis-1 tests and neither may change the shutdown snapshot contract.

`design-client` shall not be promoted to one container alternative until the user/decision makers accept that alternative. User-facing language shall not call PTY/turn continuity guaranteed when only identity/status continuity is implemented.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The reported Web failure maps to exact identifiers, locator, Ready, and status assertions.
- The session-loss fix no longer depends on settling sandbox/container cleanup policy first.
- Existing recovery and observer contracts are reused instead of creating a second warm-rebind path.
{% /consequence %}

{% consequence kind="negative" %}
- One compatibility decision remains open and blocks stable design promotion for signal-time container behavior.
- Product/UI documentation must distinguish restored conversation identity from live PTY/turn continuation.
- A later container-policy implementation needs its own adoption or recreation verification.
{% /consequence %}

{% consequence kind="neutral" %}
- Container preserve/adopt and destroy/recreate are both valid candidates until consultation resolves the conflict.
- Claude behavior is a comparison input, not an implicit implementation mandate for PTY or container internals.
- sessions store and Codex app-server topology remain unchanged.
{% /consequence %}

## Alternatives

- **Treat current graceful destroy as automatically authoritative and rewrite design-client** — not selected. Running code is evidence of behavior, not sufficient approval to replace a documented compatibility policy.
- **Retain documented signal preserve/adopt** — viable pending user decision. It may improve container continuity but requires proving adoption safety and interaction with cleanup deadlines.
- **Promote graceful destroy/recreate** — viable pending user decision. It simplifies process ownership but changes the documented signal behavior and may lose container-local warm state.
- **Define “warm” as live PTY/turn exactly-once continuity** — rejected for this change. It requires process handoff and turn idempotency contracts absent from the reported identity/status requirement.
- **Create a new Codex warm-rebind path** — rejected. Existing cold-start recovery already carries the durable conversation locator and observer Ready; a second path would split ownership.

## Confirmation

The public gateway scenario confirms axis 1. Any later container policy acceptance must add its own scenario without changing axis-1 assertions. No test or documentation may claim axis 3 from this change.
