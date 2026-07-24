---
id: adr-20260724-protocol-message-schema-sot-rest-binding
kind: adr
title: 'Message schemas are the wire SoT; REST is a binding annex, not the contract center'
status: accepted
created: '2026-07-24'
summary: 'protocol/*.schema.json (message/command/event types) is the single normative wire SoT; REST (openapi.yaml) and WS are carrier bindings of those types, and a future DataChannel transport is a third binding of the same types'
decision_makers:
  - agent-grid-maintainers
  - product-owner
consulted:
  - contract-layer-maintainers
  - server-api-maintainers
informed:
  - agent-grid-users
tags:
  - native-clients
  - phase0-1
owners:
  - agent-grid-maintainers
relations:
  - type: originatedFrom
    target: change-20260723-native-clients-phase01
source_paths:
  []
consequences:
  positive:
    - The contract survives the Phase R transport change unchanged — multi-host-gateway.md's WebRTC DataChannel/Noise data plane carries the same message types as a third binding, with no REST to port.
    - SDK generation needs only a JSON Schema consumer; no OpenAPI-generator-class toolchain is required by the contract's structure.
    - REST stays where it is genuinely superior (transcript/event-log backfill, workspace bulk reads, ticket bootstrap) without those endpoints becoming the contract's center of gravity.
  negative:
    - Two artifact kinds must stay in sync — message schemas and the REST-binding declaration — and the compatibility profile must validate both.
    - Commands ride two bindings (REST today, frames on future transports), so the command envelope must stay carrier-neutral by discipline, enforced by the round-trip fixture tests.
  neutral:
    - Commands remain on the REST binding for Phase 0/1 (matching existing server/api code); moving them onto the WS message channel for single-connection ordering with the event stream is a recorded revisit, not part of this decision.
---


# Message schemas are the wire SoT; REST is a binding annex, not the contract center

## Context

{% context %}
The contract layer carries four conversation kinds: (1) server-push events (viewUpdate, EvtApproval*/EvtQuestion*, output frames) — the supervision loop's dominant surface, already WS; (2) a small set of client commands (session create/delete, input push, approval/question answering) — REST today; (3) bulk reads (transcript/event-log backfill per ADR-0025, workspace tree/file/diff) — REST with pagination/caching semantics; (4) bootstrap (ticket mint, session-config) — REST. The system's transport trajectory (multi-host-gateway.md Phase R) is pure message frames over WebRTC DataChannel + Noise, where REST does not exist. An earlier draft of this change made openapi.yaml the single generator input by inlining the JSON Schemas into components/schemas, which made OpenAPI the de-facto contract center and pulled in an OpenAPI-generator-class toolchain.
{% /context %}

## Decision

{% decision %}
protocol/*.schema.json (events, commands, approvals/questions, capabilities, deep-links, notifications — JSON Schema 2020-12) is the single normative wire SoT. REST is a carrier binding for conversations that genuinely benefit from HTTP semantics — bulk reads, bootstrap, and (for Phase 0/1) commands — declared by protocol/openapi.yaml as a binding annex that references the message schemas; openapi.yaml is not a generator input and not the contract center. WS is the binding for the event stream and shares the same message types. A future DataChannel/Noise transport binds the same types as a third carrier without contract change. The compatibility CI drift gate treats both artifact kinds as declared surface: message schemas for typed payloads, openapi.yaml for REST routes.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The contract survives the Phase R transport change unchanged — the DataChannel data plane carries the same message types as a third binding, with no REST to port.
- SDK generation needs only a JSON Schema consumer; no OpenAPI-generator-class toolchain is required by the contract's structure.
- REST stays where it is genuinely superior (backfill, workspace bulk reads, ticket bootstrap) without becoming the contract's center of gravity.
{% /consequence %}

{% consequence kind="negative" %}
- Two artifact kinds must stay in sync — message schemas and the REST-binding declaration — and the compatibility profile must validate both.
- Commands ride two bindings (REST today, frames later), so the command envelope must stay carrier-neutral by discipline, enforced by round-trip fixture tests.
{% /consequence %}

{% consequence kind="neutral" %}
- Commands remain on the REST binding for Phase 0/1 (matching existing server/api code); moving them onto the WS message channel for single-connection ordering with the event stream is a recorded revisit, not part of this decision.
{% /consequence %}

## Alternatives

- **OpenAPI as contract center (openapi.yaml embeds all schemas; single generator input)** — Rejected: overweights the REST annex when the dominant surface (events) and the transport trajectory (DataChannel frames) are message-oriented; imposes an OpenAPI-generator toolchain on every consumer of the contract.
- **Drop openapi.yaml entirely; document REST routes in prose or a custom manifest** — Rejected: the compatibility drift gate needs a machine-readable declaration of the REST surface, and a custom manifest re-invents a worse OpenAPI. A small openapi.yaml is the cheapest standard REST declaration.
- **Move all commands onto the WS message channel now (REST only for bulk reads/bootstrap)** — Deferred: gives single-connection ordering with the event stream (attractive for first-writer-wins UX), but changes existing server/api command routes and clients/ui call sites beyond this change's scope. Recorded as a revisit; the carrier-neutral command envelope keeps it possible without schema change.

## Related

- decision inputs: `decision-input-protocol-schema-tooling`, `decision-input-transport-agnostic-envelope`
- requirements: `FR-P1-01`, `FR-P1-05`
- contracts: `contract-sdk-generation-determinism`, `contract-compatibility-ci-drift-gate`
