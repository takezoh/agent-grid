---
id: adr-20260728-durable-repository-preparation
kind: adr
title: Durable repository preparation before Session publication
status: accepted
created: '2026-07-28'
decision_makers:
- pending user consultation
consulted:
- design integration critique
informed:
- host state/runtime, gateway, and client host implementers
tags:
- github
- session-lifecycle
- reliability
owners: []
relations:
- {type: originatedFrom, target: change-20260728-github-repository-sessions}
- {type: references, target: adr-20260727-application-scoped-fleet-session-authority}
- {type: references, target: adr-20260727-server-qualified-identity-partial-failure}
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- services/host/state/
- services/host/runtime/
- services/server/api/
- protocol/
summary: Own idempotent clone and environment preparation as durable server state
  before publishing a Session, pending consultation.
consequences:
  positive:
  - Partial work is restart-safe and structurally absent from available Sessions.
  negative:
  - A new persisted aggregate, reconciliation algorithm, capacity policy, and protocol
    surface are required.
  neutral:
  - Completed session workspaces remain governed by existing explicit lifecycle rather
    than automatic preparation cleanup.
confirmation: Reducer/runtimetest/crash matrices prove one intent, one availability
  commit, bounded resources, and typed recovery across restart/cancel/retry.
updated: '2026-07-28'
---

## Status

Proposed. This decision is intentionally pending user consultation and does not authorize implementation until accepted or rejected.

## Context

Clone and environment work outlive requests, may crash or be cancelled, and must not expose a partial Session or duplicate clone.

## Decision

Pending consultation, a host-persisted creation intent maps to a server-owned durable RepositoryPreparation. The reducer owns revision-fenced transitions, bounded admission, durable clone publication/reconciliation, retry/cancel/abandon, progress snapshots, and the only commit into an available Session.

## Consequences

{% consequence kind="positive" %}
Partial work is restart-safe and structurally absent from available Sessions.
{% /consequence %}

{% consequence kind="negative" %}
A new persisted aggregate, reconciliation algorithm, capacity policy, and protocol surface are required.
{% /consequence %}

{% consequence kind="neutral" %}
Completed session workspaces remain governed by existing explicit lifecycle rather than automatic preparation cleanup.
{% /consequence %}

## Alternatives

- Publish a partial Session before clone: Violates the explicit unavailable-until-complete invariant.
- Keep preparation only in an HTTP goroutine: Cannot preserve retry identity or survive restart.
- Client-owned lifecycle: Conflicts with accepted server/domain authority.

## Confirmation

Reducer/runtimetest/crash matrices prove one intent, one availability commit, bounded resources, and typed recovery across restart/cancel/retry.
