---
id: adr-20260727-web-host-transport-continuity
kind: adr
title: Server-qualified Web host transport continuity
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags:
- web-host
- security
- websocket
- migration
owners: []
relations:
- {type: originatedFrom, target: change-20260727-web-host-client-boundary}
source_paths:
- clients/web-host
- clients/ui
summary: Preserve same-origin and security semantics through server-qualified Web
  host routes.
consequences:
  positive:
  - CSP, origin checks, credential confinement and typed retry remain at one Web trust
    boundary.
  negative:
  - Browser route paths change in the same flag day as the UI provider.
  neutral:
  - Backend gateways retain their existing /api and /ws contracts behind the prefix-stripping
    proxy.
confirmation: HTTP/WS contract tests cover qualified routing, origin, redirect, credentials
  and retry classes.
---

## Context

An unqualified `/api` or `/ws` necessarily selects an implicit single server. Keeping it as a default would preserve the forbidden Web special case. At the same time, current embed, CSP/security headers, same-origin WebSocket gate and retryable upstream failure are observable contracts.

## Decision

Expose credential-free `/host/config` and qualified `/host/servers/{serverId}/api/...` plus `/host/servers/{serverId}/ws`. Web host resolves `serverId` against its immutable snapshot, strips the host prefix, injects the bound credential, rejects cross-target redirects/downgrades, and proxies to the selected gateway.

All responses retain defensive headers. WebSocket Origin must match the Web host when present. Retryable refusal/unreachable/DNS classes remain `503` with `Retry-After: 1`. Legacy unqualified `/api` and `/ws` provider routes are removed in the flag day.

## Consequences

{% consequence kind="positive" %}
Same-origin browser security remains while multi-server selection is explicit.
{% /consequence %}

{% consequence kind="negative" %}
Web clients and tests must cut over atomically to qualified routes.
{% /consequence %}

{% consequence kind="neutral" %}
Upstream server API/WS paths do not change.
{% /consequence %}

## Alternatives

- Keep unqualified routes as default-server aliases: rejected because this is single-server synthesis.
- Browser direct-to-gateway requests: rejected because credential and origin policy would escape the host boundary.

## Confirmation

Run clients/web-host httptest and clients/ui real-stack E2E for qualified REST/WS, cross-origin rejection, redirect rejection, token omission and retryable 503.
