---
id: task-20260705-driver-conformance-suite
kind: task
title: drivertest.Conformance + registry 走査テスト
status: done
created: '2026-07-05'
priority: high
effort: medium
files_touched:
- src/client/driver/drivertest/
- src/client/driver/conformance_test.go
pr: null
tags:
- testing
owners: []
relations:
- {type: partOf, target: change-20260705-test-harness}
source_paths:
- src/client/state/driver_iface.go
- src/client/driver/
- docs/adr/adr-20260705-metadata-source-priority.md
summary: Step 純粋性・DriverEvent totality・Persist/Restore round-trip・metadata source
  priority (ADR-20260705) を共通契約化し、state.Register 全 driver に自動適用
updated: '2026-07-05'
change: change-20260705-test-harness
---

# drivertest.Conformance + registry 走査テスト

## 責務

全 driver に共通の最低保証を suite として一元化し、registry 走査で新 driver の自動加入を強制する
(spec FR-005 / AC-001, adr-20260705-driver-conformance-registry-suite)。

## 詳細手順

1. `src/client/driver/drivertest/` に `Conformance(t *testing.T, drv state.Driver)` を新設する (T0):
   - **Step 純粋性**: 同一入力 2 回で同一出力、`prev DriverState` の deep-equal 不変
   - **DriverEvent totality**: 全 `DEv*` 種の zero-value 入力で panic しない
   - **Persist/Restore round-trip**: `NewState → Step 数回 → Persist → Restore` で `View`/`Status` 一致
   - **View/Status totality**: 到達可能な各 state で panic しない
2. **metadata source priority 契約** (adr-20260705-metadata-source-priority): driver ごとの
   authoritative source を差し替えるパラメータ (claude: hook / codex: `thread/settings/updated`) を
   suite に渡し、次を検証する:
   - authoritative 確定後、transcript / launch-parse fallback が model / effort を上書きできない
   - explicit clear (null / cleared) が Persist/Restore を跨いで維持される
   - 空文字と未設定が区別される (tri-state)
3. `conformance_test.go` に registry 走査テストを置く: `state.Register` 済みの全 driver を列挙して
   suite を適用する。新 driver は登録した瞬間に対象になる (NFR-001)。
4. 既存 driver が契約に違反する場合は、契約を緩めず driver 側を修正する (違反が設計意図なら ADR で明文化
   して suite にオプトアウトを実装する — ただし原則は全員加入)。

## 前提

なし (state / driver のみで完結、並行可)。

## スコープ外

- golden transcript replay (task-20260705-recorded-fake-fixtures)
- driver 固有挙動の個別テスト追加

## 受け入れ条件

- 登録済み全 driver (claude / codex / gemini / shell / generic) が suite を pass する
- registry に仮の新 driver を足すテストで suite が自動適用されることを確認
- coverage floor (state / driver) の非後退、`make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Started implementation of the driver conformance suite and Claude metadata contract fixes.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented common driver conformance suite, registry scan coverage, Claude metadata tri-state fix, and verified with tests plus lint.
{% /transition %}
