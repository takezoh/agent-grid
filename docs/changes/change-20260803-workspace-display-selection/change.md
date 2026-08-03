---
id: change-20260803-workspace-display-selection
kind: change
title: Workspace display selection and restore
status: draft
created: '2026-08-03'
profile: sdd@1
intent: Enable users to choose the Windows Shell display from Workspace settings
  while restoring Workspace windows to the last display used.
outcomes:
- Workspace Settings lists the currently available displays and persists the selected
  Shell display without adding a Shell-owned settings surface.
- Windows Shell places its panel on the configured display and falls back safely when
  that display is unavailable.
- Workspace records the display containing its most recently moved window and creates
  windows on that display after the next process start.
scope:
- clients/workspace
- clients/windows-shell
- clients/config/shell.schema.json
- clients/config/README.md
- docs/design/design-workspace-desktop.md
- docs/design/design-windows-shell.md
non_goals:
- Per-session display preferences
- Hot-reloading a running Shell after the setting is saved
- Persisting Electron or WinAppSDK implementation-specific display identifiers across clients
change_classes:
- behavior
- capability
- responsibility
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260803-workspace-display-selection/requirements.md
  required: true
- role: implementation
  path: changes/change-20260803-workspace-display-selection/implementation.md
  required: true
- role: verification
  path: changes/change-20260803-workspace-display-selection/verification.md
  required: true
promotion: []
unresolved_decisions: []
tags:
- workspace
- windows-shell
- display
owners: []
relations:
- {type: modifies, target: design-workspace-desktop}
- {type: modifies, target: design-windows-shell}
- {type: references, target: adr-20260724-workspace-state-schema-versioning}
source_paths:
- clients/workspace
- clients/windows-shell
- clients/config
---

## Summary

Workspace owns the settings UI and Electron display observation. The selected Shell
display crosses the process boundary as a physical screen-coordinate anchor in
`shell.json`, avoiding an invalid assumption that Electron `Display.id` and Windows
App SDK `DisplayId` share a representation. Workspace's own last-display state remains
in the versioned `workspace-state.json` runtime record.

## Closure Notes

The pre-existing docs lint failure for unregistered members under
`change-20260727-resilient-server-reconnect-health` is outside this change's scope.
