# ADR 0024 — StatusLine elapsed is computed client-side at 1 Hz

Status: Accepted

## Context

`view.View.StatusLine` is a free-form string driver authors fill with the
current status text. `view.View.StatusChangedAt` is an absolute timestamp
indicating when the run state last changed. A faithful elapsed-time display
shows "running for 12s" or "idle since 3m" — a counter that ticks every
second.

Two implementation choices:

1. **Daemon push at 1 Hz**: the daemon broadcasts a fresh
   `EvtSessionsChanged` every second so the StatusLine reflects current
   elapsed. Simple on the client but bloats the wire and the daemon
   event loop.
2. **Client-side clock**: the daemon pushes only on real changes; the
   client recomputes elapsed from `StatusChangedAt` against `Date.now()`
   every second.

Option 1 multiplies broadcast volume by N sessions × M browsers per
second for what is essentially a derived value. Option 2 isolates the
ticking concern in one React effect.

## Decision

Compute elapsed client-side. `DriverViewPanel` registers a single 1 Hz
`setInterval` that triggers re-render of the elapsed-display sub-
component. The daemon broadcasts on real `EvtSessionsChanged` events
only.

Implementation hints:

- One global ticker (in a custom hook `useNow1Hz()`) shared by all
  StatusLine displays — not per-component intervals.
- `StatusChangedAt` is sent as a JS-compatible number (ms since epoch)
  on the wire (Go's `time.Time` JSON-marshals to RFC3339 by default;
  consider a custom MarshalJSON or a separate `status_changed_at_ms`
  field).
- When clocks drift (client wall-clock vs daemon wall-clock), the
  elapsed may look slightly off (sub-second). This is acceptable for α;
  if it becomes a UX issue, anchor on `hello.serverTime` and subtract a
  computed offset.

## Consequences

- Wire volume stays bounded by real state changes.
- The 1 Hz ticker is a single timer, not N timers.
- Elapsed values are approximate (client wall clock can drift). For a
  developer-facing tool this is acceptable.
- The wire must carry `StatusChangedAt` in a parseable form on the
  TypeScript side; document the format in the wire types.

## Alternatives

- **Daemon push at 1 Hz** — wastes bandwidth and CPU for derived data;
  rejected.
- **Static elapsed snapshot** — easiest but loses the live counter feel;
  rejected (does not meet FR-γ06).

## Related requirements

- FR-γ06
