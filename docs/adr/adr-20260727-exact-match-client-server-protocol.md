---
id: adr-20260727-exact-match-client-server-protocol
kind: adr
title: Require exact-match client-server protocol versions and forbid compatibility
  fallback
status: accepted
created: '2026-07-27'
summary: Client and server protocol versions must match exactly; every mismatch fails
  closed and no cross-version compatibility, capability degradation, downgrade, or
  fallback is permitted.
decision_makers:
- agent-grid-maintainers
- product-owner
consulted:
- server-api-maintainers
- client-host-maintainers
- contract-layer-maintainers
informed:
- agent-grid-users
tags:
- protocol
- client-server
- compatibility
owners:
- agent-grid-maintainers
relations:
- {type: supersedes, target: adr-20260724-capability-negotiation-bundled-remote-two-axis}
- {type: originatedFrom, target: change-20260727-client-server-protocol-version}
source_paths:
- protocol
- services/server/api
- clients/ui
- clients/web-host
- clients/workspace
- clients/windows-shell
- clients/sdk
- contracts/compatibility-policy.md
consequences:
  positive:
  - Compatibility has one deterministic meaning across every carrier and host.
  - Version skew is diagnosed per server before operational side effects.
  - Future changes cannot justify hidden degradation or fallback from an older policy.
  negative:
  - Every public protocol change requires an atomic server, client, SDK, fixture,
    simulator, and verification update.
  - Mixed-version rollout and rollback availability are intentionally unavailable.
  neutral:
  - Protocol version values are bounded public diagnostics rather than credentials.
  - Existing cross-language SDK generation and protocol-schema ownership decisions
    remain in force.
updated: '2026-07-27'
---

# Require exact-match client-server protocol versions and forbid compatibility fallback

## Context

{% context %}
The server currently advertises a protocol version but does not use it as an admission boundary. The accepted bundled/remote capability-negotiation ADR permits a remote version-skew branch that degrades missing capabilities and treats a missing version as lowest capability. That policy directly conflicts with the explicit requirement that every different, missing, or malformed client-server version be an error and that compatibility and fallback never be implemented.
{% /context %}

## Decision

{% decision %}
The public client-server protocol has one bounded opaque build-time token. The wire value is one 1–64 byte visible ASCII string matching `[A-Za-z0-9._-]+`; compatibility means exact byte equality and nothing else.

Protected REST routes authenticate before version disclosure, then apply one common version gate before operational handlers. Explicitly unauthenticated routes apply the gate directly. Missing, duplicate, malformed, or unequal versions fail with typed HTTP 426 evidence and no operational side effect. WebSocket admission inherits the exact-match decision through a single-use version-bound ticket before any application frame.

All shipped client hosts preserve mismatch as server-qualified `incompatible_protocol`, stop operational/reconnect work for that observation, preserve healthy peers, and expose client/server version evidence in server lists.

Every public protocol evolution is a breaking flag-day change that bumps the token and atomically updates the server, every shipped host, generated SDKs, simulator/fakes, fixtures, and verification. Version ranges, SemVer compatibility, capability-based degradation, legacy or dual decoders, downgrade, retry-as-fallback, user overrides, and all other cross-version success paths are forbidden.

This ADR supersedes `adr-20260724-capability-negotiation-bundled-remote-two-axis` in full. Its bundled/remote split, version-skew degradation, missing-version lowest-capability behavior, and additive compatibility policy no longer apply. `adr-20260724-protocol-message-schema-sot-rest-binding`, `adr-20260724-protocol-cross-language-sdks-supersedes-0021`, and the generated SDK decisions remain applicable as ownership/tooling constraints, not compatibility authorities.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Compatibility has one deterministic meaning across every carrier and host.
- Version skew is diagnosed per server before operational side effects.
- Future changes cannot justify hidden degradation or fallback from an older policy.
{% /consequence %}

{% consequence kind="negative" %}
- Every public protocol change requires an atomic server, client, SDK, fixture, simulator, and verification update.
- Mixed-version rollout and rollback availability are intentionally unavailable.
{% /consequence %}

{% consequence kind="neutral" %}
- Protocol version values are bounded public diagnostics rather than credentials.
- Existing cross-language SDK generation and protocol-schema ownership decisions remain in force.
{% /consequence %}

## Alternatives

- **Retain bundled/remote capability degradation** — rejected because it permits successful operation across different or missing versions and directly violates the permanent no-fallback requirement.
- **Use SemVer or version ranges** — rejected because comparison would no longer be opaque exact equality and would create a compatibility policy.
- **Rely on client preflight or WebSocket hello** — rejected because forgotten consumers and earlier REST/WS side effects remain reachable.
- **Use advisory documentation or word scanning only** — rejected because renamed retry/fallback logic and unregistered surfaces would evade the contract.

## Related

- requirements: `FR-PROTO-001` through `FR-PROTO-010`
- contracts: `contract-version-and-evolution`, `contract-server-admission`, `contract-client-continuity`, `contract-fleet-observability`, `contract-repository-enforcement`
- decision inputs: `decision-input-exact-version-only`, `decision-input-no-compatibility-fallback`, `decision-input-conflicting-capability-adr`
