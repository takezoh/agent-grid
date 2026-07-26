---
id: task-20260726-vibe-island-verification-matrix
kind: task
title: UAC-テスト対応表と verification member の整備
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- docs/changes/change-20260725-vibe-island-human-input-contract/verification.md
pr: null
tags:
- vibe-island
- human-input
- verification
owners: []
relations:
- {type: partOf, target: change-20260725-vibe-island-human-input-contract}
source_paths:
- docs/changes/change-20260725-vibe-island-human-input-contract/verification.md
pinned_context:
- adr-20260725-vibe-island-asis-contract-pin
- adr-20260725-vibe-island-verification-tiers
- docs/changes/change-20260725-vibe-island-human-input-contract/design-plan.json
- docs/changes/change-20260725-vibe-island-human-input-contract/ux.md
- clients/windows-shell/docs/e2e.md
done_criteria:
- docs/changes/change-20260725-vibe-island-human-input-contract/verification.md の対応表
  (UAC / Tier / test 参照 / 状態)
max_diff_loc: 300
change: change-20260725-vibe-island-human-input-contract
summary: ux.md の 14 UAC を Tier / テスト参照へ 1 対 1 で対応付け、未カバー行 (T3-manual 含む) を明示して verification
  member を materialize する
updated: '2026-07-26'
---

# UAC-テスト対応表と verification member の整備

## 責務

ux.md の 14 UAC を Tier / テスト参照へ 1 対 1 で対応付け、未カバー行 (T3-manual 含む) を明示して verification member を materialize する。

## 詳細手順

1. `ux.md` の 14 UAC を列挙し、`design-plan.json` の `acceptance[]` と突き合わせて取りこぼしが無いことを確認する。
2. 既存テスト資産 (`AgentGrid.Shell.Core.Tests` / `AgentGrid.Shell.Platform.Tests` / `src` の go test / `AgentGrid.Shell.WinUI.UiTests`) を実体として照合し、対応表のテスト参照が**実在するテストクラス名**を指していることを確認する。実在しない参照は修正する。
3. 各行の Tier (T0 / T1 / T3 / T3-manual) と状態 (pinned / 補強 / T3-manual) を確定する。
4. 自動化しない行 (T3-manual) には理由を明記する。偽の緑を作る fake unit test を書かない (F2/no-fake-green)。
5. `docs lint` が zero error であることを確認する。

## 前提

- Tier 戦略は `adr-20260725-vibe-island-verification-tiers` が所有する (新ハーネスを作らない)。
- verification id ↔ requirement ↔ witness の正本は `design-plan.json` の `implementation_contracts[].verification / witnesses`。

## スコープ外

- テストコードの追加・変更 (他 unit の責務)。
- production code の変更。

## 受け入れ条件

- 全 14 UAC 行に Tier と test 参照または T3-manual の明示理由がある。
- docs lint が zero error。

## 参照ツール

既存テスト一覧 (Core.Tests / Platform.Tests / src go tests / WinUI.UiTests) と `clients/windows-shell/docs/e2e.md` を参照する。

## Decision closure

文書整備であり、記法の選択が契約の観測・owner・failure semantics・検証結果を変えることはない。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}
