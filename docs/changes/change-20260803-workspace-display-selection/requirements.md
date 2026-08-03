---
change: change-20260803-workspace-display-selection
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- FR-DISPLAY-001: Workspace Settings shall list currently available desktop displays,
  identify the primary display, and let the user select the display used by Windows Shell.
- FR-DISPLAY-002: Saving Settings shall persist the Shell selection in shared client
  configuration without exposing a settings surface in Shell.
- FR-DISPLAY-003: On startup, Shell shall place its panel on the selected display. If
  the saved anchor no longer resolves to a display, Shell shall use the primary display.
- FR-DISPLAY-004: Workspace shall observe which display contains a moved Workspace
  window and persist that display in `workspace-state.json`.
- FR-DISPLAY-005: After a Workspace process restart, the first home, settings, or
  session window shall open inside the last available display used. If it is no longer
  available, Workspace shall use the primary display.
- FR-DISPLAY-006: Existing schema-version 2 Workspace state shall upgrade in memory;
  unknown future versions and malformed state shall continue to fail safe.

## Acceptance scenarios

- Given two displays, when the user selects the secondary display in Workspace Settings
  and restarts Shell, then the Shell panel's top edge is within that display's work area.
- Given a Workspace window moved to a secondary display, when Workspace exits and starts
  again, then its first window bounds are within that display's work area.
- Given a saved display that was disconnected, when either desktop app starts, then it
  starts on the primary display without an off-screen window or startup failure.
- Counterexample: a Shell-local settings window or Electron numeric display id persisted
  for direct WinAppSDK lookup does not satisfy the ownership/interoperability boundary.
