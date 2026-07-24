---
id: change-20260723-native-clients-phase01
kind: change
title: 'Native clients Phase 0/1: contract layer + approval/question domain + generated
  SDKs'
status: draft
created: '2026-07-23'
profile: sdd@1
intent: 'Give agent-grid one platform-independent, contract-first wire layer for sessions/approvals/questions/commands/events
  (Phase 1) grounded on a new server-side approval/question domain that actually holds
  a pending decision instead of auto-resolving it (Phase 0), so every future native
  client and the browser consume the same typed, generated, simulator-verifiable contract
  with no privileged back doors.'
outcomes:
- Phase 0 durable ApprovalRequest / QuestionRequest domain in host/state with expiry,
  cancel/teardown, and first-writer-wins two-client conflict resolution
- Phase 0 per-WS-connection ephemeral client-instance-id giving `decided_by` a named
  producer without preempting multi-host-gateway.md's Phase R identity chain
- Phase 1 protocol/ as normative wire SoT (OpenAPI 3.1 + JSON Schema 2020-12), C#/Swift/Kotlin/TS
  SDK generation via OpenAPI Generator, simulator, and fail-closed compatibility CI
  gate
scope:
- src/host/state (approval/question durable domain + reducers + reap)
- src/host/proto (Evt*/Cmd*/Resp* wire types)
- src/host/runtime/subsystem/stream (hold-open replacing synchronous auto-accept)
- src/server/api (REST + WS surface, ticketStore extension for client-instance-id)
- protocol/*.schema.json + openapi.yaml + protocol/simulator/
- contracts/{approval,question,reconnect,compatibility,handoff}-contract.md
- clients/sdk/{csharp,swift,kotlin,ts}
- clients/ui/src/wire adapter seam (incremental migration)
- test-harness/profiles.json + .github/workflows/ci.yml compatibility job
non_goals:
- Phase 2 desktop app vertical slice (windows-shell, workspace Electron)
- Phase R remote reachability, push delivery, WebRTC/Noise transport implementation
- Phase 3+ mobile clients
- Full runtime capability-negotiation behavior beyond the bundled/remote two-axis
  policy skeleton
- Distribution / signing / auto-update
change_classes:
- capability
- behavior
- boundary
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260723-native-clients-phase01/requirements.md
  required: true
- role: implementation
  path: changes/change-20260723-native-clients-phase01/implementation.md
  required: true
- role: verification
  path: changes/change-20260723-native-clients-phase01/verification.md
  required: true
promotion: []
unresolved_decisions:
- openapi-generator-template-set-choice
- sim-server-language-choice
tags:
- native-clients
- phase01
- protocol
- approval-question
owners:
- agent-grid-maintainers
relations: []
source_paths: []
summary: plan-20260723-native-clients.md の Phase 0 (approval/question サーバー側ドメイン + capability negotiation
  + auth 前提) と Phase 1 (typed schemas + deep links + 生成 SDK + simulator) の technical design;
  8 accepted + 1 proposed ADR, 11 implementation contracts, 12 chunks.
---

## Summary

Phase 0 replaces the synchronous auto-accept in `src/host/runtime/subsystem/stream/event.go handleRequest` with a durable `ApprovalRequest` / `QuestionRequest` state domain in `src/host/state/`. Two-client conflict resolves as first-committed-wins under the existing single-writer Reduce loop; expiry defaults to deny with the policy captured at creation (TOCTOU-free); frame/session teardown transitions pending state to cancelled and drains held driver JSON-RPC requests. A per-WS-connection ephemeral client-instance-id minted by an extended `ticketStore` gives `decided_by` a named producer without touching the bearer scheme, deferring cross-host identity to `multi-host-gateway.md` §6.2.

Phase 1 turns `protocol/*.schema.json + openapi.yaml` into the normative wire SoT, generates C# / Swift / Kotlin / TypeScript SDKs from it via a pinned OpenAPI Generator, stands up a three-part simulator (fixture + recorded stream + sim server) under `protocol/simulator/`, and wires a new `compatibility` `test-harness/profiles.json` group that fails closed on undeclared SDK surface, inconclusive scans, and new-SDK targets skipping the shared recorded-scenario suite. Deep-link URI shape adopts `plans/remote-control-mobile-session-deep-link.md` verbatim.

## Related documents

- Requirements: [`requirements.md`](./requirements.md)
- Implementation: [`implementation.md`](./implementation.md)
- Verification: [`verification.md`](./verification.md)
- Plan artifact (SoT for validate_plan.py): `/home/ubuntu/.dev-skills/design/native-clients-phase01/artifacts/plan.json`
- 11 originated ADRs — see `relations[]` above and `implementation.md` §Implementation contracts

## Closure Notes
