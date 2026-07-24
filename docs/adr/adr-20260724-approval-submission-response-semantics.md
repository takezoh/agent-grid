---
id: adr-20260724-approval-submission-response-semantics
kind: adr
title: SupervisionState uses optimistic removal with rollback-on-error and trusts
  server's authoritative resolved-by-other response
status: accepted
created: '2026-07-24'
summary: SupervisionState uses optimistic removal with rollback-on-error and trusts
  server's authoritative resolved-by-other response
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - Near-instant perceived latency on the common path.
  - UAC-006 and UAC-006r counterexamples now fail explicit FRs/contract tests instead
    of being permitted by prose.
  negative:
  - Requires the rollback outcome partition to be correct; verified by xUnit table
    tests.
  neutral:
  - adr-20260724-approval-single-writer-first-commit's authoritative outcome semantics
    are consumed unchanged.
---
# SupervisionState uses optimistic removal with rollback-on-error and trusts server's authoritative resolved-by-other response

## Context

{% context %}
F-003 needs both a near-instant approve/deny UI and correct behavior on network/API failure and on losing a two-client race (adr-20260724-approval-single-writer-first-commit). Critique blocker + major: neither observable was in EARS/contract layer.
{% /context %}

## Decision

{% decision %}
SupervisionState optimistically removes the queue item on submission and rolls back on network/server error (contract-approve-submission-rollback). On resolved-by-other, SupervisionState renders an explicit already-handled state and does not submit a duplicate decision (contract-resolved-by-other-display). No client-side dedupe flag is added; cc-duplicate-response-contract-layer assigns duplicate defense to the contract layer.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Near-instant perceived latency on the common path.
- UAC-006 and UAC-006r counterexamples now fail explicit FRs/contract tests instead of being permitted by prose.
{% /consequence %}

{% consequence kind="negative" %}
- Requires the rollback outcome partition to be correct; verified by xUnit table tests.
{% /consequence %}

{% consequence kind="neutral" %}
- adr-20260724-approval-single-writer-first-commit's authoritative outcome semantics are consumed unchanged.
{% /consequence %}

## Alternatives

- **Pessimistic wait-for-ack (no optimistic removal)** — Adds visible latency to every successful decision; reads as sluggish and UAC-005's Then does not require it.
- **Client-side pessimistic lock as primary defense** — Duplicates the server's single-writer invariant and races across two independent clients.

## Related

- decision inputs: `decision-input-answerer-identity`
- requirements: `FR-APPROVE-ROLLBACK`, `FR-APPROVE-RESOLVED-BY-OTHER`
- contracts: `contract-approve-submission-rollback`, `contract-resolved-by-other-display`
- change: `change-20260723-windows-shell-phase2`
