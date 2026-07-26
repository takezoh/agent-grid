---
id: change-20260727-agy-passive-status-observation
kind: change
title: agy passive status observation and reproducible driver characterization
status: done
created: '2026-07-27'
profile: sdd@1
intent: Observe agy lifecycle through passive official hooks without changing permission
  policy or inventing timeout transitions, and preserve reproducible evidence for
  future Driver characterization.
outcomes:
- agy is a dedicated Driver whose passive PreInvocation and fully-idle Stop events
  project Running and Waiting.
- Managed Antigravity plugins are installed on host and devcontainer paths without
  registering policy-changing PreToolUse.
- Exact Esc and native permission gaps remain explicit upstream limitations rather
  than elapsed-time inference.
- Reusable PTY capture, OSC/BEL analysis, and JSON hook recording scripts make future
  Driver research reproducible.
scope:
- src/host/driver agy reducer and shared status evidence integration
- src/host/lib/agenthook host and devcontainer managed Antigravity plugins
- scripts/research and structured investigation documentation
non_goals:
- Inferring Esc completion from keypresses, terminal text, silence, or time windows
- Registering PreToolUse and thereby changing the user's native permission policy
change_classes:
- behavior
- responsibility
- boundary
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: go:test ./host/... ./cmd/server/... ./cmd/bridge/...
- type: test
  ref: go:test -tags=e2e ./host/driver -run TestFakeVsRealAgyPassiveLifecycle
- type: test
  ref: web:unit+lint
members:
- role: requirements
  path: changes/change-20260727-agy-passive-status-observation/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-agy-passive-status-observation/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-agy-passive-status-observation/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: The accepted multi-source status ADR already governs the durable observation
    boundary; this change adds implementation and reproducible research evidence without
    a new durable design decision.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: conformsTo, target: adr-20260726-multi-source-driver-status-evidence}
source_paths: []
summary: Replace the generic agy adapter with managed passive hooks, deterministic
  status reduction, and reproducible Driver signal investigation procedures.
updated: '2026-07-27'
promotion_applied_at: '2026-07-26T15:45:58.880538+00:00'
closure:
  closed_at: '2026-07-26T15:46:03.845474+00:00'
  content_hash: sha256:14c7471bee1d643a3a9813ce5de9ca17a4792c4e9f51ffcd143e09391275181e
---

## Summary

agy を generic terminal Driver から専用 Driver へ移し、公式の
`PreInvocation` / `PostInvocation` / `PostToolUse` / `Stop` hook を
policy-neutral な観測入力として配線する。hook、process、terminal signal は
独立した evidence として保持し、経過時間による状態上書きは行わない。

## Closure Notes

Esc 中断時の Stop hook cancellation と、native permission prompt の受動的な
開始イベント欠落は upstream 制約として残る。Agent Grid はこれをキー入力、
画面本文、無出力時間から推測しない。


{% transition from="draft" to="ready" date="2026-07-27" %}
Requirements, observation boundaries, and mechanical verification are fixed.
{% /transition %}


{% transition from="ready" to="active" date="2026-07-27" %}
Dedicated Driver, managed plugins, tests, and research scripts implemented.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-27" %}
Focused suites, authenticated FakeVsReal, client tests, harness policy, and docs lint pass; unrelated repository-wide failures are recorded.
{% /transition %}
