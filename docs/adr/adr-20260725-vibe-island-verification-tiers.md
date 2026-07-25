---
id: adr-20260725-vibe-island-verification-tiers
kind: adr
title: human-input 契約の検証を T0/T1/T3 の既存 Tier 資産へ帰着させる
status: accepted
created: '2026-07-25'
summary: 新ハーネスは作らず、witness を T0 純粋層 (Core/Platform Tests) / T1 Go (reducer + gateway
  scenario) / T3 FlaUI UIA e2e へ帰着させる。UIA でしか判別できない観測のみ T3、CI 不安定な実挙動は T3-manual を明示する
decision_makers:
- agent-grid-maintainers
tags:
- native-clients
- windows-shell
- testing
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260725-vibe-island-human-input-contract}
source_paths: []
consequences:
  positive:
  - 検証コストが既存のテスト実行 (go test / dotnet test / e2e.sh) に閉じ、CI 構成の変更が不要
  negative:
  - T3 は常時実行されないため、UIA でしか観測できない退行 (要素残存・フォーカス) の検出が遅延し得る
  neutral:
  - T3-manual と明示した行 (UAC-011 の実フォーカス返却、UAC-031 の実機トースト操作) は verification member 上で機械カバレッジ外であることが可視化され続ける
confirmation: verification.md の UAC×Tier 対応表で全 14 行に Tier とテスト参照 (または T3-manual 理由)
  があること。偽の緑となる mock 化 unit test を T3-manual 行に追加しないこと
updated: '2026-07-25'
---

# human-input 契約の検証を T0/T1/T3 の既存 Tier 資産へ帰着させる

## Context

witness の検証先として、(a) T0 純粋層テスト (WsFrameParser / SupervisionReducer / PanelPresentation / IslandStateMachine / Toast / Engage)、(b) T1 Go テスト (state reducer + gateway scenario)、(c) T3 FlaUI/UIA3 e2e (AutomationId 契約、run-dev ハーネス) が既に存在する。新しい UI オートメーションハーネスを作るかどうかの判断が必要。

## Decision

新ハーネスは作らず、witness を既存 3 層へ帰着させる。UI ツリーの要素非存在 (FR-007) と実フォーカス観察 (NFR-002) だけは UIA でしか判別できないため T3 に置く。IME を含む実フォーカス返却 (UAC-011) と実機トースト操作 (UAC-031) は CI 安定性の観点から T3-manual を許容し、verification member に手動確認として明示する (mock で通る偽の緑 unit test は書かない)。

## Consequences

frontmatter `consequences` (positive / negative / neutral) が SoT。

## Alternatives

- **WinAppDriver / 専用 UI オートメーションハーネスの新設** — 却下: 既存 FlaUI 規約 (docs/note/note-20260724-windows-shell-ui-e2e-authoring.md) と重複し保守面が増える。判別力の増分は AutomationId 契約の範囲で FlaUI と同等。
- **全 witness を T3 e2e で検証する** — 却下: 純粋層で判別できる契約を e2e に持ち上げると実行時間と flake が増え、T0 の高速フィードバックを失う。
