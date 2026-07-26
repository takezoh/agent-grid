---
id: task-20260726-vibe-island-core-contract-tests
kind: task
title: 島側契約テストの整合確認と補強
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- clients/windows-shell/AgentGrid.Shell.Core.Tests
- clients/windows-shell/AgentGrid.Shell.Platform.Tests
pr: null
tags:
- vibe-island
- human-input
- windows-shell
owners: []
relations:
- {type: partOf, target: change-20260725-vibe-island-human-input-contract}
source_paths:
- clients/windows-shell/AgentGrid.Shell.Core.Tests
- clients/windows-shell/AgentGrid.Shell.Platform.Tests
pinned_context:
- adr-20260725-vibe-island-asis-contract-pin
- adr-20260725-vibe-island-verification-tiers
- docs/changes/change-20260725-vibe-island-human-input-contract/design-plan.json
- clients/windows-shell/AgentGrid.Shell.Core/SupervisionState
- clients/windows-shell/AgentGrid.Shell.Platform/Toast
- clients/windows-shell/AgentGrid.Shell.Platform/Engage
done_criteria:
- 追加/更新された xUnit テスト (Core.Tests / Platform.Tests)
max_diff_loc: 300
change: change-20260725-vibe-island-human-input-contract
summary: contract-island-attention-observability と contract-pending-card-lifecycle
  の witness を既存 T0 テストへ帰着させ、対応する assert が無い witness にはテストを追加する
updated: '2026-07-26'
---

# 島側契約テストの整合確認と補強

## 責務

`contract-island-attention-observability` と `contract-pending-card-lifecycle` の witness を既存 T0 テストへ帰着させ、対応する assert が無い witness にはテストを追加する。

## 対象 witness

| witness | 期待観察 | verification |
|---|---|---|
| witness-arrival-normal | 自動展開とカード・バッジ表示 / フォーカス非移動 | verify-island-core-t0, verify-uia-e2e |
| witness-unwatched-toast-normal | 非注視時にトーストがちょうど 1 回 | verify-toast-router-t0 |
| witness-quiet-normal | 自動収納 / 承認・質問要素の非生成 | verify-island-core-t0, verify-uia-e2e |
| witness-frame-flood-adversarial | 未知 k フレーム無視・重複 ar の単一カード・連続 v での非重複トースト | verify-island-core-t0, verify-toast-router-t0, verify-uia-e2e |
| witness-submit-normal | カード即時除去 / 返信欄クリアとフォーカス返却 | verify-lifecycle-core-t0, verify-engage-focus-t0 |
| witness-offline-rollback-normal | 除去→復元 + オフラインバッジ | verify-lifecycle-core-t0 |
| witness-race-loss-normal | カード除去 + Handled elsewhere 通知 | verify-lifecycle-core-t0 |
| witness-lifecycle-adversarial | 復元→再送→Handled elsewhere の正規化 / 破棄済み HWND へ返却しない / カードは高々 1 枚 | verify-lifecycle-core-t0, verify-engage-focus-t0 |

## 詳細手順

1. 各 witness の `expected_observables` / `forbidden_outcomes` を既存テストの assert へ 1 対 1 で帰着させる。
2. 帰着先が無い組み合わせに xUnit テストを追加する。既存パターン (fake 注入・純関数 fixture) に倣う。
3. `dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests` と `dotnet test clients/windows-shell/AgentGrid.Shell.Platform.Tests` が green であることを確認する。

## 前提

- 既存 seam (WsFrameParser 純関数 / fake gateway 注入 / IToastNotifier fake / IslandStateMachine 純関数) で全 witness が観測できる。

## スコープ外

- production code は変更しない。seam 追加が必要と判明したら unit を止めて設計へ差し戻す (`adr-20260725-vibe-island-asis-contract-pin`)。
- gateway 側テストは他 unit の責務。

## 受け入れ条件

- dotnet test が両プロジェクトで green。
- witness (arrival / toast-once / quiet / rollback / race-loss / focus-return / flood) それぞれに対応するテストが存在する。

## 参照ツール

SupervisionState / Toast / Engage の既存テストパターン (fake 注入・純関数 fixture) に倣う。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}
