---
id: adr-20260803-resilient-fleet-health-single-authority
kind: adr
title: Project fleet health from the application-scoped FleetRuntime authority
status: accepted
created: '2026-08-03'
decision_makers:
- agent-grid-maintainers
tags:
- fleet-runtime
- health
- client-host
owners:
- client-host-maintainers
relations:
- {type: originatedFrom, target: change-20260727-resilient-server-reconnect-health}
- {type: references, target: adr-20260727-application-scoped-fleet-session-authority}
- {type: references, target: adr-20260727-host-neutral-fleet-provider}
- {type: references, target: adr-20260727-server-qualified-identity-partial-failure}
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: references, target: adr-20260727-single-user-server-auth}
source_paths:
- clients/ui/src/fleet
- clients/workspace/src/main
- clients/web-host/internal/webhost
summary: The existing one-per-host FleetRuntime is the sole host health writer; adapters
  may derive delivery-liveness overlays but cannot mint health revisions.
consequences:
  positive:
  - Membership and health share one host generation and publication authority.
  - Delivery failure remains observable without creating a renderer or settings health
    writer.
  - Web and Workspace consume one credential-free qualified observation contract.
  negative:
  - Both host codecs and the SPA consumer must migrate together with canonical fixtures.
  - Host runtimes must own freshness deadlines, bounded refresh, and resource teardown.
  neutral:
  - Workspace IPC and Web same-origin delivery remain different transports.
confirmation: Run canonical cross-codec fixtures, single-writer/freshness/resource
  tests, delivery-loss overlay tests, and FakeVsRealFleetHealth conformance in the
  owning packages.
updated: '2026-08-03'
---

# Project fleet health from the application-scoped FleetRuntime authority

## Context

The accepted application-scoped fleet authority already establishes one `FleetRuntime` per Web or Workspace host as the only writer of fleet membership, member generation/freshness, and subscriber publication. Health diagnosis adds new persistent questions that decision does not answer: the qualified health data model, health freshness and failure ownership, versioned host-neutral delivery, and what a consumer may show when delivery itself is broken. Leaving these questions to renderer polling or a separate Settings monitor would recreate the split authority the accepted ADR removed.

## Decision

The existing application-scoped `FleetRuntime` is also the sole producer of configured-member health and the sole writer of host health revisions. This ADR adds no runtime and no second authority; it complements and depends on `adr-20260727-application-scoped-fleet-session-authority`.

Every host revision contains every configured stable `serverId` as one exclusive qualified observation: current Checking, Connected, Unavailable, or Incompatible protocol; Disabled without freshness; or Health unavailable with an optional nested prior observation that is stale by construction. Independent state and stale flags are forbidden. A current exact protocol mismatch is Incompatible protocol and terminates operational/reconnect work for that member until configuration or process generation changes.

Connected evidence expires after five seconds. The existing member worker performs one refresh whose capabilities, ticket, and hello phases each terminate within three seconds. Failed or silent refresh produces Health unavailable and may retain the prior observation only as nested stale evidence. Renderer count cannot multiply workers, probes, timers, or sockets.

`clients/ui/src/fleet/session-protocol.ts` owns the versioned credential-free wire schema and decoder. Workspace preload/IPC and Web Go codecs implement it and are constrained by one canonical JSON fixture suite. Host revisions are ordered by `(hostInstanceId, revision)`.

Adapter delivery liveness is a separate consumer-owned envelope. On delivery loss, the provider derives an effective Health unavailable view and may nest the last accepted host observation as stale, but it preserves the last host identity/revision and cannot mint, increment, or rewrite host health. A later valid host snapshot removes the overlay. The SPA, StatusBar, Connections surfaces, and Workspace settings cannot probe servers independently or persist health into configuration.

Raw transport errors are classified at the producer transport boundary into a closed sanitized reason enum before snapshot construction or health logging. Credential-bearing values, URLs, tickets, and raw error strings never enter the health wire, DOM, or health logs.

## Consequences

{% consequence kind="positive" %}
Membership and health have one host generation/publication authority, while delivery loss remains visible through a non-authoritative overlay.
{% /consequence %}

{% consequence kind="negative" %}
Both hosts, canonical fixtures, provider codecs, and SPA consumers require one flag-day source migration; there is no renderer-probe fallback.
{% /consequence %}

{% consequence kind="neutral" %}
Workspace and Web retain IPC versus same-origin transports, and active terminal channels remain separately routed.
{% /consequence %}

## Alternatives

- Extend the accepted fleet-session ADR in place was rejected because qualified health, bounded freshness, delivery overlays, and typed redaction are a distinct persistent failure contract; silently broadening an accepted historical decision would obscure why these responsibilities were added.
- Renderer or Settings polling was rejected because it creates competing health writers and probe work proportional to views.
- Let the provider advance a health revision on delivery loss was rejected because a disconnected consumer cannot authoritatively revise host evidence.
- Persist health in `servers.json` was rejected because runtime evidence and credential-bearing configuration have different owners and lifetimes.

## Confirmation

Run shared canonical fixture codec tests in TypeScript, Workspace, and Go; fake-clock freshness/phase-timeout tests; single-writer/resource shutdown tests; adapter close/reconnect overlay tests; secret-sentinel wire/DOM/log tests; and the opt-in `FakeVsRealFleetHealth*` backstop.


{% transition from="proposed" to="accepted" date="2026-08-03" %}
Approved by user for plan delivery on 2026-08-03.
{% /transition %}
