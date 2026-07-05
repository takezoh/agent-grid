---
id: task-20260705-runtimetest-harness
kind: task
title: runtimetest loop harness + EventSink seam
status: done
created: '2026-07-05'
priority: high
effort: medium
files_touched:
- src/client/runtime/runtimetest/
- src/client/runtime/tap_manager.go
- src/client/runtime/runtime.go
pr: null
tags:
- testing
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependencyOf, target: task-20260705-tap-osc-contract}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/client/runtime/runtime.go
- src/client/runtime/backends.go
- src/client/runtime/subsystem/stream/backend.go
summary: 全 fake backend 注入済み Runtime を実起動し Enqueue/WaitFor/Quiesce を提供する loop harness
  と、tap_manager の enqueue 先 EventSink seam 化
updated: '2026-07-05'
---

# runtimetest loop harness + EventSink seam

## 責務

runtime loop の shell テストを ad-hoc 起動から共通 harness に統一する土台を作る。他 task (tap contract 等)
の前提となる基盤 task。

## 詳細手順

1. `src/client/runtime/runtimetest/` package を新設する (T1 tier)。提供 API:
   - `New(t *testing.T, opts ...Option) *Harness` — noop/recording backend 群
     (`noopBackend`/`noopPersist` 等既存のものを再利用) を注入済みの `Runtime` を構築し
     `Run(ctx)` を goroutine で実起動、`t.Cleanup` で shutdown する
   - `Enqueue(ev state.Event)` — event の投入
   - `WaitFor(t, func(state.State) bool)` — published snapshot (`atomic.Pointer[State]` の公開読み口)
     を poll する同期プリミティブ (timeout 付き)
   - `Quiesce()` — eventCh / internalCh の消化完了を待つ
   - Option で個別 backend (recordingBackend 等) の差し替えを許す
2. `tap_manager.go` の enqueue 先を `EventSink` interface (`Enqueue(state.Event)`) に seam 化する
   (adr-20260705-eventsink-seam-tap-relay-contracts の Decision 1)。production 挙動は不変。
3. harness を使う新規シナリオを 2 本追加する:
   - `RequestShutdown` の timeout 経路 (wedged loop 相当を fake backend の block で再現)
   - `eventCh` 満杯時の非ブロッキング drop (`Enqueue` の drop 分岐)
4. 既存の spawn 系テスト (`spawn_complete_test.go` 等) のうち 1〜2 本を harness に移行し、API の妥当性を
   確認する (全面移行はスコープ外)。

## 前提

なし (最初に着手可能)。

## スコープ外

- tap contract 本体 (task-20260705-tap-osc-contract)
- 既存 shell テストの全面 harness 移行 (機会あり次第の漸進で良い)

## 受け入れ条件

- `cd src && go test ./client/runtime/...` green、`make test-race` green
- 新規 2 シナリオが harness 経由で決定的に pass する
- `make lint` green (depguard: runtimetest は runtime 内なので追加ルール不要)


{% transition from="todo" to="in_progress" date="2026-07-05" %}
implementation and verification completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
implemented harness, EventSink seam, and acceptance tests
{% /transition %}
