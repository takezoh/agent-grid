---
id: adr-20260624-0081-codex-frame-init-serialize
kind: adr
title: ADR 0081 — Codex frame init serialize + passive adopt
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: referencedBy, target: component-20260624-client-overview}
- {type: referencedBy, target: component-20260624-client-process-model}
- {type: referencedBy, target: component-20260624-client-stream-backend-testing}
- {type: referencedBy, target: adr-20260711-0082-frame-exec-launcher}
- {type: referencedBy, target: spec-20260711-frame-exec-launcher}
source_paths:
- src/client/runtime/subsystem/stream/
decision_makers:
- unknown
summary: The stream backend fronts a per-session codex app-server (stream:session:<sessionID>
  — see factory.go) and multiplexes N frames over one WebSocket connection. thread/started
  notifications broadcast to every connected
---

<!-- migrated_from: docs/adr/0081-codex-frame-init-serialize.md -->

# ADR 0081 — Codex frame init serialize + passive adopt

Status: Accepted

## Context

The stream backend fronts a per-session codex app-server (`stream:session:<sessionID>` — see `factory.go`) and multiplexes N frames over one WebSocket connection. `thread/started` notifications broadcast to every connected client indiscriminately (no per-client filtering exists in the v2 protocol), so the backend must decide which frame each unknown thread belongs to.

Empirical constraints established while diagnosing the "Idle-stuck-badge" bug (2026-07-01, see [ADR-0001](../adr/adr-20260624-0001-multiplexed-backends-shared-routing-contract.md) Update — passive adopt):

1. **codex CLI owns thread creation.** `codex --remote` issues its own `thread/start` on its connection. There is no `--attach-thread <id>` flag or `CODEX_THREAD_ID` env var — the id is generated internally.
2. **codex resume `<id>` --remote` requires a local rollout file with content.** Backend's own `thread/start` does NOT create such a file (empty rollout is created on first turn, not on `thread/start`). So the ADR-0001 workaround of "backend creates T1, CLI resumes T1" fails client-side with "No saved session found".
3. **App-server broadcasts to every connected client.** Backend and CLI are two different clients on the same app-server; both hear each other's `thread/started`.

The pre-restructure design (backend calls `codexclient.StartThread`, then spawns `codex --remote` without `resume`) left backend and CLI creating **two** threads that both existed forever. Backend routed events by `frameForThread(id)` and silently dropped everything the CLI's thread carried — including the `turn/started` that drives driver Status Idle → Running.

Options considered:

- **Fabricated rollout hack** (backend writes minimal session_meta to `~/.codex/sessions/…/rollout-<id>.jsonl` before CLI spawn). Works empirically but depends on undocumented codex file format and is fragile against upstream changes.
- **Per-frame app-server** (one app-server subprocess per frame instead of per session). Structurally eliminates broadcast ambiguity but N-multiplies process count for multi-frame sessions.
- **cwd disambiguation + adopt** (frame binding by matching `cwd` field of `thread/started`). Fails structurally when two frames share a cwd (deliberately banned by ADR-0001).
- **`initSem` serialize + passive adopt** (this ADR).

## Decision

Backend is a **passive router**. It never calls `thread/start` or `thread/resume` itself. Thread lifecycle owner:

- **Fresh cold-start**: the codex CLI (`codex --remote unix://<sock>`) creates the thread and broadcasts `thread/started`. Backend adopts it into the pending frame reserved by the initSem semaphore.
- **Cold-start recovery**: backend receives a persisted `ThreadID` from state and spawns `codex resume <id> --remote unix://<sock>`. The CLI attaches to that thread via its local rollout (written by a prior session's turns, guaranteed non-empty). Backend pre-registers the id so the resulting broadcast routes deterministically without touching initSem.

Serialization mechanism (per Backend):

- `Backend.initState *initState` — a mutex-guarded `*pendingSlot` field + per-generation `free chan struct{}` for wait wakeup. Four atomic operations under `initState.mu`: `acquire(ctx, frameID)` blocks up to `initAcquireTimeout` if a slot is already held; `takeAny()` drains it on unknown thread arrival (adopt); `takeIfOwned(frameID)` drains it on frame kill (ReleaseFrame, gated by pending-check under `b.mu`); `takeIfExpired(now)` drains it in the reaper. Every transition-to-empty closes `free` and replaces it, so blocked acquirers wake race-free.
- A `reapExpiredSlots` goroutine periodically calls `takeIfExpired` and, on a hit, emits `SubsystemFailed` via `failFrame` then reuses `Backend.ReleaseFrame` for the binding + worktree cleanup — safety net for the case where a spawned CLI crashes before its first `thread/start`, so neither adopt nor Runtime-driven ReleaseFrame fires.

An earlier iteration used `chan pendingSlot` (capacity 1) as the semaphore; its non-atomic drain-check-put-back pattern created a race chain (reaper drops legitimate thread/started, reaper cross-talks with concurrent adopt, releaseOwnSlot deadlocks or displaces the wrong frame) that surfaced across successive code-review passes. The mutex+optional redesign removes every put-back window by making all state transitions atomic.

The **invariant**: at any moment, **at most one frame per Backend** has `binding.threadID == ""`. Therefore when an unknown thread arrives, the mapping to a pending frame is unique by construction — cross-talk (ADR-0001) is preserved without any cwd/heuristic disambiguation, and functionality is preserved without any hack of codex-internal file formats.

## Consequences

- **Concurrent fresh BindFrame within one session is serialized** (typical use: one codex frame per session → no visible cost; multi-frame push scenarios: second frame's BindFrame blocks up to ~100 ms per adopt round-trip).
- **Recovery ids must come from a codex-written rollout**. `state.DriverState.SessionID` (populated via `SubsystemPayload.ColdStartSessionID` in `handleSubsystem`) is such an id — the CLI wrote it in a prior session.
- **BindFrame returns `Plan.Stream.ColdStartSessionID == ""` for fresh cold-start**. The driver fills it in later via the `SubsystemSessionReady` payload (see `codex_event.go:63-65`) once the CLI's thread/started arrives with a real sessionId. Downstream persistence (`codex_persist.go`) tolerates this async fill by design.
- **Multi-frame concurrent init is limited to serial** — this ADR rejects per-frame app-server topology (higher resource cost) in exchange for keeping thread-lifecycle mechanics inside a single well-defined semaphore.

Related: [ADR-0001](../adr/adr-20260624-0001-multiplexed-backends-shared-routing-contract.md) (Routing Isolation Invariant), [ADR-0002](../adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md) (fake fidelity backstop).
