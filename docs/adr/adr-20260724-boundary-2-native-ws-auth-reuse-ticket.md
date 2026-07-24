---
id: adr-20260724-boundary-2-native-ws-auth-reuse-ticket
kind: adr
title: Native clients reuse the existing REST-bearer→mint-ticket→/ws?ticket= flow;
  the gateway is NOT extended to accept an Authorization header on WS upgrade
status: accepted
created: '2026-07-24'
summary: Native clients reuse the existing REST-bearer→mint-ticket→/ws?ticket= flow;
  the gateway is NOT extended to accept an Authorization header on WS upgrade
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
  - Zero server/api change; auth_test.go stays unchanged.
  - Single documented native connect path across C#/TS SDKs.
  negative:
  - One extra REST round-trip per WS open; acceptable against loopback and rare enough
    (adopt/reconnect) not to be worth a new server branch.
  neutral:
  - 'adr-20260724-stdlib-only-go-wire is preserved: Go wire types are not extended
    for native SDKs.'
---
# Native clients reuse the existing REST-bearer→mint-ticket→/ws?ticket= flow; the gateway is NOT extended to accept an Authorization header on WS upgrade

## Context

{% context %}
src/server/api/mux.go's GET /ws today has no header-auth branch (auth.go's own comment documents that the ticket flow exists because browser WS cannot set headers). Native clients could set headers; the choice is deliberate.
{% /context %}

## Decision

{% decision %}
Native clients follow the same two-step mint→connect flow as browsers. contract-b2-native-ws-auth-path enforces this at the SDK-vs-real-gateway fidelity backstop. No server/api change ships in Phase 2; a future header-auth branch would supersede this ADR via a compatibility-policy.md entry.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Zero server/api change; auth_test.go stays unchanged.
- Single documented native connect path across C#/TS SDKs.
{% /consequence %}

{% consequence kind="negative" %}
- One extra REST round-trip per WS open; acceptable against loopback and rare enough (adopt/reconnect) not to be worth a new server branch.
{% /consequence %}

{% consequence kind="neutral" %}
- adr-20260724-stdlib-only-go-wire is preserved: Go wire types are not extended for native SDKs.
{% /consequence %}

## Alternatives

- **Extend GET /ws to accept an Authorization header** — Requires server/api change, auth_test.go branch, and a compatibility-policy.md entry — for a round-trip cost not worth paying in Phase 2.

## Related

- decision inputs: `decision-input-stdlib-only-go-wire`, `decision-input-cross-language-sdk-strategy`
- requirements: `FR-B2-02`
- contracts: `contract-b2-native-ws-auth-path`
- change: `change-20260723-windows-shell-phase2`
