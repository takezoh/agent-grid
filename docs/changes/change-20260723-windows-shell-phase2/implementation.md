---
change: change-20260723-windows-shell-phase2
role: implementation
id: impl-20260723-windows-shell-phase2
kind: implementation
title: 20260723 Windows Shell Phase 2 Implementation
status: draft
created: '2026-07-23'
summary: Implementation contracts + component decomposition + S1-S5 milestones + ADRs
  for the Phase 2 Windows Shell + Workspace + WSL daemon supervision surface. Materialized
  from plan.json by design integrator; canonical shape kept here.
change_id: change-20260723-windows-shell-phase2
components:
- id: component-shell-core-gatewayclient
  name: Shell.Core.GatewayClient
  responsibility: 'Boundary-2 adapter on the Shell side: owns the generated C# SDK
    wrapper (clients/sdk/csharp), UNC-path bearer-token acquisition, REST ws-ticket
    mint, WS connect/reconnect with backoff + REST backfill resume, and the health
    probe DaemonSupervisor drives.'
  depends_on: []
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/
    owner: windows-shell Shell.Core (native-clients Phase 2); first consumer of clients/sdk/csharp
      per clients/sdk/README.md
    integration_points:
    - clients/sdk/csharp (generated OpenAPI client)
    - src/server/api/mux.go POST /api/ws-ticket
    - src/server/api/mux.go GET /ws?ticket=<t>
    - src/server/api/auth.go TokenAuth
    - src/server/api/ticket.go ticketStore
    test_seams:
    - fake gateway HTTP/WS server substituted via injectable HttpClient/WebSocket
      factory (xUnit)
    - FakeVsReal fidelity backstop against the real server binary's ticket-mint→/ws
      flow (opt-in T3)
    rationale: plan-20260723-windows-shell-design.md §2/§3.1 names GatewayClient as
      the generated-client owner; protocol/README.md and clients/sdk/README.md name
      windows-shell GatewayClient as the first C# SDK consumer.
    accepted_adr_refs:
    - adr-20260724-protocol-cross-language-sdks-supersedes-0021
    - adr-20260724-sdk-codegen-openapi-generator
    - adr-20260724-capability-negotiation-bundled-remote-two-axis
    - adr-20260724-approval-answerer-identity-per-ws-instance
    - adr-20260624-0025-transcript-rest-backfill-then-ws-tail
    - adr-20260624-0012-daemon-client-eager-dial-supervisor
  contract_refs:
  - contract-b2-token-acquisition
  - contract-b3-restart-continuity
  decision_closure_reason: ''
- id: component-shell-core-daemonsupervisor
  name: Shell.Core.DaemonSupervisor
  responsibility: 'Boundary-3 adapter: pure NotRunning→Spawning→Healthy↔Degraded→Swapping/Adopted
    state machine plus thin Runner shell that spawns/adopts the WSL-hosted daemon.
    Sole authoring source for daemon-health state; drives tray-icon appearance directly
    and MUST NOT depend on ToastNotifier package.'
  depends_on:
  - component-shell-core-gatewayclient
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Core/DaemonSupervisor/
    owner: windows-shell Shell.Core (native-clients Phase 2)
    integration_points:
    - wsl.exe process spawn via injectable Runner interface (mirrors platform/agentlaunch
      Dispatcher seam)
    - component-shell-core-gatewayclient health probe (/api/sessions)
    - WS resubscribe fan-out to Shell panel and Workspace windows on restart
    - Shell tray icon appearance (H.NotifyIcon.WinUI)
    test_seams:
    - xUnit pure state-machine test feeding (state, event) pairs
    - fake Runner interface substituting wsl.exe (mirrors src/server/api/testsupport/fakeagents
      convention)
    - depguard-style dependency-direction lint asserting no import edge from DaemonSupervisor
      package to ToastNotifier package
    - fidelity backstop against a real WSL-hosted server binary for spawn/adopt/restart
      (opt-in T3)
    rationale: plan-20260723-windows-shell-design.md §3.6 fixes state names and adopt-before-spawn
      / graceful-restart contract; ADR-0025 and adr-20260716-restart-continuity-compatibility-axes
      bound 'session survives restart' server-side; DaemonSupervisor is the SSOT for
      daemon-health per critique-driven owner correction.
    accepted_adr_refs:
    - adr-20260624-0012-daemon-client-eager-dial-supervisor
    - adr-20260624-0025-transcript-rest-backfill-then-ws-tail
    - adr-20260716-restart-continuity-compatibility-axes
  contract_refs:
  - contract-b3-daemon-supervisor-state-machine
  - contract-b3-wsl-detach-mechanism
  - contract-daemon-health-toast-budget
  - contract-health-toast-structural-separation
  decision_closure_reason: ''
- id: component-shell-core-supervisionstate
  name: Shell.Core.SupervisionState
  responsibility: Pure reducer over the boundary-2 WS event stream (EvtApproval*/EvtQuestion*/viewUpdate)
    producing panel/tray glance view state. Owns optimistic-submission rollback on
    approve/deny failure and renders authoritative resolved-by-other outcomes without
    local dedupe.
  depends_on:
  - component-shell-core-gatewayclient
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Core/SupervisionState/
    owner: windows-shell Shell.Core (native-clients Phase 2)
    integration_points:
    - component-shell-core-gatewayclient WS event stream
    - adr-20260724-approval-broadcast-coexists-view-update frame shapes (EvtApproval*/EvtQuestion*
      alongside viewUpdate)
    - adr-20260724-approval-single-writer-first-commit resolved-by-other response
      kind
    test_seams:
    - xUnit pure Reduce(state, event) → state' test (mirrors host/state FC/IS pattern)
    - xUnit table-driven test for optimistic-removal rollback on network error
    - xUnit table-driven test for resolved-by-other rendering without duplicate submission
    rationale: plan §3.1 fixes SupervisionState as pure input→output module fed by
      single receive loop, matching ARCHITECTURE.md's FC/IS principle applied to the
      C# layer; critique demands rollback and resolved-by-other be explicit contracts,
      not prose.
    accepted_adr_refs:
    - adr-20260724-approval-question-state-domain-in-host-state
    - adr-20260724-approval-broadcast-coexists-view-update
    - adr-20260724-approval-expiry-deny-default-no-extension
    - adr-20260724-approval-single-writer-first-commit
    - adr-20260724-approval-lifecycle-teardown-cancel
  contract_refs:
  - contract-approve-submission-rollback
  - contract-resolved-by-other-display
  decision_closure_reason: ''
- id: component-shell-core-deeplinkrouter
  name: Shell.Core.DeepLinkRouter
  responsibility: Pure agent-grid:// URI → routing-decision function (panel-focus-item
    vs WorkspaceLauncher hand-off vs jump-back). Sole registered protocol handler;
    owns the schema-gap contract for the question/jump kinds absent from the currently
    accepted protocol/deep-links.schema.json.
  depends_on:
  - component-shell-core-workspacelauncher
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Core/DeepLinkRouter/
    owner: windows-shell Shell.Core (native-clients Phase 2)
    integration_points:
    - protocol/deep-links.schema.json (currently kind enum = [session, approval] only)
    - clients/sdk/csharp generated deep-link parse/construct helper
    - clients/sdk/ts/src/deepLinks.ts (already populated per Phase 1)
    - Windows registry protocol-handler self-registration (unpackaged)
    test_seams:
    - xUnit pure URI → route test, table-driven over the accepted session/approval
      kinds
    - xUnit test asserting no client-invented kind string is emitted or accepted absent
      an additive schema extension
    rationale: protocol/deep-links.schema.json (existing) fixes the URI shape; the
      router only maps that fixed shape to a routing decision. Plan §3.4 assumes question
      and /jump variants that today's schema does not define — this component owns
      the resulting closure gap.
    accepted_adr_refs:
    - adr-20260724-deep-link-shape-adopts-remote-control-plan
  contract_refs:
  - contract-deep-link-question-jump-kind-gap
  decision_closure_reason: ''
- id: component-shell-core-workspacelauncher
  name: Shell.Core.WorkspaceLauncher
  responsibility: 'Boundary-1 adapter on the Shell side: named pipe client that sends
    {op,id} control envelopes to Workspace, spawns the Workspace executable on first
    use, and retries the pipe connection with bounded backoff.'
  depends_on: []
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Core/WorkspaceLauncher/
    owner: windows-shell Shell.Core (native-clients Phase 2)
    integration_points:
    - '\\.\pipe\agent-grid-workspace named pipe (Node net server counterpart: component-workspace-main-control-endpoint)'
    - Workspace executable spawn
    test_seams:
    - fake named-pipe server substituting Workspace main (xUnit)
    - Playwright-for-Electron fidelity backstop against the real control-endpoint.ts
    rationale: plan §4.4 fixes the pipe path, the minimal op set, and the 'no domain
      data on this pipe' invariant (gateway no-domain principle mirrored at boundary
      1).
    accepted_adr_refs: []
  contract_refs:
  - contract-b1-b2-launch-ordering
  - contract-b1-jsonlines-envelope-shape
  decision_closure_reason: ''
- id: component-shell-platform-win32
  name: AgentGrid.Shell.Platform (Win32 interop)
  responsibility: 'All Win32 interop behind an interface: WS_EX_NOACTIVATE panel styling,
    SetForegroundWindow/GetForegroundWindow/AllowSetForegroundWindow for jump-back
    and engage focus-return, AppNotification COM background activation, foreground-app/DND/lock-state
    queries for the panel-watched predicate. Owns the cross-flow focus-transfer invariant
    so no future OS integration path silently activates an arbitrary window.'
  depends_on:
  - component-shell-core-daemonsupervisor
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell.Platform/
    owner: windows-shell Shell.Platform (native-clients Phase 2)
    integration_points:
    - Win32 SetForegroundWindow / AllowSetForegroundWindow / GetForegroundWindow /
      AttachThreadInput
    - Windows App SDK AppNotificationManager (unpackaged COM background activation)
    - H.NotifyIcon.WinUI or raw Shell_NotifyIcon (tray)
    - QUERY_USER_NOTIFICATION_STATE / session-lock notification / foreground-window
      identity
    - daemon session metadata (cwd/command) surfaced via component-shell-core-gatewayclient
      for jump-back target matching
    test_seams:
    - pure jump-back/engage-restore/panel-watched logic behind IWin32InteropService,
      unit-tested with a fake implementation
    - manual T3 checklist + screenshot artifacts for actual Win32 interop (toast/COM
      activation/NOACTIVATE), per plan §7
    - static-analysis lint asserting no SetForegroundWindow call site exists outside
      JumpBackService/EngageFocusService (cross-flow invariant enforcement)
    rationale: plan §3.5/§3.2 isolates OS integration in this layer so 'OS statement
      does not change server semantics' and the contract layer never sees HWNDs or
      Win32 concepts; cross-flow focus invariant lifts UAC-008/010 to a ubiquitous
      surface rule.
    accepted_adr_refs: []
  contract_refs:
  - contract-cross-flow-focus-invariant
  - contract-engage-focus-return-mechanism
  - contract-jump-back-staged-resolution
  - contract-toast-panel-watched-detection
  decision_closure_reason: ''
- id: component-shell-ui
  name: AgentGrid.Shell (WinUI3 panel/tray UI)
  responsibility: 'XAML presentation of the always-visible top bar and tray flyout:
    subscribes to SupervisionState/DaemonSupervisor snapshots, renders glance content,
    hosts engage text entry, owns the tray Quit menu handler and the separate Stop-daemon
    menu handler.'
  depends_on:
  - component-shell-core-supervisionstate
  - component-shell-core-daemonsupervisor
  - component-shell-platform-win32
  repo_grounding:
    kind: planned
    paths:
    - clients/windows-shell/AgentGrid.Shell/Panel/
    - clients/windows-shell/AgentGrid.Shell/TrayIcon/
    - clients/windows-shell/AgentGrid.Shell/Menu/
    owner: windows-shell Shell UI (native-clients Phase 2)
    integration_points:
    - Composition API + Acrylic material
    - H.NotifyIcon.WinUI tray flyout
    - component-shell-core-daemonsupervisor lifecycle API (Restart-daemon; Stop-daemon
      is a distinct call site)
    test_seams:
    - xUnit test asserting Quit menu handler never invokes DaemonSupervisor.Stop (contract-quit-vs-daemon-stop)
    - manual T3 checklist for animation/material fidelity
    rationale: plan §3.2 fixes always-visible-bar + tray flyout as the UI surface;
      menu-handler split makes cc-quit-vs-daemon-stop-separation mechanically enforceable
      rather than prose-only.
    accepted_adr_refs: []
  contract_refs:
  - contract-quit-vs-daemon-stop
  decision_closure_reason: ''
- id: component-workspace-main-control-endpoint
  name: Workspace main/control-endpoint.ts
  responsibility: 'Boundary-1 adapter on the Workspace side: named pipe server accepting
    {op,id} envelopes from Shell and translating them into window-registry calls.
    Enforces the closed schema derived from contract-b1-jsonlines-envelope-shape.'
  depends_on:
  - component-workspace-main-window-registry
  repo_grounding:
    kind: planned
    paths:
    - clients/workspace/src/main/control-endpoint.ts
    owner: clients/workspace main process (native-clients Phase 2)
    integration_points:
    - \\.\pipe\agent-grid-workspace (Node net server)
    - component-workspace-main-window-registry
    test_seams:
    - vitest unit tests for envelope parsing/dispatch, including strict-schema rejection
      of any additional field
    - 'Playwright-for-Electron e2e: pipe-driven openSession focuses/creates exactly
      one window'
    rationale: plan §4.1/§4.4 fixes this file as the pipe server; Playwright-for-Electron
      is the repo's established Electron e2e tier extended for window discipline;
      the closed-schema stance closes the FR-B1-02 illustrative-enumeration gap flagged
      by critique.
    accepted_adr_refs: []
  contract_refs: []
  decision_closure_reason: control-endpoint.ts is a thin adapter that MUST validate
    against contract-b1-jsonlines-envelope-shape's schema and forward to window-registry;
    any implementation that satisfies that contract on the receive side yields the
    same observable behavior (accept vs reject) regardless of parser library or dispatch
    table shape.
- id: component-workspace-main-window-registry
  name: Workspace main/window-registry.ts
  responsibility: 'Sole BrowserWindow creation point (lint-enforced): session→window
    map, atomic focus-or-create semantics (converges to exactly one window per session
    id even under concurrent requests), window layout persistence/restore with versioned
    schema. Close is a view-collapse only, never a session-stop signal.'
  depends_on: []
  repo_grounding:
    kind: planned
    paths:
    - clients/workspace/src/main/window-registry.ts
    owner: clients/workspace main process (native-clients Phase 2)
    integration_points:
    - Electron BrowserWindow API (sole call site, ESLint-enforced)
    - '%APPDATA%\agent-grid\workspace-state.json persistence (schema-versioned)'
    - component-workspace-main-control-endpoint
    - component-workspace-main-daemon-config (session existence check before restore)
    test_seams:
    - vitest unit tests for the registry map, including race between concurrent openSession
      calls
    - 'Playwright-for-Electron e2e: repeated open does not increase window count;
      close→reopen restores state'
    - ESLint no-restricted-syntax rule forbidding `new BrowserWindow` outside window-registry.ts
    - vitest schema-migration test asserting older/newer workspace-state.json versions
      fall back safely (never silent corruption)
    rationale: plan §4.2 fixes this as the 唯一生成点 and explicitly calls for a lint-enforced
      single-creation-point invariant, directly grounding F-006/F-104/F-107 must-fail
      scenarios; concurrency and schema-evolution gaps are lifted from prose into
      owned contracts per critique.
    accepted_adr_refs: []
  contract_refs:
  - contract-b1-window-registry-dedup
  - contract-migration-window-per-session-invariant
  - contract-window-close-not-session-stop
  - contract-workspace-state-schema-evolution
  decision_closure_reason: ''
- id: component-workspace-main-daemon-config
  name: Workspace main/daemon-config.ts
  responsibility: 'Boundary-2 adapter on the Workspace side: resolves port/token for
    the generated TS SDK, independent of whether the window was opened via boundary-1
    pipe or Workspace''s own on-demand start.'
  depends_on: []
  repo_grounding:
    kind: planned
    paths:
    - clients/workspace/src/main/daemon-config.ts
    owner: clients/workspace main process (native-clients Phase 2)
    integration_points:
    - clients/sdk/ts (generated TS client)
    - UNC token path or Shell-forwarded config
    test_seams:
    - vitest unit tests with a fake token/port source
    - fidelity backstop against the real gateway's ticket-mint-then-/ws flow
    rationale: plan §4.1 names this file as the port/token resolver; clients/sdk/README.md
      names TS as the SDK Workspace consumes.
    accepted_adr_refs:
    - adr-20260724-protocol-cross-language-sdks-supersedes-0021
  contract_refs: []
  decision_closure_reason: daemon-config.ts is a resolver whose observable behavior
    is fully constrained by contract-b2-token-acquisition (UNC-fresh-read semantics)
    and contract-b2-hosted-mode-token-injection (preload push, never URL); alternative
    implementations of the resolver internals (Node fs vs Electron IPC path) do not
    alter the observable auth path.
- id: component-workspace-preload-renderer
  name: Workspace preload/renderer (contextBridge, boundary 4)
  responsibility: 'Typed, minimal contextBridge surface exposed to the hosted SPA:
    windowControls, hostedModeInfo (port/token/sessionId), jump-back request forwarding
    to Shell. Never exposes Node APIs to the renderer.'
  depends_on:
  - component-workspace-main-daemon-config
  repo_grounding:
    kind: planned
    paths:
    - clients/workspace/src/preload/
    owner: clients/workspace preload (native-clients Phase 2)
    integration_points:
    - Electron contextBridge.exposeInMainWorld
    - clients/ui hosted-mode flag consumption (component-uihost-hosted-mode)
    test_seams:
    - 'Playwright-for-Electron: token never observable in URL/network trace; hosted-mode
      branch cases reusing e2e/support/fake-backend.ts'
    rationale: plan §4.1 fixes contextIsolation:true/nodeIntegration:false/sandbox:true
      and the exact exposed surface; src/server/api/auth.go already documents the
      anti-query-param rationale this component must uphold client-side.
    accepted_adr_refs: []
  contract_refs:
  - contract-b2-hosted-mode-token-injection
  decision_closure_reason: ''
- id: component-uihost-hosted-mode
  name: clients/ui hosted-mode branch (existing SPA, modified)
  responsibility: 'Existing browser SPA gains a mode-flag branch: 1-window-1-session
    view, no token-entry UI, no page navigation/scrollbar/context-menu chrome, Chromium
    default accelerators inert. Browser-mode path ships unchanged.'
  depends_on:
  - component-workspace-preload-renderer
  repo_grounding:
    kind: existing
    paths:
    - clients/ui
    - src/uihost
    - src/cmd/uihost
    owner: clients/ui + src/uihost maintainers (existing component, Phase 2 adds a
      branch)
    integration_points:
    - src/server/api/mux.go reverse-proxied /api,/ws
    - window.hostedModeInfo from component-workspace-preload-renderer
    - Electron Menu accelerator override for Ctrl+T / Ctrl+L class
    test_seams:
    - existing clients/ui unit tests
    - existing Playwright e2e (npm run test:e2e) with hosted-flag cases, reusing e2e/support/fake-backend.ts
    - browser-mode regression suite runs unmodified as baseline for contract-hosted-mode-existing-spa-compat
    rationale: plan §4.3 fixes cmd/uihost (go:embed + reverse proxy) as the unchanged
      delivery mechanism; only a mode flag differs. Contract_evolution profile makes
      'browser-mode unaffected' machine-checkable rather than prose per critique.
    accepted_adr_refs: []
  contract_refs:
  - contract-hosted-mode-existing-spa-compat
  decision_closure_reason: ''
- id: component-gateway-ws-ticket-auth
  name: Gateway ticket/bearer auth (existing, unmodified default path)
  responsibility: 'Boundary-2 daemon-side authentication surface: bearer TokenAuth
    for REST, single-use ws-ticket mint/consume for WS. Phase 2 default reuses this
    surface unchanged for native clients (zero server/api change).'
  depends_on: []
  repo_grounding:
    kind: existing
    paths:
    - src/server/api/auth.go
    - src/server/api/ticket.go
    - src/server/api/mux.go
    owner: server/api maintainers (existing surface; Phase 2 is documented reuse,
      not a new owner)
    integration_points:
    - component-shell-core-gatewayclient
    - component-workspace-main-daemon-config
    test_seams:
    - existing src/server/api/auth_test.go, ticket_test.go
    - Phase 2 adds a native-SDK-vs-real-gateway fidelity backstop hitting POST /api/ws-ticket
      then GET /ws?ticket= from a non-browser client
    rationale: auth.go's own comment documents that the ticket flow exists because
      *browser* WS cannot set headers; native clients could set headers, which is
      exactly the open question contract-b2-native-ws-auth-path exists to close deliberately
      rather than by drift.
    accepted_adr_refs:
    - adr-20260724-approval-answerer-identity-per-ws-instance
  contract_refs:
  - contract-b2-native-ws-auth-path
  decision_closure_reason: ''
contracts:
- contract-b1-jsonlines-envelope-shape
- contract-b1-window-registry-dedup
- contract-b3-daemon-supervisor-state-machine
- contract-b3-wsl-detach-mechanism
- contract-b2-token-acquisition
- contract-b2-native-ws-auth-path
- contract-b2-hosted-mode-token-injection
- contract-b1-b2-launch-ordering
- contract-jump-back-staged-resolution
- contract-engage-focus-return-mechanism
- contract-b3-restart-continuity
- contract-toast-panel-watched-detection
- contract-daemon-health-toast-budget
- contract-migration-window-per-session-invariant
- contract-cross-flow-focus-invariant
- contract-deep-link-question-jump-kind-gap
- contract-approve-submission-rollback
- contract-resolved-by-other-display
- contract-health-toast-structural-separation
- contract-window-close-not-session-stop
- contract-workspace-state-schema-evolution
- contract-hosted-mode-existing-spa-compat
- contract-quit-vs-daemon-stop
contract_projections:
- id: contract-b1-jsonlines-envelope-shape
  decision_rules:
  - decision-b1-envelope-validate
  observable_effects:
  - observable-b1-envelope-response
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-b1-malformed-line
  verifications:
  - verify-b1-envelope-strict
  witnesses:
  - witness-b1-openSession-normal
  - witness-b1-malformed-adv
- id: contract-b1-window-registry-dedup
  decision_rules:
  - decision-registry-atomic-openSession
  observable_effects:
  - observable-registry-window-count
  operational_inputs:
  - input-workspace-state-file
  semantic_profiles: []
  failures:
  - failure-registry-corrupt-state-file
  verifications:
  - verify-registry-atomic-dedup
  witnesses:
  - witness-registry-single-open-normal
  - witness-registry-double-open-adv
- id: contract-b3-daemon-supervisor-state-machine
  decision_rules:
  - decision-supervisor-adopt-success
  - decision-supervisor-adopt-unreachable-then-spawn
  - decision-supervisor-spawn-crash
  observable_effects:
  - observable-supervisor-health-indicator
  operational_inputs:
  - input-shell-config-port-distro
  - input-adopt-probe-response
  semantic_profiles:
  - profile-supervisor-outcome-partition
  failures:
  - failure-supervisor-spawn-crash-loop
  verifications:
  - verify-supervisor-partition
  witnesses:
  - witness-supervisor-adopt-normal
  - witness-supervisor-spawn-crash-adv
- id: contract-b3-wsl-detach-mechanism
  decision_rules:
  - decision-wsl-detach-mechanism-witnessed
  observable_effects:
  - observable-wsl-daemon-survives-shell-quit
  operational_inputs:
  - input-wsl-detach-spike-evidence
  semantic_profiles: []
  failures:
  - failure-wsl-detach-evidence-missing
  verifications:
  - verify-wsl-detach-fidelity
  witnesses:
  - witness-wsl-detach-survives-normal
  - witness-wsl-detach-launcher-kill-adv
- id: contract-b2-token-acquisition
  decision_rules:
  - decision-token-fresh-read
  observable_effects:
  - observable-token-fresh-or-explicit-failure
  operational_inputs:
  - input-gateway-bearer-token
  semantic_profiles: []
  failures:
  - failure-token-unc-unreachable
  verifications:
  - verify-token-fresh-and-explicit-failure
  witnesses:
  - witness-token-fresh-normal
  - witness-token-rotation-adv
- id: contract-b2-native-ws-auth-path
  decision_rules:
  - decision-native-ws-reuse-ticket-flow
  observable_effects:
  - observable-native-ws-connects-via-ticket
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-native-ws-header-only
  verifications:
  - verify-native-ws-ticket-fidelity
  witnesses:
  - witness-native-ws-mint-normal
  - witness-native-ws-header-only-adv
- id: contract-b2-hosted-mode-token-injection
  decision_rules:
  - decision-hosted-mode-inject-preload
  observable_effects:
  - observable-hosted-mode-no-token-in-url
  operational_inputs:
  - input-hosted-mode-daemon-config
  semantic_profiles: []
  failures:
  - failure-hosted-mode-no-token
  verifications:
  - verify-hosted-mode-token-not-in-url
  witnesses:
  - witness-hosted-mode-preload-normal
  - witness-hosted-mode-token-leak-adv
- id: contract-b1-b2-launch-ordering
  decision_rules:
  - decision-launch-order-independent
  observable_effects:
  - observable-launch-order-boundary-2-error
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-launch-boundary-2-degraded
  verifications:
  - verify-launch-order-independence
  witnesses:
  - witness-launch-order-normal
  - witness-launch-order-partial-adv
- id: contract-jump-back-staged-resolution
  decision_rules:
  - decision-jb-hwnd-hit
  - decision-jb-process-title-fallback
  - decision-jb-no-match
  - decision-jb-conflicting-identity
  observable_effects:
  - observable-jb-target-foreground-or-explicit-failure
  operational_inputs:
  - input-jumpback-hwnd-cache
  - input-session-target-process-metadata
  semantic_profiles:
  - profile-jb-outcome-partition
  failures:
  - failure-jb-explicit-not-found
  verifications:
  - verify-jb-staged
  witnesses:
  - witness-jb-hwnd-hit-normal
  - witness-jb-stale-hwnd-adv
- id: contract-engage-focus-return-mechanism
  decision_rules:
  - decision-engage-restore-live
  - decision-engage-restore-denied
  - decision-engage-target-destroyed
  observable_effects:
  - observable-engage-restore-or-no-op
  operational_inputs:
  - input-engage-pre-foreground-hwnd
  semantic_profiles:
  - profile-engage-outcome-partition
  failures:
  - failure-engage-target-gone
  - failure-engage-restore-denied
  verifications:
  - verify-engage-restore-partition
  witnesses:
  - witness-engage-restore-normal
  - witness-engage-target-gone-adv
- id: contract-b3-restart-continuity
  decision_rules:
  - decision-restart-normal-reconnect
  - decision-restart-backfill-race
  - decision-restart-etag-mismatch-refetch
  observable_effects:
  - observable-restart-all-surfaces-connected
  operational_inputs:
  - input-last-seen-offset-per-session
  semantic_profiles:
  - profile-restart-cost-convergence
  - profile-restart-outcome-partition
  failures:
  - failure-restart-diverging-offsets
  verifications:
  - verify-restart-continuity-fidelity
  witnesses:
  - witness-restart-3-sessions-normal
  - witness-restart-mid-backfill-race-adv
- id: contract-toast-panel-watched-detection
  decision_rules:
  - decision-panel-watched-open
  - decision-panel-unwatched-standard
  - decision-panel-watched-query-failed
  - decision-panel-watched-signal-conflict
  observable_effects:
  - observable-toast-fires-iff-unwatched
  operational_inputs:
  - input-panel-watched-signal
  semantic_profiles:
  - profile-toast-outcome-partition
  failures:
  - failure-toast-query-unavailable
  verifications:
  - verify-toast-outcome-partition
  witnesses:
  - witness-toast-fires-normal
  - witness-toast-query-failed-adv
- id: contract-daemon-health-toast-budget
  decision_rules:
  - decision-health-tray-only
  observable_effects:
  - observable-zero-non-supervision-toasts
  operational_inputs: []
  semantic_profiles:
  - profile-health-toast-cost-convergence
  failures:
  - failure-health-toast-leaked
  verifications:
  - verify-health-toast-budget
  witnesses:
  - witness-health-toast-quiet-normal
  - witness-health-toast-flap-adv
- id: contract-migration-window-per-session-invariant
  decision_rules:
  - decision-mig-window-per-session
  observable_effects:
  - observable-mig-single-window-and-no-browser
  operational_inputs: []
  semantic_profiles:
  - profile-mig-window-contract-evolution
  failures:
  - failure-mig-second-browserwindow
  verifications:
  - verify-mig-single-window
  witnesses:
  - witness-mig-repeat-open-normal
  - witness-mig-lint-regression-adv
- id: contract-cross-flow-focus-invariant
  decision_rules:
  - decision-focus-invariant-call-site-audit
  observable_effects:
  - observable-focus-invariant-scan-clean
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-focus-invariant-new-call-site
  verifications:
  - verify-focus-invariant-scan
  witnesses:
  - witness-focus-invariant-clean-normal
  - witness-focus-invariant-new-site-adv
- id: contract-deep-link-question-jump-kind-gap
  decision_rules:
  - decision-deeplink-additive-extension
  observable_effects:
  - observable-deeplink-route-traceable-to-schema
  operational_inputs: []
  semantic_profiles:
  - profile-deeplink-additive-schema-evolution
  failures:
  - failure-deeplink-unknown-kind
  verifications:
  - verify-deeplink-schema-fidelity
  witnesses:
  - witness-deeplink-approved-normal
  - witness-deeplink-unknown-kind-adv
- id: contract-approve-submission-rollback
  decision_rules:
  - decision-rollback-success
  - decision-rollback-network-error
  - decision-rollback-server-error
  observable_effects:
  - observable-approve-consistent-with-server
  operational_inputs:
  - input-approve-submission-response
  semantic_profiles:
  - profile-approve-outcome-partition
  failures:
  - failure-approve-network-error
  - failure-approve-server-error
  verifications:
  - verify-approve-rollback
  witnesses:
  - witness-approve-success-normal
  - witness-approve-network-error-adv
- id: contract-resolved-by-other-display
  decision_rules:
  - decision-rbo-accepted
  - decision-rbo-resolved-by-other
  - decision-rbo-network-error
  - decision-rbo-malformed-response
  observable_effects:
  - observable-rbo-already-handled-render
  operational_inputs:
  - input-approval-response-error-kind
  semantic_profiles:
  - profile-rbo-outcome-partition
  failures:
  - failure-rbo-malformed-error-kind
  verifications:
  - verify-rbo-outcome
  witnesses:
  - witness-rbo-normal-win
  - witness-rbo-race-loss-adv
- id: contract-health-toast-structural-separation
  decision_rules:
  - decision-structural-scan
  observable_effects:
  - observable-no-supervisor-toast-edge
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-structural-toast-edge-added
  verifications:
  - verify-structural-toast-separation
  witnesses:
  - witness-structural-clean-normal
  - witness-structural-edge-added-adv
- id: contract-window-close-not-session-stop
  decision_rules:
  - decision-close-view-collapse-only
  observable_effects:
  - observable-close-preserves-session-status
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-close-triggers-stop
  verifications:
  - verify-close-not-stop
  witnesses:
  - witness-close-normal
  - witness-close-reopen-race-adv
- id: contract-workspace-state-schema-evolution
  decision_rules:
  - decision-state-schema-current
  - decision-state-schema-older
  - decision-state-schema-newer-unknown
  observable_effects:
  - observable-state-schema-safe-fallback
  operational_inputs:
  - input-workspace-state-blob
  semantic_profiles:
  - profile-state-schema-outcome-partition
  failures:
  - failure-state-schema-newer-unknown
  verifications:
  - verify-state-schema-safe-fallback
  witnesses:
  - witness-state-schema-current-normal
  - witness-state-schema-newer-unknown-adv
- id: contract-hosted-mode-existing-spa-compat
  decision_rules:
  - decision-hosted-mode-additive
  observable_effects:
  - observable-hosted-mode-additive-only
  operational_inputs: []
  semantic_profiles:
  - profile-hosted-spa-evolution
  failures:
  - failure-hosted-spa-shared-regression
  verifications:
  - verify-hosted-spa-browser-mode-unchanged
  witnesses:
  - witness-hosted-spa-browser-mode-passes-normal
  - witness-hosted-spa-shared-code-regression-adv
- id: contract-quit-vs-daemon-stop
  decision_rules:
  - decision-quit-does-not-stop-daemon
  observable_effects:
  - observable-quit-preserves-daemon
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-quit-stops-daemon
  verifications:
  - verify-quit-vs-stop
  witnesses:
  - witness-quit-preserves-daemon-normal
  - witness-quit-regression-adv
adrs:
- adr-20260724-workspace-host-electron
- adr-20260724-boundary-1-named-pipe-jsonlines
- adr-20260724-boundary-3-adopt-before-spawn
- adr-20260724-boundary-3-wsl-detach-spike
- adr-20260724-boundary-2-token-unc-fresh-read
- adr-20260724-boundary-2-native-ws-auth-reuse-ticket
- adr-20260724-hosted-mode-preload-contextbridge
- adr-20260724-toast-panel-watched-fail-open
- adr-20260724-daemon-health-toast-structural-separation
- adr-20260724-deep-link-schema-additive-extension
- adr-20260724-approval-submission-response-semantics
- adr-20260724-cross-flow-focus-transfer-invariant
- adr-20260724-workspace-state-schema-versioning
- adr-20260724-engage-focus-restore-mechanism
decision_dispositions:
- decision_input_ref: decision-input-winui3-app-sdk
  disposition: implementation_detail
  rationale: 'Draft-1 framing: framework choice does not change observable owner/wire/failure
    semantics for FR-B3-01/FR-TOAST-01/FR-EF-01. Alternatives (WinUI3 vs WPF) preserve
    every accepted contract.'
  adr_refs: []
  contract_refs: []
  implementation_decision_ref: shell-host-framework-choice
- decision_input_ref: decision-input-winui3-windows-app-sdk
  disposition: not_applicable
  rationale: Subsumed by decision-input-winui3-app-sdk (same subject; content byte-identical
    after integrator draft normalization). Recorded as not_applicable to keep validate_plan.py
    CLI clean; the subsumption is documented here in rationale.
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-h-notifyicon
  disposition: adopted
  rationale: H.NotifyIcon.WinUI is adopted for the tray icon under contract-daemon-health-toast-budget's
    tray-only observable. Structural separation ADR references this decision.
  adr_refs: []
  contract_refs:
  - contract-daemon-health-toast-budget
- decision_input_ref: decision-input-h-notifyicon-winui
  disposition: not_applicable
  rationale: Subsumed by decision-input-h-notifyicon (same subject; content byte-identical
    after integrator draft normalization). Recorded as not_applicable to keep validate_plan.py
    CLI clean; the subsumption is documented here in rationale.
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-appnotification-inline-textbox
  disposition: adopted
  rationale: AppNotificationManager is adopted for the panel-unwatched-only toast
    fallback; panel-primary lens uses the button path (short approve/deny). Panel-watched
    fail-open resolution recorded.
  adr_refs: []
  contract_refs:
  - contract-toast-panel-watched-detection
- decision_input_ref: decision-input-composition-acrylic
  disposition: implementation_detail
  rationale: Composition API + Acrylic is the presentation-layer default; alternative
    (flat panel) preserves owner/wire/failure semantics for FR-B3-01. NFR-engage-expand-latency
    and NFR-panel-animation-framerate remain the observable bounds.
  adr_refs: []
  contract_refs: []
  implementation_decision_ref: panel-animation-material-choice
- decision_input_ref: decision-input-composition-api-acrylic
  disposition: not_applicable
  rationale: Subsumed by decision-input-composition-acrylic (same subject; content
    byte-identical after integrator draft normalization). Recorded as not_applicable
    to keep validate_plan.py CLI clean; the subsumption is documented here in rationale.
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-electron-workspace-host
  disposition: adopted
  rationale: Electron is adopted per adr-20260724-workspace-host-electron; alternatives
    explicitly compared in the ADR.
  adr_refs: []
  contract_refs:
  - contract-migration-window-per-session-invariant
- decision_input_ref: decision-input-named-pipe-jsonlines
  disposition: adopted
  rationale: Named pipe + closed {op,id} JSON Lines is adopted per adr-20260724-boundary-1-named-pipe-jsonlines.
  adr_refs: []
  contract_refs:
  - contract-b1-jsonlines-envelope-shape
- decision_input_ref: decision-input-named-pipe-jsonlines-ipc
  disposition: not_applicable
  rationale: Subsumed by decision-input-named-pipe-jsonlines (same subject; content
    byte-identical after integrator draft normalization). Recorded as not_applicable
    to keep validate_plan.py CLI clean; the subsumption is documented here in rationale.
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-jump-back-provenance-handoff
  disposition: adopted
  rationale: OPT-STAGED-BEST-EFFORT is adopted per contract-jump-back-staged-resolution;
    cross-flow focus invariant lifts the fabricated-fallback prohibition to FR-FOCUS-INV.
  adr_refs: []
  contract_refs:
  - contract-jump-back-staged-resolution
  - contract-cross-flow-focus-invariant
- decision_input_ref: decision-input-jump-back-target-provenance
  disposition: not_applicable
  rationale: Subsumed by decision-input-jump-back-provenance-handoff (same subject;
    content byte-identical after integrator draft normalization). Recorded as not_applicable
    to keep validate_plan.py CLI clean; the subsumption is documented here in rationale.
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-stdlib-only-go-wire
  disposition: adopted
  rationale: 'AGENTS.md constraint: Go wire/persistence types remain stdlib-only.
    adr-20260724-boundary-2-native-ws-auth-reuse-ticket preserves this by not extending
    the Go WS handler.'
  adr_refs: []
  contract_refs:
  - contract-b2-native-ws-auth-path
- decision_input_ref: decision-input-cross-language-sdk-strategy
  disposition: adopted
  rationale: 'Accepted ADR: generated SDKs (OpenAPI Generator, clients/sdk/{csharp,ts,...})
    supersede ADR-0021''s hand-written scope. GatewayClient wraps the generated C#
    SDK; Workspace consumes the generated TS SDK.'
  adr_refs: []
  contract_refs:
  - contract-b2-native-ws-auth-path
- decision_input_ref: decision-input-capability-negotiation-axis
  disposition: adopted
  rationale: 'Accepted ADR: bundled clients skip per-capability negotiation. DaemonSupervisor''s
    adopt-then-spawn ordering (adr-20260724-boundary-3-adopt-before-spawn) applies
    to bundled default.'
  adr_refs: []
  contract_refs:
  - contract-b3-daemon-supervisor-state-machine
- decision_input_ref: decision-input-answerer-identity
  disposition: adopted
  rationale: 'Accepted ADR: per-WS ephemeral client-instance-id. SupervisionState''s
    resolved-by-other rendering consumes this identity unchanged (contract-resolved-by-other-display).'
  adr_refs: []
  contract_refs:
  - contract-resolved-by-other-display
- decision_input_ref: decision-input-window-registry-lint-boundary
  disposition: implementation_detail
  rationale: Draft-2 identifies ESLint no-restricted-syntax as the enforcement mechanism;
    the observable invariant (contract-migration-window-per-session-invariant) does
    not change if a Roslyn-equivalent or ripgrep step is used instead.
  adr_refs: []
  contract_refs: []
  implementation_decision_ref: workspace-window-registry-lint-mechanism
- decision_input_ref: decision-input-fcis-supervision-state
  disposition: adopted
  rationale: 'AGENTS.md testability constraint + host/state precedent: SupervisionState
    is a pure Reduce(state, event) function with a thin I/O shell. contract-approve-submission-rollback
    and contract-resolved-by-other-display both depend on this pure-function seam.'
  adr_refs: []
  contract_refs:
  - contract-approve-submission-rollback
  - contract-resolved-by-other-display
milestones:
- id: chunk-s1-connection-supervision
  depends_on: []
  units:
  - unit-daemon-supervisor-state-machine
  - unit-gateway-client-token-and-ticket
  - unit-wsl-detach-spike
  - unit-daemon-health-tray-appearance
  - unit-health-toast-structural-lint
  - unit-quit-vs-daemon-stop-menus
- id: chunk-s1a-s3-prototypes-gate
  depends_on:
  - chunk-s1-connection-supervision
  units:
  - unit-s3-prototypes-gate
- id: chunk-s2-panel-glance
  depends_on:
  - chunk-s1a-s3-prototypes-gate
  units:
  - unit-cross-flow-focus-invariant-lint
  - unit-jump-back-staged-resolution
- id: chunk-s3-approval-round-trip
  depends_on:
  - chunk-s2-panel-glance
  - chunk-s1a-s3-prototypes-gate
  units:
  - unit-approve-submission-rollback
  - unit-resolved-by-other-display
  - unit-engage-focus-return
  - unit-toast-notifier-panel-watched
- id: chunk-s4-workspace
  depends_on:
  - chunk-s1-connection-supervision
  units:
  - unit-workspace-window-registry
  - unit-workspace-control-endpoint
  - unit-workspace-launcher
  - unit-workspace-preload-renderer
  - unit-uihost-hosted-mode-branch
  - unit-deep-link-router-schema-gap
- id: chunk-s5-finishing
  depends_on:
  - chunk-s3-approval-round-trip
  - chunk-s4-workspace
  units:
  - unit-restart-continuity-integration
reference_algorithms: []
implementation_decisions_remaining:
- decision: shell-host-framework-choice
  representative_alternatives:
  - id: alternative-shell-host-winui3
    description: WinUI 3 / Windows App SDK
  - id: alternative-shell-host-wpf
    description: WPF (legacy XAML, mature Win32 interop)
  preserved_contract_refs:
  - contract-b3-daemon-supervisor-state-machine
  - contract-toast-panel-watched-detection
  - contract-engage-focus-return-mechanism
  invariance_argument: Both frameworks host the same IWin32InteropService seam; the
    state machine, panel-watched predicate, and engage-focus-return semantics are
    implemented in Shell.Core which is framework-agnostic. Observable outcomes are
    the same across frameworks per the contract witnesses.
  invariance_witnesses:
  - id: invariance-shell-host-framework
    counterexample_input: Approval event arrives while panel unwatched
    alternative_observations:
    - alternative_ref: alternative-shell-host-winui3
      observable_refs:
      - observable-toast-fires-iff-unwatched
      expected_observation: Toast fires; panel queue updates alone when watched
    - alternative_ref: alternative-shell-host-wpf
      observable_refs:
      - observable-toast-fires-iff-unwatched
      expected_observation: Toast fires; panel queue updates alone when watched
    equivalence_criterion: The toast-or-not outcome and panel queue count match across
      both frameworks for the same (panel-open, DND, locked, query-ok) tuple.
    verification_ref: verify-toast-outcome-partition
- decision: panel-animation-material-choice
  representative_alternatives:
  - id: alternative-panel-composition-acrylic
    description: Composition API + Acrylic material
  - id: alternative-panel-flat-solid
    description: Flat solid-color panel without Composition-driven animation
  preserved_contract_refs:
  - contract-toast-panel-watched-detection
  - contract-engage-focus-return-mechanism
  invariance_argument: Both alternatives yield identical panel-watched predicate outcomes
    and engage-focus-return behavior. NFR-engage-expand-latency and NFR-panel-animation-framerate
    are observable bounds that constrain the acceptable range of both alternatives
    (Composition + Acrylic meets them by default; flat solid can meet them under low-spec-GPU
    degradation).
  invariance_witnesses:
  - id: invariance-panel-animation
    counterexample_input: Panel flyout opens while an approval event arrives
    alternative_observations:
    - alternative_ref: alternative-panel-composition-acrylic
      observable_refs:
      - observable-toast-fires-iff-unwatched
      expected_observation: Watched classification when flyout open; no toast
    - alternative_ref: alternative-panel-flat-solid
      observable_refs:
      - observable-toast-fires-iff-unwatched
      expected_observation: Watched classification when flyout open; no toast
    equivalence_criterion: Panel-watched semantics identical across animation styles.
    verification_ref: verify-toast-outcome-partition
- decision: workspace-window-registry-lint-mechanism
  representative_alternatives:
  - id: alternative-lint-eslint-no-restricted-syntax
    description: ESLint no-restricted-syntax rule blocking `new BrowserWindow` outside
      window-registry.ts
  - id: alternative-lint-ripgrep-ci-step
    description: CI ripgrep step that greps for `new BrowserWindow(` outside window-registry.ts
  preserved_contract_refs:
  - contract-migration-window-per-session-invariant
  invariance_argument: 'Both alternatives produce identical observable outcomes: a
    PR adding `new BrowserWindow` outside window-registry.ts fails CI. The choice
    between an ESLint rule and a ripgrep step does not change the invariant or its
    verification.'
  invariance_witnesses:
  - id: invariance-registry-lint
    counterexample_input: PR adds `new BrowserWindow` in clients/workspace/src/main/random.ts
    alternative_observations:
    - alternative_ref: alternative-lint-eslint-no-restricted-syntax
      observable_refs:
      - observable-mig-single-window-and-no-browser
      expected_observation: CI ESLint step fails; PR blocked
    - alternative_ref: alternative-lint-ripgrep-ci-step
      observable_refs:
      - observable-mig-single-window-and-no-browser
      expected_observation: CI ripgrep step fails; PR blocked
    equivalence_criterion: Both mechanisms fail CI on the same counterexample PR.
    verification_ref: verify-mig-single-window
- decision: jump-back-per-app-matching-rule-table-content
  representative_alternatives:
  - id: alternative-jb-rules-data-file
    description: Rules encoded as per-target-app data (process name + title patterns)
      in a JSON/YAML rule table
  - id: alternative-jb-rules-attribute-based
    description: Rules encoded as C# attributes on per-target-app handler classes
  preserved_contract_refs:
  - contract-jump-back-staged-resolution
  invariance_argument: Both alternatives preserve the outcome_partition (activated
    / not-found / conflicting) for the same (window-enumeration, session-metadata)
    inputs. Adding a new target app extends the data/attribute set without changing
    decision-rule structure.
  invariance_witnesses:
  - id: invariance-jb-rule-table
    counterexample_input: Session created for Windows Terminal with cwd=/home/user;
      window exists with matching cwd
    alternative_observations:
    - alternative_ref: alternative-jb-rules-data-file
      observable_refs:
      - observable-jb-target-foreground-or-explicit-failure
      expected_observation: Target window becomes foreground
    - alternative_ref: alternative-jb-rules-attribute-based
      observable_refs:
      - observable-jb-target-foreground-or-explicit-failure
      expected_observation: Target window becomes foreground
    equivalence_criterion: Both encodings produce identical resolution outcomes for
      the enumerated per-app scenarios.
    verification_ref: verify-jb-staged
- decision: engage-restore-primary-win32-mechanism
  representative_alternatives:
  - id: alternative-engage-attachthreadinput
    description: AttachThreadInput-based SetForegroundWindow restore (adr-20260724-engage-focus-restore-mechanism
      baseline)
  - id: alternative-engage-sendinput-trick
    description: SendInput-synthetic-key trick to satisfy Windows' foreground-lock
      heuristic
  preserved_contract_refs:
  - contract-engage-focus-return-mechanism
  invariance_argument: Both alternatives target the same outcome partition (restore-live
    / restore-denied / target-destroyed). The choice depends on S1-S3 prototype evidence
    about which mechanism the OS accepts more consistently; observable outcomes on
    the three branches match.
  invariance_witnesses:
  - id: invariance-engage-mechanism
    counterexample_input: VS Code foreground at engage entry; unrelated window briefly
      grabs foreground mid-engage
    alternative_observations:
    - alternative_ref: alternative-engage-attachthreadinput
      observable_refs:
      - observable-engage-restore-or-no-op
      expected_observation: VS Code foreground restored on confirm
    - alternative_ref: alternative-engage-sendinput-trick
      observable_refs:
      - observable-engage-restore-or-no-op
      expected_observation: VS Code foreground restored on confirm
    equivalence_criterion: Both mechanisms produce the same 'target restored on live-HWND
      branch' observable.
    verification_ref: verify-engage-restore-partition
- decision: restart-reconnect-backoff-cadence-final-value
  representative_alternatives:
  - id: alternative-reconnect-full-jitter-30s-cap
    description: Full-jitter exponential backoff capped at 30s per attempt (adr-20260624-0012
      pattern reused)
  - id: alternative-reconnect-decorrelated-jitter
    description: Decorrelated-jitter backoff with the same 30s cap
  preserved_contract_refs:
  - contract-b3-restart-continuity
  invariance_argument: Both alternatives are bounded by NFR-daemon-restart-reconnect-delay's
    measured value; the outcome partition (normal-reconnect / backfill-race / etag-mismatch)
    fires identically regardless of jitter shape. The concrete threshold value is
    pending S1 measurement per ADR-0025-extension.
  invariance_witnesses:
  - id: invariance-reconnect-backoff
    counterexample_input: 3 sessions; daemon restart; measure time to Connected on
      Shell panel
    alternative_observations:
    - alternative_ref: alternative-reconnect-full-jitter-30s-cap
      observable_refs:
      - observable-restart-all-surfaces-connected
      expected_observation: All surfaces Connected within NFR-daemon-restart-reconnect-delay
    - alternative_ref: alternative-reconnect-decorrelated-jitter
      observable_refs:
      - observable-restart-all-surfaces-connected
      expected_observation: All surfaces Connected within NFR-daemon-restart-reconnect-delay
    equivalence_criterion: Both jitter shapes converge within the S1-measured threshold
      with the same session-set observable.
    verification_ref: verify-restart-continuity-fidelity
---

## Component decomposition

Thirteen components spread across three trees: Shell.Core / Shell.Platform / Shell UI
(clients/windows-shell), Workspace main + preload + hosted-mode SPA branch (clients/workspace
and clients/ui + src/uihost), and the reused-in-place server gateway auth surface
(src/server/api).

Every planned component names `repo_grounding.paths`, `owner`, `integration_points`,
`test_seams`, `rationale`, and `accepted_adr_refs` in the frontmatter above. `existing`
components (component-uihost-hosted-mode, component-gateway-ws-ticket-auth) point at paths
that exist in the repo today.

## Implementation contracts

Twenty-three contracts covering all 11 declared design dimensions. Each contract fixes:

- **owner_component_ref** — the component that owns the contract.
- **decision_rules** — the (input → outcome) branches, with evidence_mode and epistemic_state.
- **observable_effects** — externally observable results referenced by every witness.
- **operational_inputs** — the information/authority/versioned_state each observable depends on,
  with acquired_at → required_until continuity, mutability, invalidation, and unavailable_outcome.
- **semantic_profiles** — cost_convergence / scope_consistency / outcome_partition /
  contract_evolution as required by the contract's dimension and evidence mode.
- **invariants** and **failure_semantics** — what must always hold / what happens when it doesn't.
- **verification** — T0/T1/T2/T3 tier, method, and criterion.
- **witnesses** — normal + adversarial witnesses per adr-20260713-design-contract-witness-closure.

The full contract text lives in `plan.json`; this member's `contract_projections` array is the
stable-ID projection the postflight validator matches against.

## ADRs

Fourteen ADRs (all `status: proposed`) cover the design-choice decisions surfaced by both drafts
and by critique. Each ADR lives at `docs/adr/adr-20260724-<slug>.md` and reciprocates the plan's
`decision_dispositions[].adr_refs` and `implementation_contracts[].adr_refs`.

## Seams and testability

- **Fake + fidelity backstop + contract test triple** per AGENTS.md for every external-dependency
  boundary crossing (GatewayClient, WorkspaceLauncher, DaemonSupervisor.Runner, control-endpoint,
  daemon-config, jump-back window enumeration).
- **Roslyn analyzer / NetArchTest** for structural invariants (health/toast separation, cross-flow
  focus invariant, Quit vs Stop-daemon menu split).
- **Playwright-for-Electron** for the Workspace window discipline (repeat open = single window,
  close ≠ session stop, token not in URL).
- **ESLint no-restricted-syntax** for `new BrowserWindow` outside `window-registry.ts`.

## Milestones

S1–S5 dependency-ordered per `milestones` above. Each milestone lists its `units`; each unit's
`contract_refs` and `decision_closure_reason` are in `plan.json.chunks[].units[]`. S1 gates on
the WSL detach spike; S3 gates on the S3 pre-implementation prototypes.

