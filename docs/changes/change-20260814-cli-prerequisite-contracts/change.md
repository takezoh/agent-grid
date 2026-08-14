---
id: change-20260814-cli-prerequisite-contracts
kind: change
title: Execution plane and client config prerequisites for the terminal CLI
status: active
created: '2026-08-14'
profile: sdd@1
intent: <変更理由と期待結果>
outcomes: []
scope: []
non_goals: []
change_classes: []
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260814-cli-prerequisite-contracts/requirements.md
  required: true
- role: implementation
  path: changes/change-20260814-cli-prerequisite-contracts/implementation.md
  required: true
- role: verification
  path: changes/change-20260814-cli-prerequisite-contracts/verification.md
  required: true
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations: []
source_paths: []
summary: 終了 session の terminal state 保持、transport close の型付き reason、client config
  契約の全 client 共通化と server display name の per-entry admission — terminal CLI が依存する
  3 つの前提 contract change
updated: '2026-08-14'
---

## Summary

## Closure Notes


{% transition from="draft" to="ready" date="2026-08-14" %}
members derived from the shared accepted design plan (change-20260812-cli-terminal-client/design-plan); 4 ADR accepted
{% /transition %}


{% transition from="ready" to="active" date="2026-08-14" %}
main session implementation
{% /transition %}
