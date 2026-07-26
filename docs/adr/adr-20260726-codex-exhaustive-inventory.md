---
id: adr-20260726-codex-exhaustive-inventory
kind: adr
title: Use a build-time Codex method inventory with runtime unknown diagnostics
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
summary: Bind a checked-in inventory to the supported Codex version and route every known method explicitly. Use fake, FakeVsReal, and invariant-naming contract tests; unknown and malformed messages preserve state and enter bounded diagnostics.
---

# Use a build-time Codex method inventory with runtime unknown diagnostics

## Context

The real app-server does not provide a dependable connection-time protocol-version handshake for routing authority.

## Decision

Bind a checked-in inventory to the supported Codex version and route every known method explicitly. Use fake, FakeVsReal, and invariant-naming contract tests; unknown and malformed messages preserve state and enter bounded diagnostics.

## Consequences

- Protocol drift is visible and testable.
- Runtime does not depend on a nonexistent handshake.
- Inventory updates accompany Codex upgrades.

## Alternatives considered

### Connection-time version inventory

Rejected: No verified authoritative runtime source exists.

### Default no-op switch

Rejected: Silently accepts drift.
