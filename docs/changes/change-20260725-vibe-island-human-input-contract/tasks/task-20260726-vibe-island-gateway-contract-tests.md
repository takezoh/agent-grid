---
id: task-20260726-vibe-island-gateway-contract-tests
kind: task
title: gateway 往復契約テストの確認と補強
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/host/state
- src/server/api
pr: null
tags:
- vibe-island
- human-input
- gateway
owners: []
relations:
- {type: partOf, target: change-20260725-vibe-island-human-input-contract}
source_paths:
- src/host/state
- src/server/api
pinned_context:
- adr-20260725-vibe-island-asis-contract-pin
- adr-20260725-vibe-island-verification-tiers
- docs/changes/change-20260725-vibe-island-human-input-contract/design-plan.json
- src/host/state/reduce_approval.go
- src/host/runtime/subsystem/stream
done_criteria:
- 追加/更新された Go テスト (src/host/state / src/server/api)
max_diff_loc: 550
change: change-20260725-vibe-island-human-input-contract
summary: contract-human-input-roundtrip の witness (単一解決 / 期限切れ / teardown / 再接続非捏造)
  を Go テストへ帰着させ、欠落 assert を補強する
updated: '2026-07-26'
---

# gateway 往復契約テストの確認と補強

## 責務

`contract-human-input-roundtrip` の witness (単一解決 / 期限切れ / teardown / 再接続非捏造) を Go テストへ帰着させ、欠落 assert を補強する。

## 対象 witness

| witness | 期待観察 | verification |
|---|---|---|
| witness-approve-roundtrip-normal | 保留 JSON-RPC へ accept が 1 回 / ax 配信 | verify-gateway-scenario, verify-state-reducer |
| witness-other-client-normal | 先着のみ適用 / 後着は resolved_by_other / ax は両者へ | verify-gateway-scenario |
| witness-expiry-normal | default_decision で自動解決 / expired として broadcast | verify-state-reducer |
| witness-race-adversarial | 返信は 1 回のみ / 後着は resolved_by_other / 再接続後に既解決要求は配信されない / 不正値は非受理 | verify-gateway-scenario, verify-state-reducer |

## 詳細手順

1. `reduce_approval_test.go` / `reduce_question_test.go` と mux gateway scenario テストを読み、first-writer-wins / TTL default / teardown connection-lost / 409 conflict / hello 非 replay の assert 有無を棚卸しする。
2. 欠落 assert を追加する。特に UAC-043 の「hello フレームが pending を含まない」「既解決要求が再配信されない」を gateway scenario test へ明示 assert として追加する。
3. `cd src && go test ./host/state/... ./server/api/...` が green であることを確認する。

## 前提

- 単一 writer の Reduce ループが全コマンドを直列化し、解決済みは tombstone で終端する (契約 invariant)。

## スコープ外

- production code (reducer / gateway) は変更しない。
- pending-human-input replay (DP-PENDING-REPLAY) の実装はスコープ外。

## 受け入れ条件

- `cd src && go test ./host/state/... ./server/api/...` が green。
- first-writer-wins / TTL default / teardown connection-lost / 409 conflict / hello 非 replay の assert が存在する。

## 参照ツール

`reduce_approval_test.go` の既存パターンと mux gateway scenario テスト (fake agent) に倣う。

## max_diff_loc の改訂 (300 → 550)

実測 diff は 502 LOC (`reduce_approval_test.go` +97 / 新規 `human_input_contract_test.go` 405 行) で
既定 cap 300 を超えた。次の理由で cap を 550 へ改訂した (2026-07-26):

- 差分は**テストのみ**で production code を含まない。cap の根拠 (Bacchelli の defect 検出
  sweet spot 200-400 LOC) は review 負荷の指標であり、`contract-human-input-roundtrip` の
  7 witness を 1 ファイルで pin する構成は分割しても review 対象量が減らない。
- `human_input_contract_test.go` は単一 contract の witness 帰着という cohesive な責務であり、
  ファイル分割は「1 contract = 1 pin ファイル」の対応を壊す。
- AGENTS.md の「長さ制限は責務分割のヒューリスティックであり、cohesive な責務を損なうなら
  該当ルールを記録して調整する」に従い、task 単位で記録して調整した。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery: task commit が integration branch に存在し機械 gate が approved (cross-task review 3 ラウンド分の findings 修正済み)
{% /transition %}
