---
id: adr-20260724-approval-lifecycle-teardown-cancel
kind: adr
title: Frame/session teardown transitions pending ApprovalRequest/QuestionRequest to cancelled and drains held JSON-RPC requests
status: accepted
created: 2026-07-24
summary: Frame/session teardown transitions pending ApprovalRequest/QuestionRequest to cancelled and drains held JSON-RPC requests
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
    - Lifecycle state machine is closed under all real-world termination paths.
    - Reuses the sandbox-release regression pattern (existing test seam).
    - Client-initiated cancel is a natural extension of the same terminal transition, so its wire shape is fixed alongside expiry.
  negative:
    - Every new reduce case for approval/question must also handle a teardown-in-flight branch; test coverage matrix grows.
    - 'Cancel by a non-owning client is deliberately unsupported in Phase 0 (only the requesting client''s connection can cancel), postponed with the client-instance-id design.'
  neutral:
    - Held JSON-RPC id draining uses the existing conn.Reply/ReplyError seam in stream.Backend; no new IPC primitive.
---


# Frame/session teardown transitions pending ApprovalRequest/QuestionRequest to cancelled and drains held JSON-RPC requests

## Context

{% context %}
The current handleRequest path in host/runtime/subsystem/stream/event.go always replies to the driver's JSON-RPC id synchronously, so there is no pending state to leak. Moving to hold-open makes teardown a first-class terminal transition (issue-d2-lifecycle-cancel-teardown-missing). The sandbox release path (d1e3a8c4) is the closest existing precedent for a session-teardown reap.
{% /context %}

## Decision

{% decision %}
A frame/session teardown DEv (including sandbox release, session eviction, and daemon shutdown) triggers a Reduce cycle that: (a) transitions every pending ApprovalRequest/QuestionRequest owned by the teardown scope to cancelled, (b) drains each held driver JSON-RPC id with a connection-lost error reply, (c) removes the entries from state.PendingApprovals/PendingQuestions, and (d) broadcasts EvtApprovalResolved/EvtQuestionResolved with resolution_reason=cancelled. Client-initiated CmdApprovalCancel/CmdQuestionCancel follow the same terminal transition for a request the caller owns. No goroutine or map entry may outlive the reap.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Lifecycle state machine is closed under all real-world termination paths.
- Reuses the sandbox-release regression pattern (existing test seam).
- Client-initiated cancel is a natural extension of the same terminal transition, so its wire shape is fixed alongside expiry.
{% /consequence %}

{% consequence kind="negative" %}
- Every new reduce case for approval/question must also handle a teardown-in-flight branch; test coverage matrix grows.
- Cancel by a non-owning client is deliberately unsupported in Phase 0 (only the requesting client's connection can cancel), postponed with the client-instance-id design.
{% /consequence %}

{% consequence kind="neutral" %}
- Held JSON-RPC id draining uses the existing conn.Reply/ReplyError seam in stream.Backend; no new IPC primitive.
{% /consequence %}

## Alternatives

- **Leave pending state on teardown; rely on the driver's own cleanup** — Leaks goroutines and map entries; observable regression from the current synchronous behavior.
- **Send a `pending` frame indefinitely and hope the driver reconnects** — Contradicts the FC/IS invariant that terminal transitions are Reduce-owned.

## Related

- decision inputs: `decision-input-approval-lifecycle-teardown`
- requirements: `FR-P0-11`, `NFR-06`
- contracts: `contract-approval-question-lifecycle`
