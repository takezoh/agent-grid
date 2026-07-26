---
id: adr-20260726-composite-session-status
kind: adr
title: Use a valid composite status model with presentation precedence
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
summary: Own tagged lifecycle/failure and pending-request IDs in host/state. Derive ! from a non-empty pending set, choose ! over spinner, keep failure distinct, and treat signal as orthogonal.
---

# Use a valid composite status model with presentation precedence

## Context

Activity, unresolved human input, failure, and signal have different lifecycles; a scalar loses observable combinations.

## Decision

Own tagged lifecycle/failure and pending-request IDs in host/state. Derive ! from a non-empty pending set, choose ! over spinner, keep failure distinct, and treat signal as orthogonal.

## Consequences

- Spinner and ! are structurally exclusive.
- Bell and ! may coexist.
- Invalid internal combinations fail fast.

## Alternatives considered

### Single scalar status

Rejected: Cannot preserve concurrent signal and human-input semantics.
