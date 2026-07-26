---
id: adr-20260726-status-wire-flag-day-replacement
kind: adr
title: Replace the legacy scalar status wire in one atomic flag-day cutover
status: accepted
created: '2026-07-26'
updated: '2026-07-26'
tags:
- adr
- session-status
owners: []
relations:
- {type: originatedFrom, target: change-20260726-unified-driver-session-status}
source_paths:
- clients/ui
- clients/windows-shell
- clients/workspace
- protocol
- src/host/driver
- src/host/runtime/subsystem
- src/host/state
- src/server/api
decision_makers:
- user
summary: Replace the legacy scalar status and fallback with the composite-only wire, remove Codex OSC status inference, and regenerate all SDKs and clients atomically.
---

# Replace the legacy scalar status wire in one atomic flag-day cutover

## Context

The user explicitly rejects backward compatibility and fallback. Keeping a scalar projection, legacy event fallback, or OSC status inference would preserve multiple authorities and allow clients to observe divergent status semantics.

## Decision

Replace the legacy scalar status with the composite status wire as a breaking flag-day change. In the same landed change:

- remove the legacy scalar field;
- remove legacy event fallback;
- remove Codex Driver OSC 0/2, BEL, and OSC notification status inference while retaining other Drivers' existing observation contracts;
- regenerate every SDK; and
- update Web, Workspace, and Windows Shell atomically.

Mixed-version operation and runtime rollback fallback are unsupported. A pre-release rollback reverts the whole change revision as a unit.

## Consequences

- Every in-repository consumer moves to the composite contract in one commit.
- Old clients and SDKs are intentionally incompatible with the new server wire.
- Repository-wide generation, compile, contract, zero-legacy-search, and client parity gates prevent a partial cutover.

## Alternatives considered

### Temporary scalar projection and mixed-version rollout

Rejected by explicit user consultation because it retains legacy authority and fallback behavior. The rejected decision remains recorded in [adr-20260726-status-wire-migration](adr-20260726-status-wire-migration.md).

### Composite wire with legacy event or OSC inference fallback

Rejected because it would split status authority and permit old observations to repopulate the replacement model.

## Status provenance

- source: `user_consultation`
- evidence: `user:2026-07-26-no-backward-compat`, `consultation-unified-status-adrs`
- rationale: The user explicitly selected a flag-day composite replacement with no backward compatibility or fallback.
