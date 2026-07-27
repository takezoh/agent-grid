---
id: change-20260727-client-server-protocol-version
kind: change
title: Client-server protocol version contract
status: active
created: '2026-07-27'
profile: sdd@1
intent: Client and server admit only an exact shared protocol version, expose every
  mismatch per server, and permanently forbid cross-version compatibility or fallback.
outcomes:
- Every public REST and WebSocket carrier rejects missing, malformed, or unequal protocol
  versions before operational side effects.
- Every shipped client host preserves a typed incompatible_protocol outcome and the
  server list identifies the server and both version values or their missing/invalid
  markers.
- Protocol evolution is flag-day only; compatibility ranges, downgrade, capability
  degradation, dual decoders, and fallback are prohibited by durable policy and mechanical
  gates.
scope:
- protocol version manifest, generated language projections, OpenAPI binding, fixtures,
  simulator, and SDK conformance
- server /api admission, authentication precedence, and version-bound WebSocket tickets
- Web host, Workspace Electron, Windows Shell, and shared clients/ui fleet outcomes
  and presentation
- supersession of the bundled/remote capability negotiation policy and promotion of
  an exact-match invariant
non_goals:
- Cross-version conversion, compatibility ranges, feature negotiation, downgrade,
  fallback, or rolling mixed-version availability
- User-configurable expected versions or servers.json version overrides
- Versioning daemon Unix-socket IPC, agent protocols, or orchestrator protocols
change_classes:
- behavior
- boundary
- invariant
- capability
governance:
  gate: hard
  reasons:
  - Changes the public client-server admission and failure contract across every shipped
    host.
  - Permanently supersedes an accepted compatibility policy.
members:
- role: requirements
  path: changes/change-20260727-client-server-protocol-version/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-client-server-protocol-version/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-client-server-protocol-version/verification.md
  required: true
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-CLIENT-SERVER-PROTOCOL-EXACT-MATCH
    statement: Client and server public protocol versions are one opaque build-time
      token and compatibility means exact equality only; missing, malformed, or unequal
      versions fail closed, and cross-version compatibility, capability degradation,
      downgrade, dual decoding, and fallback are forbidden.
    enforcement: contract
  reason: The user explicitly requires a permanent no-compatibility/no-fallback contract
    for all future client-server protocol changes.
- target: design-client
  section: boundaries.provides
  action: upsert
  item:
    id: BOUNDARY-PROTOCOL-MISMATCH-OUTCOME
    statement: Every client host projects an authenticated exact-version admission
      failure as server-qualified incompatible_protocol evidence while preserving
      healthy peers.
  reason: The failure cause must remain observable in every server list without host-specific
    semantic drift.
unresolved_decisions: []
tags:
- protocol
- client-server
- compatibility
owners:
- agent-grid-maintainers
relations:
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: modifies, target: design-client}
source_paths:
- protocol
- services/server/api
- clients/ui
- clients/web-host
- clients/workspace
- clients/windows-shell
- clients/sdk
- contracts/compatibility-policy.md
summary: Define one exact client-server protocol version, reject mismatches without
  compatibility or fallback, and expose the cause in server lists.
updated: '2026-07-27'
---

## Summary

This change introduces one bounded opaque protocol token shared at build time by the server and every shipped client. Exact equality is the only successful compatibility result. Authentication keeps precedence on protected routes; after authentication (or on explicitly unauthenticated routes), missing, malformed, or unequal versions produce a typed `protocol_version_mismatch` response before handler or WebSocket application side effects.

The client fleet read model retains that response as `incompatible_protocol`, keyed by server and observation generation. Server lists show the current label plus client/server version evidence or explicit missing/invalid markers, while healthy peers remain usable. The new ADR supersedes the former version-skew capability-degradation policy, and the promotion manifest makes flag-day-only evolution a persistent design invariant.

## Closure Notes
