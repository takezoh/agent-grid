---
id: adr-20260724-approval-single-writer-first-commit
kind: adr
title: Two-client approval conflict resolves as first-committed-wins under the single-writer Reduce loop
status: accepted
created: 2026-07-24
summary: Two-client approval conflict resolves as first-committed-wins under the single-writer Reduce loop
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
    - The invariant reduces to the existing single-writer loop; no new synchronization primitive.
    - 'The loser is told ''why'' via the authoritative decision, enabling a coherent UI update instead of a silent overwrite.'
  negative:
    - Wire adds a new error kind (resolved-by-other) that every generated SDK must model.
  neutral:
    - Order within the Reduce loop is deterministic per event arrival; timing at the WS/REST boundary decides who wins, and this is treated as first-class product behavior, not a bug.
---


# Two-client approval conflict resolves as first-committed-wins under the single-writer Reduce loop

## Context

{% context %}
FR-P0-04 requires the loser of a two-client race to receive an authoritative resolved-by-other error. host/state's single-writer event loop processes events strictly one at a time (ARCHITECTURE.md), so the invariant is already available; the decision is how to expose it on the wire and how to reject client-side workarounds (optimistic lock retries).
{% /context %}

## Decision

{% decision %}
First-committed-wins under the single-writer Reduce loop: the first Reduce call for a CmdApprovalRespond commits the resolution; every subsequent Reduce call for the same ApprovalRequest ID observes status=resolved, returns state unchanged, and produces a rejection effect carrying the winning (decision, resolving_client_instance_id). Client-side optimistic locking or retries are explicitly disallowed and the wire contract does not expose a way to add them (matches plan-20260723-windows-shell-design.md §5).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The invariant reduces to the existing single-writer loop; no new synchronization primitive.
- The loser is told 'why' via the authoritative decision, enabling a coherent UI update instead of a silent overwrite.
{% /consequence %}

{% consequence kind="negative" %}
- Wire adds a new error kind (resolved-by-other) that every generated SDK must model.
{% /consequence %}

{% consequence kind="neutral" %}
- Order within the Reduce loop is deterministic per event arrival; timing at the WS/REST boundary decides who wins, and this is treated as first-class product behavior, not a bug.
{% /consequence %}

## Alternatives

- **Last-write-wins (silent overwrite)** — No precedent in host/state; produces observable inconsistency across subscribers.
- **Client-side optimistic lock + retry** — Explicitly rejected by plan-20260723-windows-shell-design.md §5.

## Related

- decision inputs: `decision-input-approval-conflict-resolution`
- requirements: `FR-P0-04`, `FR-P0-05`
- contracts: `contract-approval-resolution-single-writer`, `contract-approval-question-envelope`
