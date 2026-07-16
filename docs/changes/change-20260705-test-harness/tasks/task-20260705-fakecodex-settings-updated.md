---
id: task-20260705-fakecodex-settings-updated
kind: task
title: fakecodex / stream fake に thread/settings/updated を追加
status: done
created: '2026-07-05'
priority: high
effort: small
files_touched:
- src/platform/agent/fakecodex/presets.go
- src/platform/agent/fakecodex/fakecodex.go
- src/client/runtime/subsystem/stream/fake/appserver.go
- src/platform/agent/fakecodex/codex_real_cli_e2e_test.go
pr: null
tags:
- testing
- fake
owners: []
relations:
- {type: partOf, target: change-20260705-test-harness}
source_paths:
- src/client/runtime/subsystem/stream/event.go
- src/platform/agent/fakecodex/
- docs/adr/adr-20260705-metadata-source-priority.md
summary: authoritative metadata event を fake 両系統 (stdio/WS) の preset に追加し、FakeVsReal
  e2e で real codex の emit と照合する
updated: '2026-07-05'
change: change-20260705-test-harness
---

# fakecodex / stream fake に thread/settings/updated を追加

## 責務

commit 71d05a4 で顕在化した fake drift を解消する: `thread/settings/updated` は production
(`stream/event.go`) で authoritative metadata source として処理されるが、fake 両系統のどちらも emit
できず、fidelity backstop の照合対象にも入っていない (spec FR-002 の違反例)。

## 詳細手順

1. `fakecodex` (stdio): `presets.go` に `SettingsUpdatedHandler` (または既存 handler への合成) を追加し、
   `thread/settings/updated` を model / effort / null-clear の 3 バリエーションで emit できるようにする。
   method / event 集合契約 (`fakecodex.go` のリスト) に追加する。
2. `subsystem/stream/fake` (WS): appserver に同イベントの emit を追加し、`launch_flow_test.go` /
   `backend_test.go` 系で「settings/updated → `EvSubsystem` → driver の metadata 更新」の T1 伝搬を
   検証する。
3. FakeVsReal 拡張 (T3): `codex_real_cli_e2e_test.go` (または appserver e2e) で real codex に settings
   変更を発生させ、emit される `thread/settings/updated` の wire 形 (field 名 `effort` /
   `reasoning_effort` の揺れを含む) を fake の emit と照合する。real 側で誘発不能な場合は Skip 理由を
   明記し、schema (codexschema) 照合で代替する。
4. e2e が落ちた場合は fake を直す (fix-the-fake 原則)。

## 前提

なし (独立して着手可能)。

## スコープ外

- driver 側の metadata 処理変更 (71d05a4 で実装済み)
- 録音駆動化 (task-20260705-recorded-fake-fixtures)

## 受け入れ条件

- fake 両系統が settings/updated を emit でき、T1 伝搬テストが green
- `make test-e2e` (AG_E2E_CODEX_BIN 設定時) で FakeVsReal 照合が green
- `go vet -tags e2e` green、`make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Started implementation and verification for fake thread/settings/updated coverage.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented fake stdio/WS thread/settings/updated coverage and verification.
{% /transition %}
