---
change: change-20260723-windows-shell-phase2
role: requirements
id: req-20260723-windows-shell-phase2
kind: requirements
title: 20260723 Windows Shell Phase 2 Requirements
status: draft
created: '2026-07-23'
summary: EARS functional requirements + NFRs + acceptance scenarios for Windows Shell
  (WinUI3) + Workspace (Electron) + WSL daemon supervision (Phase 2 S1-S5). Materialized
  from plan.json by design integrator; canonical FR/NFR content maintained here for
  downstream implementation.
change_id: change-20260723-windows-shell-phase2
functional_requirements:
- id: FR-B1-01
  type: event_driven
  priority: must
  statement: WHEN Shell's WorkspaceLauncher sends an {op:"openSession", id} JSON Lines
    message over the \\.\pipe\agent-grid-workspace control channel, the Workspace
    control-endpoint SHALL create or focus exactly one BrowserWindow for that session
    id, never increasing the window count for an already-open session.
- id: FR-B1-02
  type: ubiquitous
  priority: must
  statement: Every control envelope crossing boundary 1 SHALL validate against the
    closed {op, id}-only JSON Schema declared by contract-b1-jsonlines-envelope-shape's
    current schema version; any additional field (including but not limited to session
    state summaries, health status, or notification metadata) SHALL be rejected by
    the receiver, never silently accepted.
- id: FR-B1-03
  type: event_driven
  priority: must
  statement: WHEN WorkspaceLauncher's pipe connection attempt finds no listening Workspace
    process, Shell SHALL spawn the Workspace executable and retry the pipe connection
    with bounded backoff before surfacing a launch failure to the user.
- id: FR-B2-01
  type: event_driven
  priority: must
  statement: WHEN Shell or Workspace needs the gateway bearer token, it SHALL read
    it fresh from the UNC path \\wsl$\<distro>\home\<user>\.agent-grid\gateway-token
    at each daemon connection attempt, never from a Windows-side cached copy.
- id: FR-B2-02
  type: event_driven
  priority: must
  statement: WHEN Shell or Workspace opens a WebSocket subscription to the daemon,
    it SHALL mint a single-use ticket via POST /api/ws-ticket using the bearer token
    over REST and then connect GET /ws?ticket=<t> within the existing ticketTTL, without
    requiring a gateway-side change to WebSocket header authentication.
- id: FR-B2-03
  type: unwanted
  priority: must
  statement: IF the UNC token path is unreadable OR the daemon rejects the bearer
    token, THEN Shell SHALL NOT fall back to an unauthenticated connection or a fabricated
    Connected state; it SHALL surface an explicit failure state on the supervision
    surface.
- id: FR-B2-04
  type: state_driven
  priority: must
  statement: WHILE the Workspace hosted-mode BrowserWindow loads http://127.0.0.1:<web-port>/?hosted=1&session=<id>,
    daemon-config.ts SHALL inject the resolved token via preload contextBridge (window.hostedModeInfo),
    never in the URL query string or any script-visible request artifact.
- id: FR-B3-01
  type: state_driven
  priority: must
  statement: WHILE DaemonSupervisor is in any state other than Healthy or Adopted,
    the supervision surface SHALL show a non-Connected health indicator, and DaemonSupervisor
    SHALL NOT report Connected until an authenticated /api/sessions probe against
    the configured port succeeds.
- id: FR-B3-02
  type: event_driven
  priority: must
  statement: WHEN Shell starts, DaemonSupervisor SHALL attempt adopt (health probe
    against the configured port) before attempting spawn, and SHALL spawn wsl.exe
    -d <distro> -- <path>/server ... only if adopt fails.
- id: FR-B3-03
  type: event_driven
  priority: must
  statement: WHEN a spawned daemon process exits immediately after spawn, DaemonSupervisor
    SHALL retry adopt at most once before transitioning to an explicit Degraded/failed
    state, and SHALL never spawn a second concurrent daemon instance.
- id: FR-B3-04
  type: event_driven
  priority: must
  statement: WHEN the user selects "Restart daemon", DaemonSupervisor SHALL issue
    graceful shutdown, wait for session persistence, re-spawn, and drive WS resubscribe
    across the Shell panel and every open Workspace window such that the same session
    set observed before restart is again observable within threshold-daemon-restart-reconnect-delay,
    with no manual reconnect action required.
- id: FR-B3-05
  type: unwanted
  priority: must
  statement: The tray Quit menu handler SHALL NEVER invoke DaemonSupervisor's Stop/shutdown
    API as a side effect of the user selecting Quit; daemon termination SHALL be reachable
    only through the structurally distinct "Stop daemon" menu handler.
- id: FR-JB-01
  type: event_driven
  priority: must
  statement: WHEN [Jump back] is pressed and the cached HWND for the session's target
    window is present and identity-verified, Shell.Platform SHALL call SetForegroundWindow
    on that HWND and SHALL NOT activate the supervision surface itself.
- id: FR-JB-02
  type: event_driven
  priority: must
  statement: WHEN the cached HWND is absent or fails identity verification, Shell.Platform
    SHALL attempt process-name + title matching before declaring resolution failure.
- id: FR-JB-03
  type: unwanted
  priority: must
  statement: IF both HWND lookup and process+title matching fail, THEN Shell.Platform
    SHALL display an explicit "target not found" message and SHALL NEVER activate
    an arbitrary fallback window.
- id: FR-EF-01
  type: event_driven
  priority: must
  statement: WHEN engage mode is entered, Shell.Platform SHALL record the current
    foreground HWND via GetForegroundWindow before removing WS_EX_NOACTIVATE; WHEN
    engage is confirmed or cancelled, Shell.Platform SHALL attempt to restore foreground
    to the recorded HWND.
- id: FR-EF-02
  type: unwanted
  priority: must
  statement: IF the recorded pre-engage HWND has been destroyed or its owning process
    has exited by confirm/cancel time, THEN Shell SHALL NOT force its own window to
    foreground as a substitute target.
- id: FR-FOCUS-INV
  type: ubiquitous
  priority: must
  statement: The supervision surface SHALL NEVER assign OS foreground focus to a window
    other than the staged-best-effort jump-back target (per FR-JB-01/FR-JB-02) or
    the recorded engage-return target (per FR-EF-01) as the result of an automated
    recovery decision. Any future affordance that would introduce a third automated
    focus-transfer path (e.g. panel self-activation on toast dismiss) violates this
    invariant.
- id: FR-MIG-01
  type: unwanted
  priority: must
  statement: The Workspace window-registry SHALL never create a second BrowserWindow
    for a session id that already has an open window; AND WHEN two or more openSession
    requests for the same not-yet-open session id arrive concurrently (double-click,
    racing deep link, racing toast-escalation), the registry SHALL still converge
    to exactly one BrowserWindow for that session id — no implementation may perform
    BrowserWindow creation and registry insertion as two non-atomic steps that let
    a second concurrent request pass the not-yet-open check before the first request
    registers.
- id: FR-MIG-02
  type: unwanted
  priority: must
  statement: Opening a Workspace window SHALL never invoke the OS default browser
    process, and Chromium default accelerators (Ctrl+T, Ctrl+L, and equivalents) SHALL
    be inert inside the hosted BrowserWindow.
- id: FR-MIG-03
  type: unwanted
  priority: must
  statement: IF a Workspace window is closed by the user, THEN window-registry SHALL
    NOT issue any session-stop/session-end request to the daemon; the corresponding
    session's running/waiting state SHALL be unaffected by the window close.
- id: FR-TOAST-01
  type: state_driven
  priority: must
  statement: WHILE the panel is unwatched (flyout closed OR another app foreground
    OR DND active OR workstation locked, with panel-open taking precedence when it
    conflicts with DND/lock and query-unavailable defaulting to unwatched), an approval
    or question event SHALL additionally trigger an AppNotification toast with [Approve]/[Deny]
    or an inline textbox; WHILE the panel is watched, Shell SHALL NOT publish a supplementary
    toast for the same event.
- id: FR-TOAST-02
  type: ubiquitous
  priority: must
  statement: The daemon-health-driven tray-icon-appearance update path and the supervision-event-driven
    toast path SHALL be structurally distinct code paths; DaemonSupervisor's package
    SHALL have no import or call edge into ToastNotifier's package, so a health transition
    (Healthy/Spawning/Degraded) can never invoke the toast-issuing component.
- id: FR-APPROVE-ROLLBACK
  type: unwanted
  priority: must
  statement: IF an approve/deny submission fails (network error or gateway API error
    other than resolved-by-other), THEN SupervisionState SHALL restore the item to
    the pending queue OR surface an explicit error, AND SHALL NOT report the agent
    as unblocked until a server-confirmed resolution arrives.
- id: FR-APPROVE-RESOLVED-BY-OTHER
  type: event_driven
  priority: must
  statement: WHEN a queued item's server response indicates resolved-by-other (per
    adr-20260724-approval-single-writer-first-commit), Shell SHALL render an explicit
    already-handled state and SHALL NOT submit a duplicate decision nor treat the
    local pending flag as authoritative.
- id: FR-WS-STATE-SCHEMA
  type: ubiquitous
  priority: should
  statement: Every persisted workspace-state.json record SHALL carry a schema_version
    field; a reader encountering an unknown newer schema_version SHALL fall back to
    the empty-map behavior already specified for missing/corrupt files, and SHALL
    NEVER silently corrupt or discard live state across a version boundary.
- id: FR-HOSTED-SPA-COMPAT
  type: ubiquitous
  priority: should
  statement: The clients/ui hosted-mode flag SHALL be additive (param or preload-injected)
    and SHALL NOT change the existing SPA's browser-mode URL contract, CSP, or WS-origin-check
    behavior; the existing browser-mode Playwright suite SHALL continue passing unmodified.
non_functional_requirements:
- id: NFR-engage-expand-latency
  type: performance
  criteria: Panel engage expansion completes within ~150ms (provisional; re-derive
    after S1 measurement).
  measurement: S1 histogram capture on target hardware; regression bar in Playwright-for-Electron/Composition
    timing test
- id: NFR-panel-animation-framerate
  type: performance
  criteria: Panel expand animation targets 60fps (provisional).
  measurement: S1 Composition framerate probe on target hardware
- id: NFR-daemon-health-observation-delay
  type: performance
  criteria: Health indicator reaches Connected within ~5s of a successful probe (provisional).
  measurement: S1 timing capture in DaemonSupervisor state-machine harness
- id: NFR-toast-response-latency
  type: performance
  criteria: Toast-to-confirmed-response completes within ~5s (provisional; S3 histogram
    pending).
  measurement: S3 unpackaged-COM prototype histogram
- id: NFR-com-reactivation-latency
  type: performance
  criteria: Toast button press → COM reactivation → Shell-side processing completes
    within ~1s (unmeasured under unpackaged COM; S3 prototype required).
  measurement: S3 assumption-com-background-activation-unpackaged prototype instrumentation
- id: NFR-daemon-restart-reconnect-delay
  type: performance
  criteria: 'Daemon Restart → all supervision surfaces Connected + same session set
    observable within 5s (provenance: user consultation 2026-07-24; integrates with
    adr-20260624-0025 backoff defaults — full-jitter exponential capped at 30s per
    attempt, converges under 5s for the 3-session normal case).'
  measurement: S1 reconnect-backoff histogram against the fake WSL daemon fidelity
    harness; regression alarm at p95 > 5s
- id: NFR-daemon-healthy-toast-budget-window
  type: reliability
  criteria: 'Zero non-supervision toasts within a 5min observation window while Healthy
    (provenance: user consultation 2026-07-24 confirmed 5min as the operating value;
    S3 operational logs may propose adjustment if a signal warrants).'
  measurement: S3-S5 operational log query against Windows notification history
- id: NFR-boundary-1-envelope-strict
  type: security
  criteria: Any control envelope with a field not defined in the current {op,id} schema
    version is rejected by the receiver rather than silently forwarded to window-registry.
  measurement: vitest / xUnit strict-schema fuzz cases sending unknown-field envelopes
acceptance:
- id: AC-UAC-001
  given: Existing WSL-hosted server running with a matching token
  when: Shell starts
  then: Health indicator reaches Connected within NFR-daemon-health-observation-delay
    without an intermediate spawning state; no dual-run flicker observed.
  requirement_refs:
  - FR-B3-01
  - FR-B3-02
- id: AC-UAC-002b
  given: Daemon absent; spawned process crashes immediately
  when: Health probe times out post-spawn
  then: State transitions to an explicit, actionable Degraded/failed indicator; no
    infinite spawning spinner; no second concurrent spawn.
  requirement_refs:
  - FR-B3-03
- id: AC-UAC-003
  given: Approval waiting count is 0; VS Code has foreground
  when: One approval event arrives on the WS stream
  then: Top-bar count increments non-activatingly; VS Code keystrokes continue uninterrupted.
  requirement_refs:
  - FR-B3-01
- id: AC-UAC-005
  given: Approval waiting count is 1; flyout open; VS Code foreground
  when: User activates [Approve] via pointer or via Tab-reachable keyboard path
  then: Queue count decrements; VS Code retains foreground; agent execution resumes.
  requirement_refs:
  - FR-B2-02
- id: AC-UAC-006
  given: Approval waiting count is 1; user clicked [Approve]; API call fails with
    network error
  when: Failure response arrives
  then: Queue item reappears OR an explicit error surfaces; agent remains reported
    as unblocked-pending (never resolved-looking).
  requirement_refs:
  - FR-APPROVE-ROLLBACK
- id: AC-UAC-006r
  given: Another client already resolved the item (server returns resolved-by-other)
  when: This client submits [Approve]
  then: An explicit already-handled state is rendered; no duplicate decision is sent.
  requirement_refs:
  - FR-APPROVE-RESOLVED-BY-OTHER
- id: AC-UAC-007
  given: VS Code foreground at engage entry; an unrelated window briefly grabs foreground
    mid-engage
  when: User confirms the answer
  then: VS Code foreground is explicitly restored.
  requirement_refs:
  - FR-EF-01
- id: AC-UAC-008
  given: The engage-entry window closes before confirm
  when: User confirms/cancels
  then: No SetForegroundWindow call is made against Shell itself or a substitute window;
    either no focus change occurs or an explicit alternative affordance is shown.
  requirement_refs:
  - FR-EF-02
  - FR-FOCUS-INV
- id: AC-UAC-009
  given: Target Windows Terminal HWND is cached and IsWindow true
  when: User activates [Jump back]
  then: Target window becomes foreground; supervision surface does not remain top-most.
  requirement_refs:
  - FR-JB-01
- id: AC-UAC-010
  given: HWND stale AND process+title match yields no candidate
  when: User activates [Jump back]
  then: Explicit 'target not found' message appears; no window is activated as a fallback.
  requirement_refs:
  - FR-JB-02
  - FR-JB-03
  - FR-FOCUS-INV
- id: AC-UAC-011
  given: Session X has an open Workspace window
  when: User activates [Open] for X (or a deep link arrives for X)
  then: The existing BrowserWindow is focused; window count stays at 1.
  requirement_refs:
  - FR-B1-01
  - FR-MIG-01
- id: AC-UAC-012
  given: Fresh hosted-mode window
  when: Any [Open] path completes
  then: No default browser process starts; address/tab bars absent; Ctrl+T and Ctrl+L
    are inert.
  requirement_refs:
  - FR-MIG-02
- id: AC-UAC-013
  given: PC unattended (locked or DND) when an approval arrives
  when: User returns and checks Action Center
  then: The AppNotification toast is still present and its [Approve] resolves the
    panel queue in the same act.
  requirement_refs:
  - FR-TOAST-01
- id: AC-UAC-014
  given: Panel flyout open, no DND, unlocked
  when: An approval event arrives
  then: No toast is issued; the panel queue updates alone.
  requirement_refs:
  - FR-TOAST-01
- id: AC-UAC-015
  given: Three sessions running
  when: User selects Restart daemon
  then: All supervision surfaces re-reach Connected within NFR-daemon-restart-reconnect-delay;
    the same three sessions are visible on Shell and on every open Workspace window;
    no manual reconnect click.
  requirement_refs:
  - FR-B3-04
- id: AC-UAC-111
  given: User closes a Workspace window mid-session
  when: Close handler runs
  then: No stop/end API call is issued; session status remains running/waiting; on
    re-open the window is restored without growing window count.
  requirement_refs:
  - FR-MIG-03
  - FR-B1-01
- id: AC-UAC-113
  given: Daemon stays Healthy for NFR-daemon-healthy-toast-budget-window with zero
    approval/question events
  when: Observation window elapses
  then: Zero non-supervision Windows toasts appear; tray icon appearance alone reflects
    Healthy.
  requirement_refs:
  - FR-TOAST-02
---

## Overview

Phase 2 EARS requirements landed by the design integrator from `plan.json`.
The canonical shape lives in `functional_requirements`, `non_functional_requirements`, and
`acceptance` above (frontmatter). Prose below is a reader-oriented navigation aid; it does not
override the frontmatter.

## Functional requirement clusters

- **Boundary 1 (Shell↔Workspace named pipe)**: `FR-B1-01`, `FR-B1-02` (closed-schema invariant),
  `FR-B1-03`.
- **Boundary 2 (Shell/Workspace↔WSL daemon REST/WS)**: `FR-B2-01`, `FR-B2-02`, `FR-B2-03`,
  `FR-B2-04` (hosted-mode contextBridge token injection).
- **Boundary 3 (WSL daemon lifecycle)**: `FR-B3-01`..`FR-B3-05`.
- **Jump-back (F-005)**: `FR-JB-01`..`FR-JB-03`.
- **Engage focus return (F-004)**: `FR-EF-01`, `FR-EF-02`.
- **Cross-flow focus invariant**: `FR-FOCUS-INV` (ubiquitous, lifts per-flow prohibitions to a
  surface-wide rule per critique graft #3).
- **Migration / Workspace window discipline**: `FR-MIG-01`..`FR-MIG-03`.
- **Toast (F-007/F-108)**: `FR-TOAST-01`, `FR-TOAST-02` (structural separation of daemon-health
  from toast path).
- **Approval submission semantics** (critique blocker + major): `FR-APPROVE-ROLLBACK`,
  `FR-APPROVE-RESOLVED-BY-OTHER`.
- **Persistence & compat** (critique minors): `FR-WS-STATE-SCHEMA`, `FR-HOSTED-SPA-COMPAT`.

## Non-functional requirements

Provisional performance / reliability bounds proposed here; each has a `measurement` field
naming the S1-S5 milestone that will re-confirm the concrete value. The intentionally-null
`NFR-daemon-restart-reconnect-delay` is gated on ADR-0025-extension + an S1 measurement.

## Acceptance scenarios

Each `AC-UAC-*` traces to one or more `FR-*` and mirrors the UX Given/When/Then in `ux.md`.
The counterexample structure is preserved: any implementation that satisfies the FR while
failing the acceptance scenario is a defect.

