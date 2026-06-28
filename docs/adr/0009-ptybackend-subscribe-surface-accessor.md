# ADR 0009 — Expose `SubscribeSurface(target)` on `PtyBackend`

Status: Accepted

> **2026-06-28 update**: Phase C has since landed, and the subsequent
> vocabulary scrub (commit 73603f4d) renamed `paneID` to `target` /
> `string(FrameID)` everywhere. The runtime field is now
> `Config.Backend FrameBackend`, and the legacy "tmux backend" alternative
> no longer exists. The justifications below that refer to "the tmux backend"
> or "paneID" are preserved as historical context — they describe the
> constraint the ADR was decided under, not the current code shape.

## Context

`PtyBackend` currently holds `mgr *termvt.Manager` as an unexported field.
A1-α's `terminal_relay` needs to reach `termvt.Session.Subscribe` to fan output
out to subscribed connections, but it sits behind the `cfg.Backend` (formerly
`cfg.Tmux`) backend abstraction. Reaching into `mgr` directly would bypass the
abstraction and prevent future backend swaps (e.g. phase D's remote backend).

## Decision

Add three methods to `PtyBackend`:

```go
SubscribeSurface(target string) (*termvt.Subscription, error)
WriteSurface(target string, data []byte) error
ResizeSurface(target string, cols, rows int) error
```

`target` (the `string(FrameID)` key into the `termvt.Manager`) is the only
argument. `ConnID` and `SessionID` are not exposed — they are state-level
concerns that the runtime owns. The tmux backend implements these as
`ErrNotImplemented` until phase C removes it.

`platform/termvt` is **not modified**. The accessors are pure forwarders to
`mgr.Get(target)` plus existing termvt methods.

## Consequences

- termvt's API stays frozen; only the backend abstraction gains symmetry.
- The backend interface stays free of state-level types (`state.ConnID`,
  `state.SessionID`) — no reverse import direction.
- Phase D's remote backend implements the same three methods, and the wire
  carries only the backend `target` (`string(FrameID)`).
- Three more methods become visible on the backend surface. tmux backend
  paths return `ErrNotImplemented`, which is acceptable since tmux backend is
  scheduled for removal in phase C.
- `terminal_relay` holds the per-`(ConnID, SessionID)` map; the backend holds
  none of it.

## Alternatives

- **Access `termvt.Manager` directly from runtime** — rejected because it
  breaks the backend abstraction and blocks future backend swaps.
- **Add `SubscribeRaw` to `platform/termvt` itself** — rejected because
  modifying `platform/termvt` is out of scope for α (the layer below the
  backend abstraction stays frozen).

## Related requirements

- FR-014, FR-015, FR-016
