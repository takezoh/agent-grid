---
id: task-20260705-wire-fixtures-pipeline
kind: task
title: Go 生成 golden wire fixtures + vitest 消費 + CI diff gate
status: done
created: '2026-07-05'
priority: high
effort: medium
files_touched:
- src/server/web/wire_fixtures_test.go
- src/client/web/src/wire/testdata/
- src/client/web/src/wire/codec.test.ts
- src/client/web/src/wire/fixtures.ts
- .github/workflows/ci.yml
pr: null
tags:
- testing
- web
owners: []
relations:
- {type: partOf, target: change-20260705-test-harness}
source_paths:
- src/server/web/wire.go
- src/client/web/src/wire/
- docs/adr/adr-20260624-0021-frontend-wire-types-hand-written.md
summary: server/web が viewUpdate/output/control の canonical JSON fixture を生成、codec.test.ts
  が同一ファイルを消費、再生成 git diff --exit-code を CI に追加
updated: '2026-07-05'
change: change-20260705-test-harness
---

# Go 生成 golden wire fixtures + vitest 消費 + CI diff gate

## 責務

Go↔TS wire 契約の手動同期 (手書き `fixtures.ts`) を、Go 生成 fixture の共有 + CI diff gate に置換する
(spec FR-006 / AC-002, adr-20260705-wire-fixtures-pipeline)。

## 詳細手順

1. `src/server/web/wire_fixtures_test.go` を新設する: `-update` flag 付きで hello / viewUpdate
   (model / effort / status 変種を含む) / surface output (asciicast) / control の canonical JSON を
   `src/client/web/src/wire/testdata/*.json` に書き出し、flag 無しでは既存ファイルとの一致を assert する。
   fixture は commit する。
2. `codec.test.ts` を testdata JSON の読み込みに切り替え、全 entry の decode round-trip と viewUpdate
   専用 assert を維持する。`store/daemon` の replay テストにも同 fixture を入力する。
3. 手書き `fixtures.ts` を削除 (または testdata の thin loader に縮退) する。
4. CI (`ci.yml`) に「fixture 再生成 → `git diff --exit-code`」step を追加する (codex-schema-check と
   同方式)。web dist build ジョブ内に組み込み、追加ジョブは作らない。
5. ADR 修繕の実装反映: adr-20260624-0021 に references (本 pipeline ADR へ)、adr-20260624-0023 の
   supersede transition (adr-20260705-view-update-sessions-only の accept と同時に docs CLI で実施)。

## 前提

なし (独立して着手可能)。

## スコープ外

- wire 型の codegen 化 (adr-0021 の手書き mirror 判断は維持)
- gateway scenario e2e (task-20260705-gateway-scenario-e2e)

## 受け入れ条件

- Go 側フィールド追加 → fixture 未再生成で CI step が fail することを実証 (AC-002)
- vitest / go test が同一 fixture で green
- `make lint`、biome green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Started implementing Go-generated wire fixtures, TS consumption, and CI diff gate.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented fixture generation, TS consumption, CI drift gate, and related ADR updates.
{% /transition %}
