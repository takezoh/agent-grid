---
id: task-20260705-recorded-fake-fixtures
kind: task
title: '録音駆動 fake: T3 transcript 録音 → preset 照合'
status: todo
created: '2026-07-05'
priority: low
effort: large
files_touched:
- src/platform/e2etest/record.go
- src/platform/lib/claude/fakeclaude/testdata/recordings/
- src/platform/agent/fakecodex/testdata/recordings/
- src/client/driver/replay_test.go
pr: null
tags:
- testing
- fake
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependsOn, target: task-20260705-fakecodex-settings-updated}
source_paths:
- src/platform/e2etest/e2etest.go
- src/platform/lib/claude/fakeclaude/lines.go
- src/platform/agent/fakecodex/presets.go
summary: e2e 実行時に real CLI のイベント列を testdata へ録音し、fake preset との照合と T0 golden replay
  に使う record/replay 基盤
---

# 録音駆動 fake: T3 transcript 録音 → preset 照合

## 責務

fake 忠実性の保証を「人がテストを書く」から「記録が契約になる」へ移行する (spec FR-011)。T3 の実行を
合否判定だけでなく fixture 採取に使う。

## 詳細手順

1. `platform/e2etest` に録音ヘルパを追加する: e2e 実行中の real CLI イベント列 (claude stream-json 行 /
   codex app-server notification) を `-record` flag 時に `testdata/recordings/*.jsonl` へ書き出す。
   秘匿情報 (path / token / timestamp) は録音時に正規化する。
2. **preset 照合 (T3)**: 録音と fake preset (`fakeclaude/lines.go` のビルダー出力、`fakecodex/presets.go`
   の emit 列) を、イベント種・field 集合・順序の水準で照合するテストを追加する。値の完全一致は要求しない
   (session id 等は毎回変わる) — 形の一致を契約とする。
3. **T0 golden replay**: 録音を `Driver.Step` に replay し、View の系列を golden 比較するテストを
   driver 側に追加する (更新は `-update` flag)。
4. 録音の更新手順 (real CLI バージョンアップ時) を fakeclaude / fakecodex の component doc に追記する。

## 前提

- task-20260705-fakecodex-settings-updated (preset の照合対象が揃っていること)

## スコープ外

- behavioral eval (タスク完遂の成果判定) — 録音はその将来基盤になるが本 task では扱わない
- 録音の CI 自動更新 (手動 opt-in 運用から始める)

## 受け入れ条件

- `-record` 付き e2e 実行で recordings が再現的に生成される (2 回実行で正規化後 diff ゼロ)
- preset 照合テストと golden replay が green
- `go vet -tags e2e` / `make lint` green
