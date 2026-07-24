---
id: adr-20260724-approval-answerer-identity-per-ws-instance
kind: adr
title: Answerer identity is a per-WS-connection ephemeral client-instance-id minted by an extended ticketStore
status: accepted
created: 2026-07-24
summary: Answerer identity is a per-WS-connection ephemeral client-instance-id minted by an extended ticketStore
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
    - '`decided_by` on EvtApprovalResolved has a named producer without introducing a durable identity model.'
    - 'Two clients sharing the local bearer token are still distinguishable at the approval domain by their ci-*.'
    - ticketStore extension is additive; TokenAuth is unchanged; no auth.go modification is required.
  negative:
    - A client that reconnects sees a fresh ci; audit correlation across sessions requires stitching outside this design (deferred to Phase R).
    - REST callers must present the id explicitly via a header if their command originates outside the WS handler.
  neutral:
    - The id is deliberately opaque and never surfaces in UI or logs beyond audit.
    - Cross-host answering continues to be gated by multi-host-gateway.md §6.2 with no dependence on this local id.
---


# Answerer identity is a per-WS-connection ephemeral client-instance-id minted by an extended ticketStore

## Context

{% context %}
Phase 0's approval domain must record 'who resolved this' (FR-P0-03) so the loser of a two-client race can be told the authoritative winner (FR-P0-04) and audit trails have a producer for `decided_by`. The current auth surface (src/server/api/auth.go) compares a single shared bearer token: any two local clients presenting the same token are indistinguishable at the HTTP boundary. WS tickets (src/server/api/ticket.go) are single-use for the /ws handshake, so they cannot label successive REST CmdApprovalRespond calls. multi-host-gateway.md §6.2's user-signed-op chain is the authoritative surface for cross-host identity, but it is Phase R work.
{% /context %}

## Decision

{% decision %}
Extend ticketStore.mint to also mint a per-WS-connection ephemeral client-instance-id (24-byte crypto/rand opaque token), bound to the ticket for the WS's lifetime and cleared on WS close. The WS handler threads the id onto every CmdApprovalRespond/CmdQuestionRespond originating from that connection; REST callers may present the same id via a session header when their WS is active. This does NOT modify the bearer scheme, does NOT persist across connections, and does NOT identify a human. It is a scope-local, connection-lived producer for `decided_by`; the passkey / scoped-assertion chain remains multi-host-gateway.md's Phase R work.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- `decided_by` on EvtApprovalResolved has a named producer without introducing a durable identity model.
- Two clients sharing the local bearer token are still distinguishable at the approval domain by their ci-*.
- ticketStore extension is additive; TokenAuth is unchanged; no auth.go modification is required.
{% /consequence %}

{% consequence kind="negative" %}
- A client that reconnects sees a fresh ci; audit correlation across sessions requires stitching outside this design (deferred to Phase R).
- REST callers must present the id explicitly via a header if their command originates outside the WS handler.
{% /consequence %}

{% consequence kind="neutral" %}
- The id is deliberately opaque and never surfaces in UI or logs beyond audit.
- Cross-host answering continues to be gated by multi-host-gateway.md §6.2 with no dependence on this local id.
{% /consequence %}

## Alternatives

- **Immediate per-user passkey / scoped-assertion identity** — Overlaps with multi-host-gateway.md §6.2's Phase R design; blocks Phase 0 exit for no in-scope benefit.
- **Do nothing; leave decided_by as an anonymous shared-bearer holder** — Fails FR-P0-03 (identity has no source) and blocks issue-d2-answerer-identity-gap resolution.
- **Reuse the WS ticket itself as the identifier** — Single-use consumption on /ws upgrade destroys the value before subsequent commands can reference it.

## Related

- decision inputs: `decision-input-approval-answerer-identity`, `decision-input-auth-trust-boundary-split`
- requirements: `FR-P0-03`, `FR-P0-10`, `FR-P0-12`
- contracts: `contract-auth-trust-boundary-approval-answering`
