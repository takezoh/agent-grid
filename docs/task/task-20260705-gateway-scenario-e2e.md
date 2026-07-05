---
id: task-20260705-gateway-scenario-e2e
kind: task
title: 'gateway scenario e2e: fake CLI で server→view 貫通'
status: done
created: '2026-07-05'
priority: normal
effort: medium
files_touched:
- src/server/web/mux_scenario_test.go
- src/server/web/mux_e2e_test.go
pr: null
tags:
- testing
- web
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependsOn, target: task-20260705-wire-fixtures-pipeline}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/server/web/mux_e2e_test.go
- src/platform/lib/claude/fakeclaude/
- src/server/web/gateway.go
summary: real server binary + fakeclaude/fakecodex を agent CLI として session create
  → WS subscriber の viewUpdate frame 列を assert する常時実行テスト
updated: '2026-07-05'
---

# gateway scenario e2e: fake CLI で server→view 貫通

## 責務

platform (server) => view (client) の伝搬を wire レベルで貫く常時テストを新設する (spec FR-007 /
AC-003)。real CLI を使わない (fake CLI) ため PR CI で走る — tier としては T1.5 (real binary は自前の
server のみ)。

## 詳細手順

1. 既存 `mux_e2e_test.go` (real server binary を `go build` で spawn、`-short` skip) の方式を拡張し、
   scenario test を新設する:
   - fakeclaude (pty-attached fake CLI) を agent command とした session を REST で create
   - WS で surface / view を subscribe
   - fakeclaude の stream 進行に伴う `viewUpdate` frame 列を受信し、title / status / model / effort が
     driver View と一致することを assert
   - session stop → viewUpdate の消滅反映まで確認
2. codex 側 (stream subsystem 経由) の同型シナリオを 1 本追加する (`subsystem/stream/fake` の fake CLI
   `codex --remote` 互換を利用)。
3. wire assert には task-20260705-wire-fixtures-pipeline の testdata fixture を可能な範囲で共用する
   (フレーム形状の assert を fixture と二重定義しない)。
4. binary build (~5s) を考慮し `-short` skip を維持、CI では標準 test ジョブで実行されることを確認する。

## 前提

- task-20260705-wire-fixtures-pipeline (assert 素材の共用。fixture 無しでも着手自体は可能だが、二重定義を
  避けるため後行を推奨)

## スコープ外

- real claude / codex を使う fidelity 検証 (既存 T3 スイートが担う)
- ブラウザ (xterm.js / React) 層の e2e

## 受け入れ条件

- fakeclaude / fake codex 両シナリオが CI 標準ジョブで green (API key 不要)
- flaky でない (`-count=5` で安定)
- `make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Reconciling implemented gateway scenario suite with docs lifecycle.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented and verified gateway scenario suite; fake agent build failures now fail tests.
{% /transition %}
