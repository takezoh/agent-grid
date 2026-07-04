---
id: adr-20260624-0023-view-update-broadcast-shape
kind: adr
title: ADR 0023 — `view-update` broadcast is a 1:1 wire-level mirror of `EvtSessionsChanged`
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0023-view-update-broadcast-shape.md -->

# ADR 0023 — `view-update` broadcast is a 1:1 wire-level mirror of `EvtSessionsChanged`

Status: Accepted

## Context

A1-α landed the wire transport but never broadcast driver views to the
browser; A1-β built the React/Zustand client that *expects* a `view-update`
frame. A1-γ connects the two. The choice is how to shape the broadcast.

Three options:

1. **1:1 mirror**: `EvtSessionsChanged` → `viewUpdateFrame` with every
   field copied verbatim. The wire matches the internal event.
2. **Delta compression**: only send the sessions whose View changed since
   the last broadcast. Smaller frames, but the daemon must track per-
   subscriber baselines.
3. **Periodic snapshot**: broadcast at 1 Hz regardless of changes. Simple
   on the daemon side, but adds chatter for idle sessions.

The internal event is already the right shape: it fires only when state
changes, and it carries the full sessions slice. Mirroring it requires no
new state tracking and keeps the daemon's broadcast logic identical to
the in-process consumer.

## Decision

The gateway translates every `EvtSessionsChanged` into one
`viewUpdateFrame` and forwards it to every subscribed WebSocket. The
frame shape is:

```ts
{
  k: 'v',
  sessions: SessionInfo[],   // each with optional View
  activeSessionID: string | null,
  features: string[],
  serverTime: number,
}
```

No de-duplication, no delta computation, no client-side reconciliation
beyond store replace.

## Consequences

- The browser store reducer is a one-line `sessions = frame.sessions`
  replace; React reconciliation handles the per-session diff.
- Broadcast cost scales with `sessions.length × subscriber_count` per
  event. For typical workloads (≤ 20 sessions, ≤ 5 browsers) this is
  negligible.
- If the session list grows beyond ~100 entries, delta compression
  becomes worth revisiting; until then it is premature optimization.
- Wire and internal event stay in lockstep, simplifying observability
  and replay.

## Alternatives

- **Delta compression** — saves bandwidth for noisy single-session
  updates but doubles daemon-side complexity (per-subscriber baselines).
  Rejected until scale demands it.
- **Periodic 1 Hz snapshot** — wastes bandwidth for idle sessions and
  delays state propagation up to 1 second. Rejected.

## Related requirements

- FR-γ01, FR-γ02, FR-γ03, FR-γ10
