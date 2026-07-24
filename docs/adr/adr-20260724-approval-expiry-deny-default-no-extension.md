---
id: adr-20260724-approval-expiry-deny-default-no-extension
kind: adr
title: ApprovalRequest expiry defaults to deny; no agent-side TTL extension
status: accepted
created: 2026-07-24
summary: ApprovalRequest expiry defaults to deny; no agent-side TTL extension
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
    - Safety-by-default preserved (NFR-02); TOCTOU-free by construction.
    - Expiry reducer is a total function of (state, tick), matching the FC/IS invariant.
    - 'Extension policy is fixed: no agent-side surprise; operators always see a bounded pending state.'
  negative:
    - A legitimate long-running approval cannot ask for more time; the driver must re-emit a fresh approval request instead.
    - escalate (notify + wait) becomes a future design choice conditional on a notification delivery path landing (Phase R).
  neutral:
    - expires_at is captured onto ApprovalRequest and is part of the wire envelope (FR-P0-01).
---


# ApprovalRequest expiry defaults to deny; no agent-side TTL extension

## Context

{% context %}
Phase 0 requires expiry semantics on ApprovalRequest. Three axes are open: default decision on expiry (deny / hold / escalate), whether the driver may extend TTL mid-flight, and where the policy value comes from. NFR-02 fixes the safety orientation (deny for destructive kinds unless the driver explicitly opted in), but the exact source and extension rule need to be pinned.
{% /context %}

## Decision

{% decision %}
Default decision on expiry is deny for destructive command/file-change approval kinds; a driver may declare per-session auto-approve-for-session at ApprovalRequest creation time and that value is captured onto ApprovalRequest.default_decision at creation (immutable_value semantics). The captured value is used at expiry — the driver's live policy at expiry time is NOT re-read, closing the TOCTOU window. Agent-side TTL extension is not supported in Phase 0: expires_at is set at creation and cannot be mutated. hold-until-answered and escalate are rejected for Phase 0 (blocking vs delivery-path-dependent respectively).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Safety-by-default preserved (NFR-02); TOCTOU-free by construction.
- Expiry reducer is a total function of (state, tick), matching the FC/IS invariant.
- Extension policy is fixed: no agent-side surprise; operators always see a bounded pending state.
{% /consequence %}

{% consequence kind="negative" %}
- A legitimate long-running approval cannot ask for more time; the driver must re-emit a fresh approval request instead.
- escalate (notify + wait) becomes a future design choice conditional on a notification delivery path landing (Phase R).
{% /consequence %}

{% consequence kind="neutral" %}
- expires_at is captured onto ApprovalRequest and is part of the wire envelope (FR-P0-01).
{% /consequence %}

## Alternatives

- **hold indefinitely on expiry** — Blocks the driver and creates unbounded state.
- **escalate (notify + wait)** — Depends on a notification delivery path not in Phase 0/1 scope.
- **Re-read driver policy at expiry time** — TOCTOU window (issue-d2-expiry-default-source-underspecified); a mid-flight accept flip would silently upgrade a deny.
- **Allow agent-side TTL extension** — Reintroduces unbounded pending state and complicates single-writer invariants.

## Related

- decision inputs: `decision-input-approval-expiry-default-policy`, `decision-input-approval-expiry-default`
- requirements: `FR-P0-06`, `NFR-02`
- contracts: `contract-approval-question-lifecycle`, `contract-approval-question-envelope`
