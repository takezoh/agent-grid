---
id: adr-20260624-0001-multiplexed-backends-shared-routing-contract
kind: adr
title: ADR 0001 — Multiplexed backends are verified by a shared routing-isolation
  contract
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: references, target: component-20260624-client-stream-backend-e2e}
- {type: references, target: note-20260624-technical-code-enforcement}
- {type: referencedBy, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: referencedBy, target: adr-20260624-0003-termvt-fanout-isolation}
- {type: referencedBy, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: referencedBy, target: component-20260624-client-stream-backend-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
source_paths:
- src/client/runtime/subsystem/stream/
- ARCHITECTURE.md
decision_makers:
- unknown
summary: The stream subsystem backend (client/runtime/subsystem/stream) fronts one
  codex app-server connection but multiplexes many frames (agents) over it. Inbound
  server events are demultiplexed by threadID → frameID
---

<!-- migrated_from: docs/adr/0001-multiplexed-backends-shared-routing-contract.md -->

# ADR 0001 — Multiplexed backends are verified by a shared routing-isolation contract

Status: Accepted

## Context

The stream subsystem backend (`client/runtime/subsystem/stream`) fronts **one**
codex app-server connection but multiplexes **many** frames (agents) over it.
Inbound server events are demultiplexed by `threadID → frameID` (`b.threads`).

A new turn's thread id is **not** returned synchronously by `turn/start`; it
arrives later in an async `thread.started`. The backend then guesses which frame
the thread belongs to (`resolveFrameForStartedThread`): it matches the start
`cwd` against unbound frames and, when that is ambiguous, **falls back to the
currently active frame** (`activeLookup`). Once a thread is bound to the wrong
frame, every subsequent event for it — assistant text, tool output — is routed
there. The result is **cross-talk**: one agent's output (including tool results
like `ssh`/file reads) surfaces in another agent's session, and the receiving
model confabulates around the foreign input.

This became systemic once shared-container isolation made multiple frames share
one `cwd` (the ambiguous case became the norm). The existing tests missed it:
they covered single-frame flows and *structural* multi-frame binding (map
integrity), never the invariant that an event reaches **only** the frame that
started its thread.

The `activeLookup` fallback is a concrete violation of the cross-layer **"No
fabricated fallbacks"** principle ([ARCHITECTURE.md](../../ARCHITECTURE.md#design-principles)):
it invents ownership truth when the real answer is unknown.

## Decision

Pin the **Routing Isolation Invariant** with a shared, reusable contract before
fixing the demux:

> Every `state.EvSubsystem` emitted from a thread T carries `FrameID == owner(T)`,
> where `owner(T)` is the frame whose `BindFrame` started/resumed T. No event
> reaches any other frame.

Mechanics (all in `client/runtime/subsystem/stream`):

- **`recordingRuntime`** captures every emitted `EvSubsystem` by `FrameID`;
  unique marker strings (carried in `LastAssistantMessage`) tie an event back to
  the thread that produced it. `assertMarkerFrames` is the single invariant check.
- **Direct-drive contract** (`routing_contract_test.go`) feeds the event handlers
  synchronously — deterministic, no goroutines.
- **Wired harness** (`routing_wired_test.go`) drives the real `codexclient.Conn`
  against an in-process fake app-server built on `codexclient.Server`, so the
  async read loop is exercised under `-race` and the fake emits the **same wire
  shapes** production does.
- **Fuzz** (`routing_fuzz_test.go`, stdlib `testing.F`) explores interleavings of
  binds/starts/active-switches/messages.

The bug-reproducing ("cross-talk") cases are **RED** on the current demux and are
gated behind `AG_ROUTING_PINS` so CI stays green until the fix; the
GREEN regression guards (distinct-cwd routing, release cleanup, …) run always.
The fix that binds threads by their initiating request flips the pins GREEN, at
which point the gate is removed and the cases become permanent regression cover.

Any future multiplexed backend adopts this contract rather than rolling its own
per-backend assertions.

## Consequences

- The class of bug ("an agent's output appears in another agent") is caught by a
  named, load-bearing invariant rather than by chance.
- The in-process fake is validated against a real app-server (the e2e uses
  distinct cwds: it confirms the fake routes like a real server, not that the
  cross-talk bug is absent), so a green contract against the fake is trustworthy
  — see [ADR 0002](../adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md).
- The enforcement is **test-pinned** (not statically lint-able); it is catalogued
  in [code-enforcement.md](../note/note-20260624-technical-code-enforcement.md).

## Update — fix landed

The demux fix is implemented: `bindThread` creates the cold-start thread
synchronously via a `thread/start` request and binds the returned id (mirroring
resume), so binding no longer depends on an async `thread.started` or the start
cwd. `resolveFrameForStartedThread` and the `activeLookup` fallback are removed;
`handleThreadStarted` only confirms an already-bound thread and drops unknown
ones. The `AG_ROUTING_PINS` gate is gone — the cases are now permanent
regression guards (same-cwd frames get distinct ids and cannot cross-talk).
Because the spawned frame now resumes the daemon-created thread (cold start uses
`codex resume <id> --remote`), this change to the spawn/attach contract must be
verified against a real app-server via the opt-in e2e
([stream-backend-e2e.md](../component/component-20260624-client-stream-backend-e2e.md)).

## Update — passive adopt (2026-07-01, see ADR-0081)

Empirical verification showed the "backend creates T1 via `thread/start`, CLI
resumes T1 via `codex resume <id> --remote`" mechanism above **does not work**
for fresh cold-start:

- `codex resume <id> --remote` requires a **local rollout file** at
  `~/.codex/sessions/…/rollout-<id>.jsonl` with valid content. codex CLI
  performs this local check before ever contacting the remote endpoint.
- The app-server only writes the rollout file after the **first turn** runs
  on the thread — not on `thread/start` alone. So the backend-created T1
  starts as a 0-byte file, `codex resume` client-side check fails with
  "No saved session found", and the CLI never attaches.
- As a workaround the pre-restructure code left `RemoteAttachArgs` without
  the `resume <id>` prefix. That in turn caused the CLI to issue its **own**
  `thread/start` on its connection, producing a second thread T2 that the
  backend received via broadcast but silently dropped
  (`frameForThread(T2) == ""`) — the Idle-stuck-badge production bug.

The current design (ADR-0081) inverts thread ownership: the CLI owns the
thread lifecycle (Claude Driver's model, applied to codex). Backend no longer
calls `thread/start` at all. Fresh interactive frames are pre-registered
under an `initSem` semaphore (capacity 1, holds the FrameID of the
currently-adopting frame); when the CLI's thread/started broadcast arrives,
`handleThreadStarted` drains the slot and binds the id — the serialisation
invariant guarantees "at most one pending frame" so no cwd/heuristic
disambiguation is needed. Cold-start recovery still uses `codex resume
<persistedID> --remote`, but the id comes from a rollout the CLI itself
wrote in a prior session (guaranteed non-empty), so the local check passes.

The Routing Isolation Invariant at the top of this document still holds —
same-cwd frames still get distinct thread ids because the CLI mints fresh
ids, and adopt maps them 1:1 to the single pending frame.
