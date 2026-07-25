---
id: design-windows-shell
kind: design
title: Windows Shell (WinUI notch supervision surface)
status: active
created: '2026-07-25'
scope_type: component
responsibilities:
- id: RESP-001
  statement: Own the always-on-top notch (dynamic island) supervision surface that
    morphs between a compact aggregate bar and an expanded approval/question panel.
- id: RESP-002
  statement: Consume the gateway supervision surface (REST ticket mint + lifecycle
    WS) per configured server and reduce frames into one merged supervision snapshot.
- id: RESP-003
  statement: Supervise per-server daemon availability (probe, connect-only or WSL
    spawn) and surface degraded state through tray appearance.
- id: RESP-004
  statement: Launch or address the workspace app for a session (named-pipe first,
    process spawn fallback) and delegate foreground rights so the workspace can
    activate.
- id: RESP-005
  statement: Own the desktop configuration contract (servers/appearance/shell/workspace
    documents under the config directory) consumed by both shell and workspace.
- id: RESP-006
  statement: Route agent-grid:// deep links to panel focus, session open, or jump-back.
invariants:
- id: INV-001
  statement: Layering is one-way — Shell.Core (pure, net8.0) references neither
    Shell.Platform (win32 interop) nor the WinUI host; composition wires them.
  enforcement: conformance
- id: INV-002
  statement: Supervision and daemon snapshot events may fire on any thread; UI
    subscribers marshal via their captured DispatcherQueue, and producers invoke
    subscribers individually with per-subscriber exception isolation so one failing
    subscriber can neither block delivery nor unwind producer state machines or
    the startup sequence.
  enforcement: test
- id: INV-003
  statement: The lifecycle WS parser honors the hello/delta contract (k:"h" carries
    the one-time full session snapshot on attach; k:"v" carries subsequent deltas)
    and must not silently drop known frame kinds.
  enforcement: test
- id: INV-004
  statement: Island mode transitions (compact/expanded, auto-expand on new pending,
    auto-collapse only for auto-opened panels) are owned by a pure state machine;
    the window layer only applies its output.
  enforcement: test
- id: INV-005
  statement: AutomationIds are a published UIA contract (never renamed); collapsed
    elements leave the UIA tree, so always-visible and state-dependent ids are
    asserted separately (see note-20260724-windows-shell-ui-e2e-authoring).
  enforcement: test
- id: INV-006
  statement: Workspace addressing treats a pipe connect timeout (surfaced as
    OperationCanceledException from the linked cancel token) as "workspace absent
    → spawn", while caller-initiated cancellation propagates without spawning;
    the exception contract is explicit at the pipe client and retry loop.
  enforcement: test
- id: INV-007
  statement: Cross-process activation of the workspace is the shell's duty — it
    holds the user's foreground rights at interaction time and delegates them
    (AllowSetForegroundWindow with the pipe server PID) before sending openSession.
  enforcement: test
- id: INV-008
  statement: The panel never steals focus (WS_EX_NOACTIVATE); only explicit engage
    entry takes focus, and exit restores the prior foreground window.
  enforcement: review
- id: INV-009
  statement: Desktop configuration files are the single source of connection truth;
    environment variables do not override servers.json.
  enforcement: review
boundaries:
  provides:
  - Notch/tray/toast supervision UX (approvals, questions, session chips, engage
    replies) for every configured server.
  - Workspace launching and activation on behalf of session-open intents (chips,
    open button, deep links).
  - The desktop configuration schema contract shared with the workspace app.
  consumes:
  - Gateway REST (ticket mint, submissions) and lifecycle WS frames per server.
  - The workspace named-pipe control endpoint and executable install location.
  - Win32 window/foreground primitives via Shell.Platform interop adapters.
  forbidden:
  - Shell.Core referencing Shell.Platform or WinUI types.
  - Re-implementing gateway wire decoding outside WsFrameParser.
  - Suppressing supervision events or swallowing subscriber exceptions at the
    producer without isolation.
variability:
  fixed:
  - Hello/delta lifecycle WS contract and discriminated k-frame union.
  - One merged supervision snapshot across servers with per-server ServerId tagging.
  - Foreground delegation flowing from shell interaction to workspace activation.
  free:
  - Visual styling of the island (sizes, radii, colors, morph animation).
  - Daemon launch modes per server (connect_only vs WSL spawn).
  - Additional supervision frame kinds, provided the parser handles them explicitly.
capabilities:
- id: cap:notch-supervision-surface
  uniqueness: global
- id: cap:workspace-activation-delegation
  uniqueness: global
failure_responsibilities:
- Connection failures degrade to offline indication and full-jitter reconnect;
  they never crash the panel or stop other servers' surfaces.
- Daemon probe/spawn failures surface as tray degradation, never as blocking UI.
trust_boundaries:
- Gateway endpoints are configured, possibly non-loopback servers; tokens come
  from per-server token files and are never embedded in URLs.
compatibility_policies:
- Published AutomationIds and the desktop config schema evolve additively; renames
  require a migration path.
source_paths:
- clients/windows-shell/
- clients/desktop-config/
tags:
- native-clients
- windows
owners: []
relations: []
summary: 'Governs the Windows notch shell: dynamic-island supervision UX, daemon supervision,
  gateway supervision surface consumption, workspace launch/foreground delegation,
  and the desktop configuration contract.'
---

## Purpose

The Windows Shell is the native glance-and-act surface for agent supervision: a
notch-style always-on-top panel (Vibe Island model) that stays compact while
agents work, auto-expands when an approval or question needs the user, and jumps
into the workspace app for deep work. It is the desktop's composition point for
multiple configured servers.

## Conformance

- Pure logic (island state machine, wire parsing, presentation mapping, launch
  retry/exception contracts) lives in Shell.Core with cross-platform unit tests.
- UIA e2e (FlaUI, T3 opt-in) drives the built exe against a run-dev gateway; the
  AutomationId contract and side-effect discipline are normed in
  note-20260724-windows-shell-ui-e2e-authoring.
- Known gaps tracked in issues/: cross-client wire parity fixtures, named-pipe
  exception-type fidelity test, headless deep-link foreground authority,
  UnhandledException observability.

## Related Decisions

- Hello/delta coexistence on the lifecycle WS: adr-20260724 approval-broadcast
  family (server side).
- Sidebar/hosted UX split with the workspace: see design-clients-ui.
