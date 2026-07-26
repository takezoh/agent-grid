---
id: adr-20260726-shared-status-display-matrix
kind: adr
title: Use one checked-in language-neutral status display matrix
status: accepted
created: '2026-07-26'
updated: '2026-07-26'
tags:
- adr
- session-status
owners: []
relations:
- {type: references, target: change-20260726-unified-driver-session-status}
source_paths:
- clients/ui
- clients/ui/src/components/StatusIcon.tsx
- clients/windows-shell
- clients/windows-shell/AgentGrid.Shell.Core/SupervisionState/PanelPresentation.cs
- clients/workspace
- src/host/driver
- src/host/runtime/subsystem
- src/host/runtime/subsystem/stream/event.go
- src/host/state
- src/host/state/view/status.go
- src/server/api
decision_makers:
- user
summary: Keep one checked-in semantic-key fixture consumed directly by parity tests and renderer adapters. Do not place glyph metadata on the runtime wire. Use dedicated unknown rather than stale last-known.
---

# Use one checked-in language-neutral status display matrix

## Context

API metadata and independent client mappings create competing owners and runtime payload complexity.

## Decision

Keep one checked-in semantic-key fixture consumed directly by parity tests and renderer adapters. Do not place glyph metadata on the runtime wire. Use dedicated unknown rather than stale last-known.

## Consequences

- All clients share labels and allowed combinations.
- Unknown cannot masquerade as current state.
- Native renderers remain free to use platform glyph implementations within the fixed semantic role.

## Alternatives considered

### Generated API metadata

Rejected: Couples presentation assets to runtime wire and duplicates native mapping boundaries.

### Last-known plus stale

Rejected: Can present unauthoritative state as current.
