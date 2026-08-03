---
id: task-20260803-reconnect-health-conformance
kind: task
title: Cross-boundary conformance and promotion
status: done
created: '2026-08-03'
priority: high
effort: medium
files_touched:
- clients/ui/e2e/**
- clients/ui/e2e/real-stack/**
- clients/ui/scripts/e2e-real-stack.sh
- services/server/api/**/*fleet*test.go
- services/server/api/**/*health*test.go
- services/internal/harnesspolicy/**
- services/internal/harnesspolicy/e2e_test.go
- services/internal/harnesspolicy/dependency_test.go
- test-harness/e2e-suites.json
- test-harness/dependencies.json
- Makefile
- docs/design/design-host.md
- docs/changes/change-20260727-resilient-server-reconnect-health/change.md
- docs/changes/change-20260727-resilient-server-reconnect-health/verification.md
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260727-resilient-server-reconnect-health}
- {type: dependsOn, target: task-20260803-active-recovery}
- {type: dependsOn, target: task-20260803-host-fleet-health}
source_paths: []
change: change-20260727-resilient-server-reconnect-health
summary: Close cross-host integration evidence, FakeVsReal conformance, full gates,
  and persistent design promotion.
updated: '2026-08-03'
---

<!-- 本文セクション構成はproducer方法論 (skills/_parts/decompose/knowledge/core.md) が規定する。
     dev-docs skill 自体はセクション構成を強制しない。 -->

## 責務

Unit 1〜4で閉じたowner-package testsを置き換えず、組み立てたWeb/Workspace/gateway境界を通るintegration evidenceと外部gateway fake tripleを完成させる。全機械gateを同一統合HEADで実行し、accepted ADRのchange promotionをpersistent client designへ反映する。

## 詳細手順

- browser fake-backend smokeとreal-stackで、hello-only recovery、exact exhaustion、no-session Retry、independent fleet degradation、Connections diagnosisを観測する。
- modified gateway seamにdeterministic fake、invariant-naming contract test、`FakeVsRealFleetHealth*` opt-in backstopを揃える。`test-harness/dependencies.json`へtripleを、`test-harness/e2e-suites.json`へ`services/server/api` packageを登録し、`services/internal/harnesspolicy/dependency_test.go`と`e2e_test.go`でregistry整合を固定する。`make test-e2e` → `scripts/run-go-e2e.sh` → registry package selection → `FakeVsRealFleetHealth*`の到達を確認し、real binary未設定以外のskipを許可しない。
- Workspace host wiring、Web host route、gateway scenarioのcompatibilityを確認し、secret sentinelをassembled wire/DOM/logでも検証する。
- repository AGENTS.mdのaffected package test、typecheck、lint、Go test、browser gatesを同じHEADで実行し、未実行T3/manual evidenceをgreenに数えない。
- change promotionの3項目を`design-client`のinvariant/responsibility/failure-responsibilityへupsertし、verification evidenceとdocs relation/lintを閉じる。

## 前提

`task-20260803-active-recovery`と`task-20260803-host-fleet-health`がunit-local implementation/testをgreenにしている。accepted ADRのdecisionは再審議せず、promotion manifestを機械的に反映する。

## スコープ外

Unit 1〜4の欠落機能やunit-local testsの後付け、assertion弱化、skip追加、Windows Shell tray health、runtime config reloadは扱わない。欠落はowning taskへ差し戻す。

## 受け入れ条件

- fake browserとreal-stackの双方でUAC-001〜UAC-014のcross-boundary代表traceが通り、active/fleet軸と両host diagnosisの組立結果を識別できる。
- gateway external dependencyについてfake、named invariant contract、`FakeVsRealFleetHealth*`が同一scripted evidenceを比較する。dependency/e2e-suite両registryとowning registry testsがgreenで、`make test-e2e`が`services/server/api`の登録packageを選択して`FakeVsRealFleetHealth*`へ到達し、authority未設定以外でskipしない。
- verification.md記載のtargeted commandsとAGENTS.mdのaffected full gateが同一HEADでgreenになり、未実行・expected skipを明示して成功扱いしない。
- `design-client`への3 promotion item、accepted ADR relation、change verification evidenceが反映され、docs lintがzero error/warningになる。
- integration-only remediationがunit-local contractを再定義せず、test削除・skip追加・assertion弱化を含まない。


{% transition from="todo" to="in_progress" date="2026-08-03" %}
plan-delivery implementation and owning gates accepted
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-03" %}
implementation integrated; affected mechanical and conformance gates verified
{% /transition %}
