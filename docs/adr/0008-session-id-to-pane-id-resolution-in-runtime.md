# ADR 0008 — Resolve `SessionID` to backend handle in runtime, not in proto

Status: Superseded

> ⚠️ **Superseded by ADR 0004 and commit 73603f4d (Go backend `pane` vocabulary scrub).**
> The "paneID" abstraction described below no longer exists: `termvt.Manager`
> now keys on `string(FrameID)` directly and there is no separate physical-handle
> namespace inside the backend. The wire/runtime/state layers all share the
> same `FrameID` and no translation step exists. This ADR is retained as
> historical context only.

## Context (historical)

The `termvt.Manager.Get` key space used to be a physical "paneID" (e.g. `"pty:1"`
for `PtyBackend`, `"%1"` for the tmux backend) — physical resource IDs allocated
by `PtyBackend.SpawnWindow`. The state-level `SessionID` (e.g. `"s1"`) is a
distinct, logical ID minted by `Reduce(EvCreateSession)`. A single session can
own multiple frames; the currently visible one is
`Sessions[sid].ActiveFrame().TargetID`.

Naively calling `mgr.Get(SessionID)` always returned not-found. The browser must
not see backend-internal handles (they couple the wire to backend choice). So
someone had to translate.

## Decision (historical)

Translate `SessionID → ActiveFrame.TargetID` inside `client/runtime` when
interpreting `EffSurfaceSubscribeStart` / `EffSurfaceWriteRaw` /
`EffSurfaceResize`. The wire and `Reduce` deal exclusively in `SessionID`;
`PtyBackend` saw only the physical handle.

If `Sessions[sid].ActiveFrame()` is `nil` at the moment of `Reduce`, the
reducer returns `RespErr(frame-not-ready)` immediately — see
[ADR 0018](./0018-defer-subscribe-race-to-beta.md) for the race deferral.

## Why this was undone

Subsequent refactors (ADR 0004, the frame-namespace migration, and the final
vocabulary scrub in commit 73603f4d) eliminated the separate physical-handle
namespace. `FrameID` is now the single backend key, and there is no mapping
step. The current backend has no "paneID" concept at all.

## Related requirements

- FR-014, FR-015, FR-016
