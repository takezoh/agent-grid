---
change: change-20260727-client-server-protocol-version
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- `FR-PROTO-001` (ubiquitous): The client-server protocol shall have one build-time opaque token whose wire domain is one header value of 1–64 visible ASCII bytes matching `[A-Za-z0-9._-]+`; compatibility shall mean exact byte equality only.
- `FR-PROTO-002` (event-driven): When an authenticated protected request, or an explicitly unauthenticated route request, enters `/api/*`, the server shall validate the version before dispatch; only an exact match may reach the operational handler.
- `FR-PROTO-003` (unwanted): If credentials are invalid or missing on a protected route, the server shall preserve the existing 401/403 result without disclosing version evidence; otherwise a missing, duplicate, malformed, or unequal version shall return HTTP 426 with `protocol_version_mismatch`, client/server evidence, and zero operational side effects.
- `FR-PROTO-004` (event-driven): When a WebSocket ticket is minted, it shall be single-use and bound to the admitted version; `/ws` shall reject absent, stale, replayed, or differently bound tickets before hello or application frames.
- `FR-PROTO-005` (state-driven): While a server is observed as `incompatible_protocol`, every shipped host shall suppress operational REST, WebSocket open, reconnect/backoff, capability degradation, and fallback until a later explicit refresh or existing compatibility-probe cadence observes an exact match.
- `FR-PROTO-006` (complex): When a fleet contains matching, mismatching, and unavailable servers concurrently, each result shall remain server-qualified and latest-generation fenced, and matching servers shall remain selectable and operable.
- `FR-PROTO-007` (state-driven): While a mismatch is current, the server list shall show the current display label, typed cause, client version, and server version, using explicit missing/invalid markers rather than generic unavailable text.
- `FR-PROTO-008` (ubiquitous): Every public client-server protocol change shall bump the token and update server, all shipped hosts, generated SDKs, simulator/fakes, fixtures, and verification in one flag-day change.
- `FR-PROTO-009` (unwanted): The repository shall never introduce version ranges, SemVer compatibility, capability-based degradation, legacy/dual decoders, downgrade, retry-as-fallback, or any other cross-version compatibility path.
- `FR-PROTO-010` (ubiquitous): The exact-match/no-fallback ADR and persistent design invariant shall supersede the accepted bundled/remote capability-negotiation policy; the cross-language SDK and protocol-schema ownership ADRs remain in force.

## Non-functional requirements

- `NFR-PROTO-001` (security): Authentication precedes version disclosure on protected routes; version values are public diagnostics, never credentials or URL-query secrets.
- `NFR-PROTO-002` (performance): Validation performs one constant-bounded comparison per request admission/bootstrap; the 64-byte maximum forbids input-size-dependent unbounded work and no per-message negotiation is added.
- `NFR-PROTO-003` (reliability): Existing per-server timeout, generation/cancellation, partial-failure, credential confinement, and redirect-rejection boundaries remain unchanged.
- `NFR-PROTO-004` (maintainability): A closed inventory of public routes, shipped clients, decoders, retry decision sites, SDK projections, fixtures, and simulator surfaces fails CI when incomplete or when any surface bypasses the exact-equality chokepoint.
- `NFR-PROTO-005` (compatibility): Old-client/new-server and new-client/old-server pairings both fail with the same typed mismatch; there is no rollout or rollback compatibility mode.

## Acceptance

- An exact token reaches representative read/write handlers once; missing, duplicate, malformed, older, and newer values return typed 426 with handler effect count zero.
- Invalid bearer plus mismatch returns the existing auth failure and no version disclosure; valid bearer plus mismatch returns typed 426.
- A matching ticket opens WS once; replay, stale binding, mismatch, and direct no-ticket access produce no application frame.
- Web, Workspace, and Windows recorded scenarios classify the same server evidence identically, perform zero operational/reconnect attempts after mismatch, and preserve a matching peer.
- Server-list tests show current label and version evidence or explicit markers and prove mismatch is not rendered as unavailable.
- Mutation fixtures for range comparison, dual decoder, capability hide/disable, renamed fallback retry, and an unregistered route/client/decoder/retry site each fail the compatibility gate.

## Non-goals

- Mixed-version availability, rolling rollout, user version overrides, or compatibility rescue.
- Versioning internal daemon IPC, agent-specific protocols, or orchestrator protocols.

