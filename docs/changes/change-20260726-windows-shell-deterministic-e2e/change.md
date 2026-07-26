---
id: change-20260726-windows-shell-deterministic-e2e
kind: change
title: Right-size Windows Shell E2E with a fake gateway
status: done
created: '2026-07-26'
profile: sdd@1
intent: Keep only OS, transport-fidelity, and native-rendering concerns in Windows
  T3 while driving Shell UI states through a deterministic fake gateway.
summary: Right-size Windows Shell E2E and wire server events into native UIA tests.
outcomes:
- Real gateway T3 is limited to REST and ticket-to-WebSocket fidelity.
- Native UIA deterministically renders sessions, approval, question, and status icons
  from fake server events.
scope:
- clients/windows-shell test harness and E2E documentation
non_goals:
- Production Shell behavior or gateway protocol changes
- Workspace Electron automation
change_classes:
- internal_design
- implementation_only
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: windows:dotnet test AgentGrid.Shell.sln
- type: test
  ref: windows:FlaUI PanelWindowUiE2E
- type: test
  ref: dev-docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260726-windows-shell-deterministic-e2e/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-windows-shell-deterministic-e2e/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-windows-shell-deterministic-e2e/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: The testing note and Windows E2E guide are updated directly; no component
    design invariant changes.
unresolved_decisions: []
tags:
- testing
- windows-shell
- fake
owners: []
relations:
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- clients/windows-shell/AgentGrid.Shell.Core.Tests/E2E/RunDevGatewayE2ETests.cs
- clients/windows-shell/AgentGrid.Shell.WinUI.UiTests/E2E/FakeGatewayServer.cs
- clients/windows-shell/AgentGrid.Shell.WinUI.UiTests/E2E/PanelUiSession.cs
- clients/windows-shell/AgentGrid.Shell.WinUI.UiTests/E2E/PanelWindowUiE2ETests.cs
- clients/windows-shell/docs/e2e.md
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T11:41:46.670468+00:00'
closure:
  closed_at: '2026-07-26T11:41:50.463578+00:00'
  content_hash: sha256:0ce4527c3fc2cbcdc2c2423e3c3ac7cac8ab910d1d997c17727c709301533662
---

## Summary

The previous UIA suite consumed an empty real server, making event-driven
sections absent and session assertions conditional. A fixture-owned fake now
emits exact Shell REST/WS frames; Windows T3 retains the executable, WinUI,
UIA, layout, and accessibility boundaries.

## Closure Notes
