---
id: design-clients-ui
kind: design
title: Shared SPA (browser and hosted modes)
status: active
created: '2026-07-25'
scope_type: component
responsibilities:
- id: RESP-001
  statement: Present sessions, terminals, transcripts, approvals, and the command
    palette as one React SPA consumed by browsers (served by uihost) and by the Electron
    workspace (bundled).
- id: RESP-002
  statement: Own the TypeScript wire codec for gateway frames (hello/delta session
    updates, approval/question broadcasts) as the browser-side counterpart of the
    server wire contract.
- id: RESP-003
  statement: Resolve authentication per mode — browser bootstrap flow versus hosted
    preload-injected token — behind one auth module.
- id: RESP-004
  statement: Own layout chrome (app shell grid, sidebar, drawer, header) and its responsive
    and persisted-state behavior.
- id: RESP-005
  statement: Own the translation between the compound fleet session key ([serverId,
    sessionId]) and the upstream bare session id, with one owner per boundary — Connection
    for the WS wire, and the fleet identity module (SessionRoute resolver + FleetSessionRow)
    for store and HTTP routing.
invariants:
- id: INV-001
  statement: One SPA serves both modes; mode differences are runtime flags (hostedModeInfo
    presence), never build-time forks.
  enforcement: review
- id: INV-002
  statement: Hosted mode is additive — browser-mode URL contract, CSP, and WS-origin
    checks stay unchanged, and the browser Playwright suite passes unmodified (FR-HOSTED-SPA-COMPAT).
  enforcement: test
- id: INV-003
  statement: Bearer tokens never appear in URLs; hosted tokens arrive only via the
    preload contextBridge.
  enforcement: test
- id: INV-004
  statement: The sidebar is hosted-independent — default visible in every mode, collapsible
    via the desktop toggle, with the choice persisted across launches (user decision
    2026-07-25); the hosted flag only drives drag-region and scrollbar styling.
  enforcement: test
- id: INV-005
  statement: Color values live in tokens.css only; component styles reference tokens
    (no hex/rgb literals outside tokens).
  enforcement: test
- id: INV-006
  statement: UI-local view state (drawer open, sidebar collapse) stays in component
    state per FR-STORE-001; the Zustand daemon store remains the sole owner of session-selection
    state.
  enforcement: review
- id: INV-901
  statement: New Session target rows expose source and fully qualified identity in
    every modality; independently failing sources never globally block healthy targets.
  enforcement: contract
- id: INV-007
  statement: Connection's public API speaks fleet keys only; ClientFrame assembly
    and serialization stay private to socket/connection.ts, and every outbound WS
    frame carries the upstream bare session id.
  enforcement: test
- id: INV-008
  statement: Every inbound session-id-bearing element is requalified to a fleet key
    through one path before reaching the store; store rows carry (serverId, upstreamSessionId)
    as required fields and session identity is decided by strict fleet-key equality
    (no alias fallbacks).
  enforcement: test
- id: INV-009
  statement: Data-plane session-id resolution failures never throw into the xterm
    emitter; the frame is dropped and recorded by reason — internal contract violations
    reach the user, benign server-switch transitions only increment a diagnostic counter.
  enforcement: test
boundaries:
  provides:
  - The complete session UI surface for browser and hosted consumers.
  - The TS wire codec and connection bootstrap consumed nowhere else.
  consumes:
  - Gateway REST/WS endpoints; uihost static serving; workspace preload context.
  forbidden:
  - Mode-specific feature forks beyond the hosted context flag.
  - Reading tokens from URLs or persisting them in web storage.
  - id: BND-004
    statement: Handling the fleet session key syntactically (sessionKey / parseSessionKey)
      or serializing a ClientFrame outside the declared owner set, and re-deriving
      a session route from an id string when the authoritative row is in hand.
variability:
  fixed:
  - Single-bundle dual-mode delivery and the additive hosted flag.
  - Named-grid app shell with breakpoint-driven sidebar/drawer swap.
  free:
  - Visual themes and density (token-driven), component composition, palette tooling.
capabilities:
- id: cap:shared-session-spa
  uniqueness: global
failure_responsibilities:
- Connection loss degrades to status banner + reconnect; the SPA never hard-fails
  the window.
trust_boundaries:
- All gateway input is validated at the codec boundary; hosted preload input is treated
  as trusted same-user context.
compatibility_policies:
- Wire codec changes track the server's discriminated frame union; unknown frame kinds
  are ignored deliberately, known kinds must be handled.
tags:
- web
- native-clients
owners: []
relations: []
source_paths:
- clients/ui/
summary: 'Governs the shared React SPA served to browsers by uihost and embedded by
  the Electron workspace: dual-mode auth, wire codec consumption, session presentation,
  and hosted-independent chrome.'
---

## Purpose

clients/ui is the one session UI. The 3c53b116 split moved it out of the server
tree so browsers (via uihost embedding) and the Electron workspace (via bundle
copy) consume the identical artifact; hosted mode adds context, never forks
behavior.

## Conformance

- Unit suite (vitest + happy-dom) pins layout, store discipline, tokens, and the
  hosted-independent sidebar behavior; Playwright smoke pins real-browser wiring
  against the deterministic fake backend (design-host browser-harness section).
- Legacy component detail remains embedded in design-host's migration history;
  this document is the governing surface for clients/ui.

## Related Decisions

- Sidebar default-visible + persisted collapse: user decision 2026-07-25
  (feat(ui) commit 11480f81 range).
- Hosted token injection contract: change-20260723-windows-shell-phase2
  FR-B2-04 / FR-HOSTED-SPA-COMPAT.
