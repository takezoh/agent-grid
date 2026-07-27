---
id: adr-20260727-host-neutral-fleet-provider
kind: adr
title: Host-neutral configured fleet provider
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags:
- client-host
- fleet-provider
- security
owners: []
relations:
- {type: originatedFrom, target: change-20260727-web-host-client-boundary}
- {type: supersedes, target: adr-20260727-fleet-provider-boundary}
source_paths:
- clients/desktop-config
- clients/ui
- clients/workspace
- clients/web-host
summary: Generalize configured fleet/provider semantics across Web host and Workspace
  while retaining host-owned credentials.
consequences:
  positive:
  - Web host and Workspace expose one server-qualified provider contract and partial-failure
    semantics.
  negative:
  - Web host becomes a credentialed multi-server router and must own target/redirect/cancellation
    policy.
  neutral:
  - Host delivery remains HTTP/WS for Web and preload/IPC for Workspace.
confirmation: Shared multi-server fixtures produce deep-equal credential-free projections
  for both hosts.
---

## Context

`adr-20260727-fleet-provider-boundary` correctly keeps Electron credentials in main, but its browser one-member adapter makes fleet cardinality and policy depend on host kind. The new requirement forbids that special case while `adr-20260727-server-qualified-identity-partial-failure` and `adr-20260727-single-user-server-auth` remain authoritative.

## Decision

Every clients/ui host loads `servers.json` schema version 2 once at startup and owns one immutable credential-bearing snapshot. It exposes the same credential-free, server-qualified FleetProvider projection. Every operation names `serverId`; healthy results survive another server's typed failure; retry/generation/cancellation semantics are host-neutral.

Only transport differs: Web uses qualified same-origin HTTP/WS through Web host; Workspace uses preload/IPC through Electron main. Runtime reload is not supported.

This supersedes only the browser one-server/default-origin clause of `adr-20260727-fleet-provider-boundary`; its credential confinement, redirect rejection and lifecycle ownership are generalized to every host.

## Consequences

{% consequence kind="positive" %}
The SPA cannot encode browser/Electron cardinality or policy differences.
{% /consequence %}

{% consequence kind="negative" %}
Web host must implement fleet routing and credential isolation, not a transparent single upstream proxy.
{% /consequence %}

{% consequence kind="neutral" %}
Electron main and Web host remain separate transport adapters.
{% /consequence %}

## Alternatives

- Keep browser one-server adapter: rejected because it is the explicitly forbidden behavior.
- Expose tokens to clients/ui so it can route directly: rejected because it violates the accepted trust boundary.
- Reload config at runtime: rejected because it adds mixed-version lifecycle semantics not required here.

## Confirmation

Run the shared FleetProvider conformance suite against Web and Workspace adapters with a two-server bearer fixture; projections must be equal and contain no bearer material.
