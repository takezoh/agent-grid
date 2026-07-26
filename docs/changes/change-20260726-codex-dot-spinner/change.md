---
id: change-20260726-codex-dot-spinner
kind: change
title: Use the Codex dot spinner for working status
status: done
created: '2026-07-26'
profile: sdd@1
intent: Replace the circular progress-ring working indicator with the Codex-style
  radial dot spinner in every desktop UI.
summary: Use one eight-dot rotating spinner for Running and Working in Web/Workspace
  and Windows Shell.
outcomes:
- Running and Working render as an eight-dot opacity trail rather than a circle and
  arc.
- Bell and action-required composition semantics remain unchanged.
scope:
- clients/ui StatusIcon
- clients/windows-shell WinUI session status icon
non_goals:
- Status state definitions or event mapping
- Waiting, idle, stopped, failed, bell, or action geometry
change_classes:
- behavior
- implementation_only
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: clients/ui:StatusIcon unit + real-stack Playwright
- type: test
  ref: windows:FlaUI PanelWindowUiE2E
- type: test
  ref: dev-docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260726-codex-dot-spinner/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-codex-dot-spinner/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-codex-dot-spinner/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: This changes presentation geometry without altering the durable status contract.
unresolved_decisions: []
tags:
- status
- ui
- windows-shell
owners: []
relations:
- {type: originatedFrom, target: change-20260726-unified-driver-session-status}
source_paths:
- clients/ui/src/components/StatusIcon.tsx
- clients/ui/src/css/status-icon.css
- clients/windows-shell/AgentGrid.Shell.WinUI/Panel/PanelWindow.xaml
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T11:52:26.684633+00:00'
closure:
  closed_at: '2026-07-26T11:53:29.546834+00:00'
  content_hash: sha256:49de6ab7bc754248079366199aee93405793b55c13ce8885348d336754d5a4e2
---

## Summary

The former faded circle plus foreground arc read as a generic progress ring.
Both clients now use the same eight radial points with a rotating opacity tail,
matching the compact dotted spinner shown by Codex terminal tabs.

## Closure Notes
