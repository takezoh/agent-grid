---
id: adr-20260728-client-host-github-device-flow
kind: adr
title: Client-host GitHub device-flow account and catalog authority
status: accepted
created: '2026-07-28'
decision_makers:
- pending user consultation
consulted:
- design integration critique
informed:
- client host, shared SPA, and platform implementers
tags:
- github
- client-host
- oauth
- catalog
owners: []
relations:
- {type: originatedFrom, target: change-20260728-github-repository-sessions}
- {type: references, target: adr-20260727-host-neutral-fleet-provider}
- {type: references, target: adr-20260727-single-user-server-auth}
- {type: references, target: adr-20260727-application-scoped-fleet-session-authority}
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- clients/web-host/
- clients/workspace/
- clients/ui/
summary: Choose host-owned GitHub OAuth device flow, protected token persistence,
  retained catalog, and credential-free projection pending consultation.
consequences:
  positive:
  - One credential-safe account/catalog behavior is shared across browser and Electron.
  negative:
  - A dedicated OAuth App registration and bounded catalog runtime must be operated
    in each host.
  neutral:
  - The SPA continues to consume only credential-free provider state.
confirmation: GitHubAuthContract and GitHubCatalogContract produce deep-equal Web/Workspace
  projections; configured FakeVsReal backstops pass or explicitly skip.
updated: '2026-07-28'
---

## Status

Proposed. This decision is intentionally pending user consultation and does not authorize implementation until accepted or rejected.

## Context

Browser and Electron require one credential-safe account/catalog capability without callback or installation authority divergence.

## Decision

Pending consultation, use a GitHub OAuth App device flow with fixed public client_id and scopes. Each host owns protected credential persistence/refresh and a bounded validated complete-generation catalog, while the SPA receives only credential-free status, repositories, and a discriminated target projection.

## Consequences

{% consequence kind="positive" %}
One credential-safe account/catalog behavior is shared across browser and Electron.
{% /consequence %}

{% consequence kind="negative" %}
A dedicated OAuth App registration and bounded catalog runtime must be operated in each host.
{% /consequence %}

{% consequence kind="neutral" %}
The SPA continues to consume only credential-free provider state.
{% /consequence %}

## Alternatives

- GitHub App installation/web callback: Adds installation/admin and callback authority states not present in the UX.
- Ambient gh authentication: Makes credential authority and availability host-environment dependent.
- Browser callback plus Electron custom protocol: Creates two security and recovery state machines.

## Confirmation

GitHubAuthContract and GitHubCatalogContract produce deep-equal Web/Workspace projections; configured FakeVsReal backstops pass or explicitly skip.
