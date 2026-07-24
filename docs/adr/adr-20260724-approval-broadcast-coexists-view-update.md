---
id: adr-20260724-approval-broadcast-coexists-view-update
kind: adr
title: 'EvtApproval*/EvtQuestion* frames coexist with viewUpdate on the same subscription surface (extension, not new surface)'
status: accepted
created: 2026-07-24
summary: 'EvtApproval*/EvtQuestion* frames coexist with viewUpdate on the same subscription surface (extension, not new surface)'
decision_makers:
  - agent-grid-maintainers
  - product-owner
consulted:
  - runtime-maintainers
  - server-api-maintainers
  - contract-layer-maintainers
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
    - New frames land under an already-tested envelope; no new subscribe/unsubscribe negotiation is required.
    - Fixture pipeline (adr-20260705-wire-fixtures-pipeline) is the natural regression seam for the new frames.
  negative:
    - 'Adds two new k values to the discriminated union, growing the switch in wire.go''s encodeServerEvent and the browser socket decoder.'
  neutral:
    - ADR-0023 remains superseded by ADR-20260705 for sessions-only scope; this ADR only extends the value set of k, not the frame shape.
---


# EvtApproval*/EvtQuestion* frames coexist with viewUpdate on the same subscription surface (extension, not new surface)

## Context

{% context %}
ADR-0023 pins viewUpdate as a discriminated-union k=... frame; ADR-20260705 supersedes it for the activeSessionID scope but keeps the shape. New EvtApproval*/EvtQuestion* broadcasts must not silently escape ADR-0023's pinned shape (issue-d2-adr-0023-omission-for-approval-broadcast) and must not force a new subscription negotiation on legacy clients.
{% /context %}

## Decision

{% decision %}
New EvtApproval*/EvtQuestion* WS frames are emitted as additional k=... members of the same discriminated-union that already carries v/tt/et/n, on the same subscription surface. The frames are session-scoped following adr-20260705-view-update-sessions-only. Legacy clients that only recognize existing k values MUST ignore unknown k values without disconnecting, matching the wire-fixtures pipeline's forward-compat discipline.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- New frames land under an already-tested envelope; no new subscribe/unsubscribe negotiation is required.
- Fixture pipeline (adr-20260705-wire-fixtures-pipeline) is the natural regression seam for the new frames.
{% /consequence %}

{% consequence kind="negative" %}
- Adds two new k values to the discriminated union, growing the switch in wire.go's encodeServerEvent and the browser socket decoder.
{% /consequence %}

{% consequence kind="neutral" %}
- ADR-0023 remains superseded by ADR-20260705 for sessions-only scope; this ADR only extends the value set of k, not the frame shape.
{% /consequence %}

## Alternatives

- **New dedicated subscription surface for approval/question frames** — Legacy clients would need to opt in; increases the surface area of the subscription protocol without solving a real problem.
- **Reuse EvtSessionsChanged / viewUpdate to carry approval state** — Overloads a broadcast whose sole purpose is session-list changes; contradicts the sessions-only scope pinned by adr-20260705.

## Related

- decision inputs: `decision-input-broadcast-shape-approval-coexistence`, `decision-input-question-answer-wire-shape`, `decision-input-reconnect-extension-scope`, `decision-input-transport-agnostic-envelope`, `decision-input-compatibility-ci-gate-mechanism`
- requirements: `FR-P1-12`, `FR-P0-02`
- contracts: `contract-approval-question-envelope`
