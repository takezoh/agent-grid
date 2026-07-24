---
id: adr-20260724-boundary-2-token-unc-fresh-read
kind: adr
title: Bearer token is UNC-read per connection attempt; no Windows-side cache
status: accepted
created: '2026-07-24'
summary: Bearer token is UNC-read per connection attempt; no Windows-side cache
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - No cache-invalidation policy to maintain; rotation race is closed by construction.
  - Explicit failure semantics replace silent Connected-looking states.
  negative:
  - Extra \\wsl$ round-trip at connection time; rare (adopt/spawn/reconnect only),
    not per-request.
  neutral:
  - Aligns with adr-20260724-boundary-2-native-ws-auth-reuse-ticket which also reuses
    per-attempt REST rather than long-lived sessions.
---
# Bearer token is UNC-read per connection attempt; no Windows-side cache

## Context

{% context %}
Boundary 2 authentication needs to handle token rotation and transient UNC unreachability. Options: read UNC fresh per attempt vs cache with invalidation policy.
{% /context %}

## Decision

{% decision %}
Read \\wsl$\<distro>\home\<user>\.agent-grid\gateway-token fresh on every daemon connection attempt (contract-b2-token-acquisition). A 401 triggers an immediate re-read. UNC unreadable surfaces as an explicit failure state; no unauthenticated fallback.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- No cache-invalidation policy to maintain; rotation race is closed by construction.
- Explicit failure semantics replace silent Connected-looking states.
{% /consequence %}

{% consequence kind="negative" %}
- Extra \\wsl$ round-trip at connection time; rare (adopt/spawn/reconnect only), not per-request.
{% /consequence %}

{% consequence kind="neutral" %}
- Aligns with adr-20260724-boundary-2-native-ws-auth-reuse-ticket which also reuses per-attempt REST rather than long-lived sessions.
{% /consequence %}

## Alternatives

- **Windows-side cached token with periodic refresh** — Reintroduces staleness/rotation race that fresh reads close by construction.

## Related

- decision inputs: (none)
- requirements: `FR-B2-01`, `FR-B2-03`
- contracts: `contract-b2-token-acquisition`
- change: `change-20260723-windows-shell-phase2`
