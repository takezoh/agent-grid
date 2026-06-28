# ADR 0004 — Reuse the pure core via a PtyBackend, not a parallel server stack

Status: Accepted

## Context

The tmux-free split (historical plan / design docs lived under `plans/` and were
removed once execution completed — see git history) shipped a phase-2 web stack — `platform/termvt`, `server/session`, `server/web`,
`client/web` — that operates pty-backed sessions and streams them to the browser.

That stack works, but it **bypasses the pure core**: `server/web` and
`server/session` import neither `client/state` (the reducer) nor `client/driver`.
The design's strategy (remote-client-design.md §2) was the opposite — *replace the
`TmuxBackend` implementation with a `PtyBackend`, keep the pure core untouched* —
so the runtime/reducer/driver run unchanged on a tmux-free backend.

We must decide how the two reconcile:

- **(i) PtyBackend** — wrap `platform/termvt` behind the existing `TmuxBackend`
  role interfaces (`client/runtime/backends.go`) so the unchanged
  runtime/reducer/driver drive pty sessions; the web gateway renders
  driver-derived state.
- **(ii) Server reimplementation** — leave the web stack separate and
  re-implement status detection / driver views / persistence on the server side.

### Due-diligence findings

The `TmuxBackend` seam splits cleanly into a **data plane** and a
**presentation plane**, and termvt already supplies the data plane:

| Historical `TmuxBackend` method (tmux CLI vocabulary) | `platform/termvt` primitive |
|---|---|
| `SpawnWindow` | `NewSession(Spec)` |
| `SendKeys` / `SendKey` / `SendEnter` / `PasteBuffer` | `WriteInput([]byte)` |
| `pipe-pane` (output tap) | `Subscribe()` (snapshot-first fan-out) |
| `capture-pane` | `Snapshot()` / `em.Render()` |
| `display-message -p '#{pane_width},#{pane_height}'` / `resize-window` | `Resize()` / `Size()` |
| `kill-window` | `Close()` |
| `pane_dead` / `pane_dead_exit_code` | `EventExit` (exit **code** not yet retained) |
| OSC 9/133 + title/bell tee | `registerOSC()` → `Control` event |

(Names in the left column are tmux CLI command/format strings, not agent-reactor
Go types. Agent-reactor's current backend has no "pane" concept.)

Two facts make (i) low-risk and incremental:

1. A complete no-op `TmuxBackend` (`noopTmux`) already exists "until production
   wiring lands" — proof the runtime can be wired to a non-tmux backend.
2. The data-plane methods map 1:1 onto existing termvt primitives; only small
   additions are needed (below).

The **presentation plane** — tmux layout commands (`swap-pane` / `break-pane` /
`join-pane` / `select-pane` / `run-shell`) and `TmuxControl`
(`SetStatusLine` / `DetachClient` / `KillSession` / `DisplayPopup`) — has no
server-side equivalent in a pty multiplexer. The design already anticipates
this: layout composition moves client-side (remote-client-design.md,
"client-side layout composition replaces the tmux 3-window control screen").

## Decision

Adopt **(i)**. Introduce a `PtyBackend` that implements the `TmuxBackend` role
interfaces over `platform/termvt`, and keep the pure core (`state.Reduce`,
`Driver`) unchanged.

- **Data plane** (`FrameLifecycle`, `FrameIO`, `FrameInspect`, `SessionEnv`,
  liveness): implemented for real against termvt. `FrameID` is the single
  key — there is no separate physical-handle namespace.
- **Presentation plane** (window layout, `BackendControl`): stubbed (like
  `noopTmux`) initially; relocated client-side in the tmux-removal phase.

This is the linchpin (plan §4, B1). It unblocks reuse of driver intelligence on
the web surface (plan A) and the eventual tmux removal (plan C).

## Consequences

**Positive**

- One source of agent intelligence; the web surface inherits driver views,
  run-state detection, tags, and persistence instead of re-deriving them.
- Unblocks tmux removal (plan C): once the runtime runs on PtyBackend, the tmux
  backend can be deleted.
- Incremental and testable: the seam + `noopTmux` let PtyBackend land method by
  method behind existing reducer/driver tests.

**Required termvt additions (small)**

- Retain `cmd.Wait()`'s exit code and expose it on `EventExit` (for
  `FrameExitStatus`).
- OSC parity: add 777 (notify), 7 (cwd), 99 to the existing 9/133/title/bell.
- `SendKey` named-key → byte translation (`Escape` → `0x1b`, etc.).
- `CaptureFrame` adapter: trailing N lines, SGR-stripped, from the emulator grid.

**Rejected — (ii) server reimplementation**

- Permanent duplication of driver logic across two code paths.
- Cements the design divergence rather than closing it.
- Does not enable tmux removal.

## Open questions — resolved in B1

- **Session ownership → the runtime's PtyBackend owns its own `termvt.Manager`.**
  `NewPtyBackend()` constructs a private `termvt.Manager`; the session daemon
  and `cmd/server` are separate processes, so each holding its own Manager
  cannot collide. B1 does **not** touch `server/session.Service` / `cmd/server`.
  Converging the web surface onto the daemon's runtime-owned sessions (so the
  web inherits driver intelligence, and `cmd/server` is absorbed or proxied) is
  plan A, not B1.
- **Reattach after daemon restart → not preserved across restart in B1.** termvt
  sessions are children of the session daemon and die with it — the same model
  as the already-shipped `cmd/server` (sessions survive client disconnect but
  not a host restart). Session *definitions* persist via `SessionSnapshot`; on
  restart the daemon cold re-spawns rather than re-attaching live processes.
  PtyBackend's `SetEnv`/`ShowEnvironment` are in-process only and documented as
  non-persistent; a tmux-session-env replacement for cross-restart frame recovery
  is deferred (a detached supervisor that outlives the daemon is explicitly out
  of scope here).

## Status of B1 implementation

The PtyBackend type and its unit tests are implemented and reviewed
(`client/runtime/pty_backend.go`; `platform/termvt` gained `Session.ExitCode`
and `CaptureTail`). Data plane is real, presentation plane is stubbed. It is
**not yet wired into the runtime** (`NewPtyBackend` is test-only). The
integration prerequisites before wiring — missing-frame error contract vs
the now-renamed sentinel (`ErrFrameMissing`), shell-string vs argv command form,
`Resize` target shape, session-env persistence, output tap wiring, main-window
kill guard — were
tracked under "B1-wiring の前提条件" in the (now-removed) `plans/` design docs;
all six were resolved or rendered moot by the time wiring completed.
