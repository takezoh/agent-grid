---
id: adr-20260726-driver-observation-boundaries
kind: adr
title: Normalize authoritative Driver events and keep Shell limited-capability
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
summary: Codex uses app-server exclusively and removes its OSC status inference. Claude/Gemini keep their existing driver-owned OSC title observations, Grok keeps runtime lifecycle observations, and Shell keeps OSC 133 prompt boundaries.
---

# Normalize authoritative Driver events and keep Shell limited-capability

## Context

Drivers expose different evidence; inventing a new Shell protocol or retaining title inference would split authority.

## Decision

Codex uses app-server exclusively and removes its OSC 0/2 and notification/BEL status inference. Claude and Gemini retain their existing driver-owned OSC title observations because no equivalent app-server source was introduced for them. Grok uses runtime lifecycle observations. Shell uses process lifecycle and retained OSC 133 prompt boundaries.

## Consequences

- All Drivers implement one operation contract.
- Shell reports unsupported detail as unknown.
- Claude/Gemini OSC title handling and Shell OSC 133 remain intact.

## Alternatives considered

### New Shell structured protocol

Rejected: Adds an unrequested producer/wire boundary.

### Terminal-title inference

Rejected: Is explicitly forbidden and non-authoritative.
