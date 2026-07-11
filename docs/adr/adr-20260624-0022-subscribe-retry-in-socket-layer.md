---
id: adr-20260624-0022-subscribe-retry-in-socket-layer
kind: adr
title: ADR 0022 — Subscribe retry lives in the socket layer, integrated with the Zustand
  store
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0018-defer-subscribe-race-to-beta}
- {type: supersededBy, target: adr-20260711-terminal-subscription-desired-reconcile}
source_paths:
- src/client/web/src/socket/retry.ts
decision_makers:
- unknown
summary: ADR 0018 deferred subscribe race handling to β. The α reducer returns RespErr{Code:'frame-not-ready'}
  when Sessions[sid].ActiveFrame() == nil. β must observe this and retry until the
  frame is ready (or until the user
---

<!-- migrated_from: docs/adr/0022-subscribe-retry-in-socket-layer.md -->

# ADR 0022 — Subscribe retry lives in the socket layer, integrated with the Zustand store

Status: Accepted

## Context

[ADR 0018](../adr/adr-20260624-0018-defer-subscribe-race-to-beta.md) deferred subscribe race
handling to β. The α reducer returns `RespErr{Code:'frame-not-ready'}` when
`Sessions[sid].ActiveFrame() == nil`. β must observe this and retry until
the frame is ready (or until the user gives up).

Three places to put the retry:

1. **Inside the Zustand store** — store action handles `frame-not-ready` and
   re-emits subscribe. Couples retry policy to state shape.
2. **Inside the WebSocket layer** — socket layer owns retry state; store
   only observes "subscribed" / "retrying" / "failed" status flags.
3. **As a React custom hook** — `useSubscribeWithRetry(sid)` wraps the
   subscribe lifecycle. Tied to component lifetime, hard to share across
   surfaces.

The reconnect-on-disconnect logic already lives in the socket layer
(FR-β06). Putting subscribe retry there too keeps both retry policies in
one place with one backoff implementation.

## Decision

Implement subscribe retry in `src/client/web/src/socket/retry.ts`:

- `subscribeWithRetry(sessionId: string): Promise<void>` issues
  `CmdSurfaceSubscribe`, awaits the response, and on `RespErr` with code
  `frame-not-ready` retries with the same exponential backoff used for WS
  reconnect (250 ms → 4 s, full jitter, cap 16 attempts).
- The retry state (`requested` / `confirmed` / `retrying` / `failed`) is
  reflected into a slice of the Zustand store (`store/subscriptions.ts`)
  so that UI components can render banners or spinners.
- After 16 failed attempts, retry stops and the store transitions to
  `failed`. The user must re-select the session to retry.
- On WebSocket reconnect (FR-β06), the socket layer re-iterates the
  store's subscription set and re-issues subscribes for any in
  `confirmed` state, using the same `subscribeWithRetry` flow.

## Consequences

- Retry policy is configured in one file and shared between reconnect and
  subscribe-race paths.
- The Zustand store stays UI-focused; retry mechanics live with the wire.
- Unit-testable: mock the WebSocket, feed `RespErr(frame-not-ready)`,
  assert backoff timing and final state transitions.
- A bug in the retry logic affects only the socket layer, not React
  component lifetimes.
- React hooks remain thin: they observe the store and trigger
  `subscribeWithRetry` once per session select.

## Alternatives

- **Store-internal retry** — couples retry mechanics to state shape;
  duplicates the reconnect backoff implementation.
- **React hook only** — retry state dies when the component unmounts; a
  fast-switching user could leave stale subscribes behind.
- **No retry; fail loud** — violates FR-β05 and the ADR 0018 contract.

## Related requirements

- FR-β05, FR-β06, FR-β12, FR-β15
