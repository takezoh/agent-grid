---
id: adr-20260724-approval-question-state-domain-in-host-state
kind: adr
title: ApprovalRequest/QuestionRequest state domain lives under host/state (no new host/approval package)
status: accepted
created: 2026-07-24
summary: ApprovalRequest/QuestionRequest state domain lives under host/state (no new host/approval package)
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
    - Reuses the existing FC/IS invariants (single-writer loop, no time.Now inside Reduce, atomic State snapshot) without a new abstraction layer.
    - Reducer file naming (reduce_approval.go, reduce_question.go) matches the exempted dispatch-table pattern per ARCHITECTURE.md.
    - New Cmd/DEv cases in reduce_event.go stay adjacent to related dispatch.
  negative:
    - 'host/state/ grows by two new reduce_*.go files and a small number of new types, increasing the surface a first-time reader must scan.'
    - Any future promotion of approval/question to a cross-daemon capability would need to move types out, an additive refactor deferred to Phase R.
  neutral:
    - SubsystemApproval remains the driver-facing payload; ApprovalRequest is the state-facing durable representation.
    - The rename from SubsystemApprovalResolved (event) to a resolve action is not part of this ADR.
---


# ApprovalRequest/QuestionRequest state domain lives under host/state (no new host/approval package)

## Context

{% context %}
Phase 0 introduces a durable ApprovalRequest/QuestionRequest domain that must be reachable from the existing Reduce dispatch and Session.Driver/frame state. Placing the types under host/state/ reuses the FC/IS core, the single-writer event loop, the atomic.Pointer[State] snapshot discipline, and the reduce_*.go dispatch-table exemption from the function-length limit.
{% /context %}

## Decision

{% decision %}
The new types (ApprovalRequest, QuestionRequest, PendingApprovals, PendingQuestions) live under src/host/state/, alongside the existing driver_iface.go SubsystemApproval type. Reducer additions land as reduce_approval.go / reduce_question.go following the existing reduce_*.go convention. No new host/approval/ or host/question/ package is created.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Reuses the existing FC/IS invariants (single-writer loop, no time.Now inside Reduce, atomic State snapshot) without a new abstraction layer.
- Reducer file naming (reduce_approval.go, reduce_question.go) matches the exempted dispatch-table pattern per ARCHITECTURE.md.
- New Cmd/DEv cases in reduce_event.go stay adjacent to related dispatch.
{% /consequence %}

{% consequence kind="negative" %}
- host/state/ grows by two new reduce_*.go files and a small number of new types, increasing the surface a first-time reader must scan.
- Any future promotion of approval/question to a cross-daemon capability would need to move types out, an additive refactor deferred to Phase R.
{% /consequence %}

{% consequence kind="neutral" %}
- SubsystemApproval remains the driver-facing payload; ApprovalRequest is the state-facing durable representation.
- The rename from SubsystemApprovalResolved (event) to a resolve action is not part of this ADR.
{% /consequence %}

## Alternatives

- **New src/host/approval/ package containing types + reducer** — Duplicates the reduce_*.go convention host/state already uses; forces a cross-package boundary for what is purely state.
- **Keep synchronous auto-accept in host/runtime/subsystem/stream/event.go and expose an approval object only over the wire** — Violates plan gap #1; state is where reconnect and single-writer invariants live.

## Related

- decision inputs: `decision-input-approval-question-domain-placement`
- requirements: `FR-P0-01`, `FR-P0-02`
- contracts: `contract-approval-question-lifecycle`, `contract-approval-resolution-single-writer`
