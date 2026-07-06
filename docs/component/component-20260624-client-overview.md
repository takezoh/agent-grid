---
id: component-20260624-client-overview
kind: component
title: client/ — agent-grid client (Session Lifecycle Manager)
status: active
created: '2026-06-24'
updated: '2026-07-04'
tags:
- technical
- client
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: references, target: component-20260624-client-interfaces}
- {type: references, target: component-20260624-client-ipc}
- {type: references, target: component-20260624-client-process-model}
- {type: references, target: component-20260624-client-state-monitoring}
- {type: references, target: component-20260624-orchestrator-overview}
- {type: referencedBy, target: note-20260624-agent-overview}
- {type: referencedBy, target: note-20260624-docs-overview}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
- {type: referencedBy, target: note-20260624-technical-overview}
- {type: referencedBy, target: component-20260705-client-web-browser-harness}
source_paths:
- src/client/web/
- src/cmd/server/
- src/server/web/
- ARCHITECTURE.md
- src/client/state/
- src/client/runtime/
- src/client/state/view/
- src/client/driver/
provides:
- client-agent-grid-client-session-lifecycle-manager
---

<!-- migrated_from: docs/technical/client/README.md -->

# client/ — agent-grid client (Session Lifecycle Manager)

`client/` is all of the client: the in-process session daemon (state machine + runtime + drivers + IPC) and the browser frontend assets under `client/web/`. Both are shipped inside the `server` binary (`cmd/server`). It depends on `platform/` but **must not** import `orchestrator/` (enforced by the `depguard` rule `client-no-orchestrator`).

The agent-grid client is a *session lifecycle manager*, not an agent orchestrator. It gives you visibility and fast access to agents running across many projects; it does not decide what those agents do. The daemon owns sessions and exposes typed IPC over a Unix socket; the co-resident HTTP/WS gateway under `server/web/` translates browser REST + WebSocket traffic into IPC, so the browser is the operator's primary surface.

## Functional Core / Imperative Shell

This is the **canonical statement** of how `client/` realizes the cross-layer core principles ([ARCHITECTURE.md → Design Principles](../../../ARCHITECTURE.md#design-principles)). It is the strict Functional Core / Imperative Shell form — pure reducer plus zero mutexes — shared by **both decision-loop layers**: the orchestrator's `scheduler` realizes the same form (`scheduler.Reduce` over an immutable, mutex-free `State`; see the [orchestrator deep dive](../component/component-20260624-orchestrator-overview.md#design-principles-orchestrator-realization)). The pattern below is the reference implementation; `platform/`, being an I/O-wrapping library rather than a decision loop, uses dependency-injection seams instead.

- **Functional Core (`client/state/`)** — all state transitions are a pure function `state.Reduce(state, event) → (state', []Effect)`. No goroutines, mutexes, or actors (the no-mutex rule is enforced by `forbidigo`). Drivers run synchronously inside `Reduce`; the only permitted synchronous I/O is bounded read-only filesystem stat (e.g. checking whether a resume file exists). Everything else is emitted as an `Effect`.
- **Imperative Shell (`client/runtime/`)** — a single event loop owns state mutation and interprets `Effect` values into real I/O (PTY spawn / IPC writes / sandbox launch / worker pool). Long-lived I/O readers only *emit* events; they never read or write state. The worker pool (discrete jobs) and stream readers (continuous sources) are both instances of this principle.

This split is why the core is testable without mocks: `Reduce` and `Driver.Step` are verified purely by their return values.

## Packages

| Package | Responsibility |
|---|---|
| `client/state/` | Pure domain layer — `State`, `Event`, `Effect`, `Reduce`. No I/O, no goroutines. Imports only stdlib + stdlib-only internal packages (`features`). |
| `client/state/view/` | Wire-safe view types — `Status`, `View`, `Card`, `Tag`. Stdlib-only; no `state` import. |
| `client/driver/` | Driver implementations — value-type plugins + per-frame `DriverState`. No I/O. |
| `client/runtime/` | Imperative shell — single event loop, Effect interpreter, backend abstraction. |
| `client/runtime/worker/` | Worker pool — slow I/O jobs (summarize, transcript parse, git, github fetch). |
| `client/runtime/subsystem/` | `Subsystem`/`Factory` interfaces + the `cli` and `stream` implementations. The only place in `runtime/` allowed to import `driver/<tool>`. |
| `client/proto/` | Typed IPC wire layer — Command / Response / ServerEvent sum types + codec. Imports `state/view` only. |
| `client/proto/sessions/` | Session-management helpers wrapping `proto.Client`. Imports `state`. |
| `client/tools/` | Operator tool abstraction (palette-style tool invocation surfaced through IPC). |
| `client/web/` | Browser frontend assets (React + xterm.js) embedded by `cmd/server`. |
| `client/config/` | TOML loading, DataDir injection, SandboxResolver. |
| `client/cli/` | Subcommand registry — tool-specific subcommands registered via `init()`. |
| `client/lib/peers/` | Peers MCP server (IPC specific to the client). |
| `client/lib/{claude,codex}/transcript/` | Transcript renderers (depend on `state` for frontend integration). |

## Terminology

| Term | Meaning |
|---|---|
| **Session** | A unit of work for an agent. `state.Session` owns a stack of execution **frames** (`[]SessionFrame`). The active frame is the stack tail; the root frame defines the session's existence — if it dies, the session is deleted. |
| **Frame** | One execution context within a session, carrying its own `Command`, `LaunchOptions`, `DriverState`, `SubsystemID`, `TargetID`. Frame death truncates the stack from that frame onward; push-driver appends a new frame on top. |
| **Frame surface** | The pty surface attached to a frame, served by `PtyBackend` over `platform/termvt`. The backend keys its `termvt.Manager` on `string(FrameID)` directly — there is no separate physical-handle namespace. The browser xterm.js view subscribes to the same frame surface via the `server` gateway. |
| **Subsystem** | Runtime-owned execution backend (`Start/BindFrame/ReleaseFrame/Stop`). `cli` manages single-process per-frame launch and worktree lifecycle; `stream` fronts long-lived structured backends (Codex App Server). The stream subsystem resolves the per-session UDS the app-server binds (`Factory.ResolveSockPath`) and derives the host-side dial path from the launch's bind mounts (`WrappedLaunch.HostPath`), but delegates exec wrapping (direct vs `docker exec`) to the `agentlaunch.Dispatcher` it holds. |
| **Warm start** | Runtime startup against an existing `<dataDir>` — restores the frame stack from `sessions.json` and rebinds live frames; surviving containers are adopted. |
| **Cold start** | Runtime startup with no live frames (fresh boot / kill recovery) — respawns frames in root-to-tail order; surviving containers are discarded and provisioned fresh so `postCreate` daemons are guaranteed present. |

## Code dependency direction

- `main` → `runtime`, `driver`, `proto`, `tools`, `config`, `logger`
- `runtime` → `state` (calls `Reduce`), `proto` (wire codec), `runtime/worker`, `runtime/subsystem` (interface only — no concrete subsystem imports)
- `runtime/subsystem/<kind>` → `state`, `driver/<tool>` (constants/socket paths only), `lib/*`, `sandbox/`
- `runtime/worker` → `state` only (JobID, JobInput, EvJobResult); not driver/lib
- `state` is self-contained — stdlib + stdlib-only internal packages (`features`) only
- `state/view` → stdlib only; `state` re-exports its types as aliases
- `driver` → `state` (embed base types), `runtime/worker` (RegisterRunner), `lib/*`
- `proto` → `state/view` only (does **not** import `state`)

Frames route events: `Reduce` routes session-level events by sessionID and frame-level events (hooks, subsystem events, lifecycle) by frameID to the owning frame's `Driver.Step`.

## Daemon ↔ client processes

The daemon exposes typed IPC (`proto`) over a Unix socket. Two physical endpoints serve different client classes: the **host endpoint** (`<dataDir>/server.sock`, SO_PEERCRED auth) serves the co-resident HTTP/WS gateway, the `server event <type>` / `server host-exec` / `server mcp-exec` subcommands, and any future native client; the **container endpoint** (`<dataDir>/run/<project-hash>/server.sock`, bearer-token auth) serves sandboxed agents and accepts only `hook-event`/`subsystem-event`. See [process model](../component/component-20260624-client-process-model.md) and [IPC](../component/component-20260624-client-ipc.md).

## Design decisions

| Decision | Choice | Rationale |
|---|---|---|
| No optimistic updates | Do not modify view state on IPC error | Auto-recovers on next poll; avoids state inconsistency. |
| Shutdown semantics | `EffReleaseFrameSandboxes` runs on explicit shutdown; SIGINT/SIGTERM only persist `sessions.json` so containers survive warm restart | Container lifetime is a state-layer effect, ordered in the event loop rather than a defer stack. Sessions restore on the next boot via `sessions.json`. |
| Claude cold-start launch | Assemble `claude --resume <id>` in `Driver.PrepareLaunch(LaunchModeColdStart, …)` | `--resume` knowledge stays in the driver; the runtime interprets the baked plan verbatim. |
| Launch plan resolution | In the reducer (pure), with one cold-start bootstrap exception | Driver-specific logic stays in the pure core; the bootstrap goroutine is the only safe direct caller. |
| Resident tracking | `SubsystemID -> Subsystem` (`subsystems`), `FrameID -> Subsystem` (`frameSubsystems`), `FrameID -> SubsystemID` (`frameSubsystemIDs`), `FrameID -> TargetID` | These are **plain maps owned exclusively by the event loop** — no mutex (single-writer). The spawn goroutine holds no `*Runtime` and reports completion via an internal spawn-complete event; the loop is the sole writer. `subsystems` holds every live Subsystem keyed by its opaque SubsystemID, dispatched via per-kind Factories registered in `runtime.New`. `frameSubsystems` routes `ReleaseFrame` to the owning subsystem. `frameSubsystemIDs` is used by `reapSubsystemIfLast`: when the last frame of a Session is released, `Factory.Remove` is called to stop the app-server (stream subsystem reap). Shutdown ranges `subsystems` and calls `Stop` on each. CLI uses one Subsystem per project; the stream subsystem uses one Subsystem per session managed by the client (`stream:session:<id>`). |
| IPC timeout | Not set on the protocol itself | Runtime-side I/O (subprocesses via `exec.CommandContext`, `worker.Pool.Stop()` bounded to 500 ms) is fully ctx-scoped, so client disconnect and daemon exit never hang. A pure event-loop deadlock still requires external restart. |
| Frame ownership of DriverState | Each `SessionFrame` holds its own `DriverState`, updated in-place by `Driver.Step` inside `Reduce` | Session outlives any frame; push-driver layers a fresh context; frame death truncates only its slice. |
| Hook event target identification | Inject a frame-scoped env var at frame-spawn time | Env vars are race-free at kernel exec level. See [state monitoring](../component/component-20260624-client-state-monitoring.md#hook-event-routing-and-race-free-identification). |
| Hook payload abstraction | `CmdEvent.Payload` as opaque `json.RawMessage` | Driver-specific fields need no state/runtime/proto changes. |
| Agent hook integration | `server event <eventType>` → `proto.CmdEvent`/`CmdHookEvent` → `EvDriverEvent` → `reduceDriverHook` → `Driver.Step(DEvHook)` | Used by hook-driven agents (Claude, Gemini). Host-side events carry `SenderID`; sandboxed events resolve the frame via bearer token. Hooks for truncated frames are dropped. |
| Structured stream integration | `codex app-server` → `proto.CmdSubsystemEvent` → `EvSubsystem` → `reduceSubsystem` → `Driver.Step(DEvSubsystem)` | Used by Codex. **Exactly one `codex app-server` runs per session managed by the client** (`stream:session:<id>`). All frames within the same Session share one app-server; different Sessions get separate processes. The app-server is launched via `agentlaunch.Dispatcher.Wrap` + `agentlaunch.Spawn` (argv-direct; no bespoke `docker exec` construction in the stream backend) and binds a per-session UDS (`codex-<sessionID>.sock`). Frames join via `BindFrame`, which registers the frame's binding (empty threadID for fresh cold-start, pre-bound for cold-start recovery) and rewrites `Plan.Command` — the daemon itself never calls `thread/start` on the app-server; the codex CLI owns thread creation and backend adopts the CLI's thread when its `thread/started` broadcast arrives (see [ADR-0081](../adr/adr-20260624-0081-codex-frame-init-serialize.md)). The daemon dials the UDS directly (host-side path resolved from the launch's bind mounts via `WrappedLaunch.HostPath`); each frame attaches over the same socket with `codex --remote unix://<sock>` (fresh) or `codex resume <persistedID> --remote unix://<sock>` (recovery). No TCP routing bridge. The stream layer emits structured tool/approval/plan/diff/message/thread-lifecycle events; `TargetID` carries the logical thread identity. When a session's last frame is released, the app-server is reaped. |
| Container egress restriction | Delegate to host (`docker network` + iptables) via `extra_create_args` | Hostname allowlists cannot be expressed by `docker create` flags alone. |
| Sandbox launcher abstraction | `runtime.AgentLauncher` wraps each `LaunchPlan`; `SandboxDispatcher` resolves direct vs devcontainer per project. The stream daemon holds a separate `runtime.Config.StreamDispatcher` backed by a non-TTY `DevcontainerLauncher` (`docker exec -i`) that shares the same `sandbox.Manager` as the interactive per-frame launcher (`-it`) | Keeps sandbox rewriting out of the reducer; one daemon mixes sandboxed and direct projects. Interactive frames vs the daemon stream consumer require different TTY settings but must share the same container lifecycle. |
| Container↔host path translation | `lib/pathmap` rewrites IPC payload paths using the frame's mounts. Per-frame bearer token and mounts are held together in a single `framereg.Registry` (one RWMutex), written atomically (`RegisterWithMounts`) by the event loop and read by the container endpoint's per-connection goroutines. | `state/`, `runtime/` (above the launcher), and `proto/` stay unaware of container layout. The registry's RWMutex is the **one sanctioned lock** in the runtime root: container hook handlers read off-loop, so token/mounts cannot be plain loop-owned maps — and writing token+mounts under one lock closes the window where a hook could resolve a token but miss its mounts. |

## Deep dives

- [Process model](../component/component-20260624-client-process-model.md) — daemon process, pty frame model, rendering responsibilities
- [IPC and tool system](../component/component-20260624-client-ipc.md) — message format, command surface, concurrency model, Tool abstraction
- [State monitoring](../component/component-20260624-client-state-monitoring.md) — driver plugins, the polling pipeline, hook routing, persistence
- [Interfaces](../component/component-20260624-client-interfaces.md) — Go type definitions, data files, source tree
