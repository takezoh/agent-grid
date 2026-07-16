---
id: task-20260705-fuzz-reduce
kind: task
title: 'FuzzReduce: ランダム event 列に対する Reduce 大域不変条件'
status: done
created: '2026-07-05'
priority: low
effort: small
files_touched:
- src/client/state/reduce_fuzz_test.go
- .github/workflows/ci.yml
pr: null
tags:
- testing
owners: []
relations:
- {type: partOf, target: change-20260705-test-harness}
source_paths:
- src/client/state/reduce.go
- src/client/state/state.go
summary: stdlib fuzz で event 列を Reduce に fold し、no-panic / HeadFrameID 整合 / MRU 整合
  / 旧 State 不変を assert
updated: '2026-07-05'
change: change-20260705-test-harness
---

# FuzzReduce: ランダム event 列に対する Reduce 大域不変条件

## 責務

個別 reducer テストが到達しない event 順序の組合せを fuzz で覆う (spec FR-012)。T2 tier。

## 詳細手順

1. `src/client/state/reduce_fuzz_test.go` に stdlib fuzz を新設する。fuzz 入力 bytes を event 列に
   デコードする小さな generator を書き (event 種 + 主要フィールドの enum 選択)、初期 State から fold する。
2. 各 step で大域不変条件を assert する:
   - `Reduce` が panic しない (登録済み event 種のみを生成する — default panic は未実装検知用に維持)
   - 全 Session の `HeadFrameID` が `Frames` 内に存在する (または空の規約に従う)
   - `MRUFrameIDs` が実在 frame のみを指し重複しない
   - 入力 State が変異していない (fold 前 snapshot と deep-equal — copy-on-write 検証)
   - 返る Effect が closed sum の範囲内
3. seed corpus に既存 regression (persist 系など) の event 列を数本入れる。
4. CI fuzz job (`FuzzStreamRouting` / `FuzzApplyInboundProto` と同じ 30s 枠) に追加する。

## 前提

なし (state package のみで完結)。

## スコープ外

- property-based ライブラリ (rapid 等) の導入 — event 生成が複雑化した将来に再検討
- driver 内部 state の fuzz (conformance suite が担う)

## 受け入れ条件

- fuzz が 30s green + 発見した不変条件違反があれば修正 or issue 化して seed に固定
- `make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Started implementation
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented fuzz reducer invariants and verified with tests
{% /transition %}
