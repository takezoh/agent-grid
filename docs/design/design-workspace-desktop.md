---
id: design-workspace-desktop
kind: design
title: Workspace desktop app (Electron host for the shared SPA)
status: active
created: '2026-07-25'
scope_type: component
responsibilities:
- id: RESP-001
  statement: Host the shared SPA (clients/ui bundle) in a standalone Electron app
    that users can launch directly or that the shell addresses per session.
- id: RESP-002
  statement: Own window registry semantics — atomic focus-or-create per
    (server, session) so concurrent openSession requests converge on one window.
- id: RESP-003
  statement: Expose the named-pipe control endpoint (openSession/activate envelope)
    that the shell and CLI address; the pipe name is a per-user singleton.
- id: RESP-004
  statement: Inject hosted-mode context (bearer token, session preselect) via the
    preload contextBridge; tokens never appear in URLs.
- id: RESP-005
  statement: Manage desktop lifecycle — background start, idle-quit after the
    configured window-less period, and default window geometry from workspace
    config.
- id: RESP-006
  statement: Own desktop display selection UX, persist the display containing the
    most recently moved Workspace window, restore new windows to that display, and
    write the physical display anchor consumed by Windows Shell.
invariants:
- id: INV-001
  statement: The SPA bundle is copied verbatim from clients/ui dist; the workspace
    does not re-implement wire decoding, session presentation, or auth resolution.
  enforcement: review
- id: INV-002
  statement: The agent-grid-workspace named pipe is a per-user singleton; a second
    workspace instance must not race the pipe, and clients may resolve the server
    process from the connected pipe handle (GetNamedPipeServerProcessId).
  enforcement: contract
- id: INV-003
  statement: Hosted-mode context is additive — the SPA's browser-mode URL contract,
    CSP, and WS-origin behavior stay unchanged (FR-HOSTED-SPA-COMPAT).
  enforcement: test
- id: INV-004
  statement: The workspace does not attempt to seize foreground on its own; it
    focuses in response to control messages under the shell's delegated rights
    (see design-windows-shell cap:workspace-activation-delegation).
  enforcement: review
- id: INV-005
  statement: Cross-process display selection uses a physical screen-coordinate
    anchor; Electron Display.id is Workspace-internal and is never interpreted as
    a Windows App SDK DisplayId.
  enforcement: test
boundaries:
  provides:
  - Per-session desktop windows over the shared SPA with hosted token injection.
  - The control endpoint consumed by the shell's WorkspaceLauncher and the CLI.
  consumes:
  - clients/ui build output (copy-ui) and the desktop configuration contract
    (design-windows-shell owns the schema).
  - Gateway REST/WS indirectly through the embedded SPA.
  forbidden:
  - Forking SPA behavior per mode beyond the injected hosted context.
  - Persisting or logging bearer tokens; passing tokens via URLs.
variability:
  fixed:
  - Focus-or-create window semantics and the pipe envelope contract.
  - Preload-injected hosted context as the only mode signal.
  free:
  - Window chrome, default geometry, idle-quit timing (config-driven).
  - Packaging/installation layout (build-windows / install-local scripts).
capabilities:
- id: cap:session-window-authority
  uniqueness: global
failure_responsibilities:
- Pipe endpoint failures must leave process spawn as the callers' fallback path;
  a wedged pipe must not brick session opening.
trust_boundaries:
- The control pipe accepts local per-user input only; envelopes are validated
  before acting.
compatibility_policies:
- Control envelope changes are versioned additively; the shell and CLI are the
  paired consumers.
tags:
- native-clients
- electron
owners: []
relations: []
source_paths:
- clients/workspace/
summary: 'Governs the standalone Electron workspace: window registry with focus-or-create
  semantics, the named-pipe control endpoint, hosted-mode token injection, and idle
  lifecycle.'
---

## Purpose

The workspace app is the desktop deep-work surface: where the shell's glance
ends, a workspace window opens the full session UI (terminal, transcript,
review) using the exact same SPA that browsers get, plus injected hosted
context. It exists so desktop users never need a browser
(goal-hosted-mode-no-browser) while the SPA remains single-sourced.

## Conformance

- Window registry and control endpoint behavior are covered by workspace unit
  tests; SPA compatibility is pinned by the browser Playwright suite staying
  green unmodified.
- The shell side of the pipe contract (exception semantics, PID discovery,
  foreground delegation) is tested under design-windows-shell.

## Related Decisions

- Hosted chrome is not suppressed (sidebar decision 2026-07-25) — see
  design-clients-ui.
