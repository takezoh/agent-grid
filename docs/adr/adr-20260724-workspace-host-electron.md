---
id: adr-20260724-workspace-host-electron
kind: adr
title: Workspace window host is Electron (electron-builder dir target) for Phase 2
status: accepted
created: '2026-07-24'
summary: Workspace window host is Electron (electron-builder dir target) for Phase
  2
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - TS-language sharing with clients/ui reduces per-session code duplication and reuses
    e2e/support/fake-backend.ts for Playwright-for-Electron.
  - Multi-window + IME + xterm.js maturity is available out of the box.
  negative:
  - ~150-250MB idle memory per open window; acceptable because Workspace is on-demand
    under DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND (accepted).
  neutral:
  - electron-builder dir target defers distribution; a future MSIX packaging ADR will
    supersede as needed.
---
# Workspace window host is Electron (electron-builder dir target) for Phase 2

## Context

{% context %}
Phase 2 needs a hosted-mode window host for on-demand session windows (F-006/F-107). Options: Electron (electron-builder dir target), WinUI3 + WebView2 island, continued browser-tab hosting. cc-no-browser-in-local-flow disqualifies browser-tab hosting outright. Electron delivers xterm.js/IME/multi-window maturity and TS-language sharing with clients/ui at the cost of a second runtime; WebView2 island avoids the second runtime but pushes multi-window + IME + web↔native bridge work onto hand-written C#.
{% /context %}

## Decision

{% decision %}
Adopt Electron (electron-builder dir target) as the Workspace host. clients/workspace/ owns the main process, preload, and hosted-mode integration. Distribution (installer/signing/auto-update) is explicitly deferred.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- TS-language sharing with clients/ui reduces per-session code duplication and reuses e2e/support/fake-backend.ts for Playwright-for-Electron.
- Multi-window + IME + xterm.js maturity is available out of the box.
{% /consequence %}

{% consequence kind="negative" %}
- ~150-250MB idle memory per open window; acceptable because Workspace is on-demand under DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND (accepted).
{% /consequence %}

{% consequence kind="neutral" %}
- electron-builder dir target defers distribution; a future MSIX packaging ADR will supersede as needed.
{% /consequence %}

## Alternatives

- **WinUI3 + WebView2 island** — Pushes multi-window/IME management into hand-written C# with less xterm.js precedent; rejected per plan §1.1.
- **Continued browser-tab hosting** — Disqualified by cc-no-browser-in-local-flow.

## Related

- decision inputs: `decision-input-electron-workspace-host`
- requirements: `FR-MIG-01`, `FR-MIG-02`, `FR-MIG-03`
- contracts: `contract-migration-window-per-session-invariant`
- change: `change-20260723-windows-shell-phase2`
