---
id: adr-20260728-ephemeral-github-clone-authority
kind: adr
title: Ephemeral GitHub clone authority
status: accepted
created: '2026-07-28'
decision_makers:
- pending user consultation
consulted:
- design integration critique
informed:
- server API and platform security implementers
tags:
- github
- security
- credentials
- git
owners: []
relations:
- {type: originatedFrom, target: change-20260728-github-repository-sessions}
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: references, target: adr-20260727-single-user-server-auth}
- {type: references, target: adr-20260727-server-argv-materialization}
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- services/server/api/
- services/platform/lib/gitclone/
- services/platform/procgroup/
summary: Bound remote clone authority to authenticated one-use leases and a replay-resistant
  askpass broker pending consultation.
consequences:
  positive:
  - Private repositories can be cloned without durable server credential ownership.
  negative:
  - No-auth servers cannot create repository sessions and the broker needs strict
    process/peer lifecycle handling.
  neutral:
  - Existing project-session creation remains independent.
confirmation: Lease and askpass contract tests cover auth.mode=none, TTL, capacity,
  revoke, replay, peer/nonce validation, and sentinel absence.
updated: '2026-07-28'
---

## Status

Proposed. This decision is intentionally pending user consultation and does not authorize implementation until accepted or rejected.

## Context

A selected remote server needs private-repository authority without acquiring durable account ownership or exposing a token to renderer/process surfaces.

## Decision

Pending consultation, authenticated exact-version HTTPS or loopback admission creates a 60-second repository-bound one-use memory lease; no-auth servers reject it. A peer-credential and inherited-pipe-nonce-bound askpass broker consumes authority once and exposes only typed sanitized results.

## Consequences

{% consequence kind="positive" %}
Private repositories can be cloned without durable server credential ownership.
{% /consequence %}

{% consequence kind="negative" %}
No-auth servers cannot create repository sessions and the broker needs strict process/peer lifecycle handling.
{% /consequence %}

{% consequence kind="neutral" %}
Existing project-session creation remains independent.
{% /consequence %}

## Alternatives

- Token in URL, argv, environment, or git config: Leaks authority through durable or process-visible surfaces.
- Server-local gh authentication: Creates a second ambient credential authority.
- Client archive upload: Changes git semantics and adds a large transfer/provenance boundary.

## Confirmation

Lease and askpass contract tests cover auth.mode=none, TTL, capacity, revoke, replay, peer/nonce validation, and sentinel absence.
