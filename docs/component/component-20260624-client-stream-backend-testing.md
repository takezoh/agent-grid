---
id: component-20260624-client-stream-backend-testing
kind: component
title: 'Stream backend: routing-isolation test harness'
status: active
created: '2026-06-24'
updated: '2026-07-04'
tags:
- technical
- client
- legacy-import
owners: []
relations:
- {type: referencedBy, target: component-20260624-client-stream-backend-e2e}
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: references, target: component-20260624-client-stream-backend-e2e}
- {type: referencedBy, target: component-20260624-platform-termvt-multiplexer-testing}
- {type: referencedBy, target: note-20260624-agent-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
source_paths:
- src/client/runtime/subsystem/stream/
provides:
- stream-backend-routing-isolation-test-harness
---

<!-- migrated_from: docs/technical/client/stream-backend-testing.md -->

# Stream backend: routing-isolation test harness

The stream subsystem backend multiplexes many frames (agents) over a single
codex app-server connection. Its one safety-critical property is **routing
isolation**: an event from a thread must reach only the frame that owns that
thread. A leak is *cross-talk* — one agent's output (including tool results)
surfacing in another agent's session. This page documents the harness that pins
that property. Rationale lives in
[ADR 0001](../adr/adr-20260624-0001-multiplexed-backends-shared-routing-contract.md) and
[ADR 0002](../adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md). Setup for the
real-server backstop: [stream-backend-e2e.md](../component/component-20260624-client-stream-backend-e2e.md).

## The invariant

> Every `state.EvSubsystem` emitted from a thread T carries `FrameID == owner(T)`,
> where `owner(T)` is the frame whose `BindFrame` started/resumed T.

Corollary: thread→frame binding derives from the **initiating request**, never
from ambient state such as the active frame (a "fabricated fallback").

## How the fix makes cross-talk impossible

`Backend.BindFrame` reserves a per-frame slot in `initState` — a
mutex-guarded `*pendingSlot` with a per-generation wait channel (see
`initsem.go`) — for fresh cold-start, and pre-registers the persisted
thread id for cold-start recovery. Backend itself never issues
`thread/start` — the codex CLI owns the thread lifecycle. When the CLI's
`thread/started` notification arrives, `handleThreadStarted` calls
`initState.takeAny()` to atomically consume the reservation and binds the
incoming thread id into the pending frame's binding; if the thread id was
pre-registered (recovery), the existing map entry routes the notification
directly. Two same-cwd frames therefore get distinct thread ids because
the CLI mints fresh ids per invocation, and the "at-most-one pending"
invariant means adopt has an unambiguous target — no cwd/heuristic guess
is ever needed. See [ADR-0081](../adr/adr-20260624-0081-codex-frame-init-serialize.md)
for the full contract, and [ADR-0001](../adr/adr-20260624-0001-multiplexed-backends-shared-routing-contract.md) Update — passive adopt for the empirical
evidence that motivated the switch away from backend-owned `thread/start`.

## Files

| File | Role |
|---|---|
| `routing_contract_test.go` | `recordingRuntime`, `assertMarkerFrames`, the direct-drive `inProc` harness, and `TestStreamRoutingContract` (the case table). |
| `routing_wired_test.go` | the `wired` harness driving the real `codexclient.Conn` against a `fake.AppServer` (WebSocket-over-UDS); tests the async adopt path under `-race`. |
| `routing_fuzz_test.go` | `FuzzStreamRouting` (stdlib `testing.F`) over random message/release interleavings. |
| `routing_e2e_test.go` | `//go:build e2e` real **app-server** fidelity backstop (any conforming server, not just codex); skips when no backend env is set. See [stream-backend-e2e.md](../component/component-20260624-client-stream-backend-e2e.md). |
| `routing_backstop_test.go` | Always-on fake-based version of the isolation invariant. Same `runIsolationScenario` shape as the e2e above but drives `fake.AppServer` directly, so the invariant is re-verified on every default `go test` run. |
| `init_serialize_test.go` | Pins the `initState` invariants (serialization, timeout, ReleaseFrame drain, reaper cleanup, silent-drop-on-no-pending). Regression net for ADR-0081. |
| `interactive_flow_test.go` | End-to-end integration: `fake.AppServer` + pty-spawned `fake.FakeCLI` + real `Backend`. Verifies the CLI-owned thread flows all the way to a driver `Status = StatusWaiting` transition after a prompt round trip. |
| `fake/` package | High-fidelity fake of codex-app-server + fake CLI (pty-attached, argv-compatible with `codex --remote`). Reused by wired, backstop, and interactive_flow tests. |

`recordingRuntime` is the shared observation point: it records each emitted
`EvSubsystem`'s `FrameID`, and markers travel in `Payload.LastAssistantMessage`,
so `framesWithMarker` answers "which frames received this thread's output".

## How a case is built

The direct-drive contract binds frames the way `bindThread` leaves them (each to
a distinct thread id), then feeds server events into the handlers:

```go
h := newInProc(t)
h.bind("A", "tA", "/work") // distinct thread id, even with a shared cwd
h.bind("B", "tB", "/work")
h.message("tA", "MARK_A")
h.message("tB", "MARK_B")
h.wantMarkerFrames("MARK_A", "A") // isolation: only A
h.wantMarkerFrames("MARK_B", "B")
```

The wired harness exercises the real path end-to-end: a cold `BindFrame` issues
`thread/start`, binds the returned id, and `TestStreamRoutingWiredIsolation`
asserts two same-cwd frames get distinct ids and never cross-talk — under
`-race`.

## Running

```sh
# regression guards + structural fuzz seeds (the ci job's test step;
# a separate `fuzz` CI job also actively fuzzes — see .github/workflows/ci.yml)
cd src && TMPDIR=/tmp go test ./client/runtime/subsystem/stream/

# concurrency check
cd src && go test -race ./client/runtime/subsystem/stream/

# active fuzzing
cd src && go test -run x -fuzz 'FuzzStreamRouting$' -fuzztime=30s \
  ./client/runtime/subsystem/stream/

# fidelity backstop against a real app-server (opt-in; see stream-backend-e2e.md)
REACTOR_E2E_CODEX_BIN=$(which codex) \
  go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/
```

## Invariant ↔ pinning tests

| Behaviour | Pinned by |
|---|---|
| Same-cwd frames (distinct ids) never cross-talk | `TestStreamRoutingContract/two_frames_same_cwd_distinct_threads`, `TestStreamRoutingWiredIsolation` |
| Completion routes by exact thread id | `.../completion_reverse_order` |
| `thread.started` confirms an already-bound thread | `.../thread_started_confirms_bound` |
| Unknown `thread.started` is dropped (no cwd/active adoption) | `.../thread_started_for_unknown_thread_drops`, `TestHandleThreadStartedUnknownThreadDrops` |
| Released frame drops stray events | `.../release_drops_stray_events` |
| Random interleavings preserve by-id isolation | `FuzzStreamRouting` |
| No duplication / garbage-frame / panic | `FuzzStreamRouting` (structural checks) |
| Fake matches real app-server wire behaviour | `TestStreamRoutingE2EIsolation` (opt-in, per backend) |
