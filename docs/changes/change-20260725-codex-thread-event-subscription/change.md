---
id: change-20260725-codex-thread-event-subscription
kind: change
title: Codex thread event subscription ownership
status: done
created: '2026-07-16'
profile: sdd@1
intent: Make Codex frame readiness a single lifecycle transaction that commits only
  after runtime activation, backend observer subscription, and canonical identity
  validation.
outcomes:
- SessionReady は bound frame ごとに runtime activation・observer subscription・canonical
  identity validation の 3 条件成立後に exactly once 発火する。
- fresh/recovery のどちらの Codex frame も backend ResumeThread 経由で observer subscription
  を持ち、thread/started は identity discovery だけに使われる。
- ReleaseFrame は tombstone-first で idempotent となり、typed thread/unsubscribe が routing
  stop より前に commit される。
scope:
- Required Subsystem ActivateFrame lifecycle contract and runtime spawn-complete call
  site
- Fresh identity-only discovery and fresh/recovery backend ResumeThread subscription
- Typed thread/unsubscribe and idempotent tombstone-first release
- Connection-scoped fake semantics and external dependency triple
- Gateway view-update and minimum Web status smoke coverage
non_goals:
- Targeted shim relay or full JSON-RPC broker
- Codex rollout file generation or mutation
- Driver reducer, Web status mapping, or shim notification transparency redesign
- Codex-unrelated lifecycle generalization beyond explicit non-stream no-op implementations
change_classes:
- behavior
- responsibility
- boundary
governance:
  gate: soft
  reasons:
  - legacy-import
evidence_refs:
- type: source
  ref: git:fb12e1d8 fix(codex) own thread status subscriptions in backend
- type: test
  ref: go:test ./platform/agent/codexclient/... ./platform/agent/codexschema/...
- type: test
  ref: go:test ./host/runtime/subsystem/stream/...
promotion:
- action: none
  reason: Legacy v1 package imported after the format v2 cutover. The durable decision
    is already recorded in adr-20260716-codex-observer-subscription-ready-ownership;
    folding an observer-subscription invariant into design-client is a separate design
    review and is deliberately not invented during migration.
members:
- role: requirements
  path: changes/change-20260725-codex-thread-event-subscription/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-codex-thread-event-subscription/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-codex-thread-event-subscription/verification.md
  required: true
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: references, target: adr-20260716-codex-observer-subscription-ready-ownership}
- {type: conformsTo, target: design-client}
source_paths: []
summary: Codex frame の Ready を runtime activation・backend observer subscription・canonical
  thread identity validation が全て commit された後にだけ発火する 1 つの lifecycle transaction にする。
updated: '2026-07-25'
closure:
  closed_at: '2026-07-25T03:35:02.942001+00:00'
  content_hash: sha256:8323b757a96831326d941ab23ce2621770be9dc5a2c6d67720d0843f7b05b52a
---

## Summary

format v2 cutover (`df834977`) より後に v1 形式で追加された `docs/specs/codex-thread-event-subscription/` (spec + plan) を v2 change package へ移行したもの。requirements / implementation member は当時の v1 artifact を verbatim で保持している。

実装は `fb12e1d8 fix(codex): own thread status subscriptions in backend` で main に着地済み。

## Closure Notes

移行時点で codex 関連の verification profile (`codexclient` protocol contract、stream subsystem と connection-scoped fake) を再実行して green を確認した。v1 plan が列挙していた repo 全体ゲートは、本 change と無関係な既存 failure (`TestRunAgentFramesMCP_RequiresToken` in `cmd/bridge`, `cmd/server`) を含むため evidence には採らない。


{% transition from="draft" to="ready" date="2026-07-25" %}
legacy v1 package imported; work already shipped in fb12e1d8
{% /transition %}


{% transition from="closing" to="active" date="2026-07-25" %}
record verification evidence and promotion decision before closure
{% /transition %}
