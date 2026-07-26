---
id: adr-20260726-status-wire-migration
kind: adr
title: Migrate composite status with a temporary deterministic scalar projection
status: rejected
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
summary: Add composite fields server-first while retaining a deterministic scalar projection; update generated SDKs and all clients; remove legacy status only after a zero-consumer and baseline/target fixture gate.
---

# Migrate composite status with a temporary deterministic scalar projection

## Context

Three clients and generated SDKs cannot switch atomically.

## Decision

Add composite fields server-first while retaining a deterministic scalar projection; update generated SDKs and all clients; remove legacy status only after a zero-consumer and baseline/target fixture gate.

## Consequences

- Mixed versions remain parseable.
- Legacy semantics are explicit, not client-invented.
- Removal requires evidence.

## Alternatives considered

### Flag-day replacement

Rejected: Breaks mixed-version consumers.
