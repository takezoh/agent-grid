---
change: change-20260803-workspace-display-selection
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Approach

1. Extend `shell.json` with an optional physical-pixel `display_anchor`. Workspace
   enumerates displays through Electron, converts each display center from DIP to a
   physical point, and owns the selection UI and writer.
2. Shell resolves that anchor with `DisplayArea.GetFromPoint` and uses the resulting work
   area for its initial panel placement. Missing/invalid anchors fall back to primary.
3. Upgrade `workspace-state.json` from schema 2 to schema 3 with `last_display` containing
   Electron's display id plus DIP bounds. The id is Workspace-internal; saved bounds give
   a topology fallback if the id changes.
4. A pure display-placement controller centers new windows inside the restored work area,
   clamps oversized defaults, and records display changes on Electron's `moved` event.

## Boundaries

- Shell does not enumerate or edit settings; it only consumes the optional anchor.
- Display selection is client presentation state and never enters server configuration.
- Session-specific window bounds remain supported, but process-restart placement targets
  the globally last used Workspace display.
