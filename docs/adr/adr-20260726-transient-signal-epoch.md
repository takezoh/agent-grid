---
id: adr-20260726-transient-signal-epoch
kind: adr
title: Identify transient signals by server epoch and monotonic sequence
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
summary: The status owner issues (serverEpoch, signalSequence). Each client keeps an in-process highest-consumed marker per session/epoch; reconnect does not replay, and client or server restart resets the consumption scope.
---

# Identify transient signals by server epoch and monotonic sequence

## Context

Snapshot replay must neither duplicate a signal on reconnect nor suppress new signals after restart.

## Decision

The status owner issues (serverEpoch, signalSequence). Each client keeps an in-process highest-consumed marker per session/epoch; reconnect does not replay, and client or server restart resets the consumption scope.

## Consequences

- Reconnect behavior is deterministic.
- No durable user configuration is introduced.
- A client restart may replay the latest still-present signal once; the signal is non-authoritative and transient.

## Alternatives considered

### Durable per-user marker

Rejected: Adds persistence and cross-device semantics outside scope.
