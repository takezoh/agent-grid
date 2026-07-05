---
id: adr-20260705-wire-fixtures-pipeline
kind: adr
title: 'Cross-language wire fixtures: Go-generated, TS-consumed, CI-gated'
status: proposed
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- testing
- web
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: references, target: adr-20260624-0021-frontend-wire-types-hand-written}
source_paths:
- src/server/web/wire.go
- src/server/web/wire_test.go
- src/client/web/src/wire/
- .github/workflows/ci.yml
summary: server/web が golden fixture JSON を生成し vitest が同一ファイルを消費、再生成 diff を CI で gate
  して Go↔TS wire drift を機械検出する (ADR 0021 の未実装機構を置換)
---

# Cross-language wire fixtures: Go-generated, TS-consumed, CI-gated

## Context

{% context %}
adr-20260624-0021 は frontend wire 型を手書き mirror とし、drift 検出として「Go helper が
`src/client/web/src/wire/testdata/` に fixtures を生成し `wire_fixtures_test.go` が検出する」機構を
規定した。しかしこの機構は**実装されていない**。実際には `src/client/web/src/wire/fixtures.ts` が手書きで
「Go 側 wire_test.go と byte-for-byte 一致」を人手同期しており、直近の `model` / `effort` フィールド追加
(commit 71d05a4) でも `server.ts` への手動 +2 行が発生した。人手同期は追加漏れ・型齟齬を CI で検出できない。

一方、リポジトリには既に機械式の cross-boundary drift gate の成功例がある: codex-schema-check は committed
schema bundle と real codex の出力を regen + diff で照合する。
{% /context %}

## Decision

{% decision %}
codex-schema-check と同じ「生成 + diff gate」方式で Go↔TS wire 契約を機械化することにする:

1. **生成 (Go が正)** — `server/web/wire_fixtures_test.go` が viewUpdate / surface output (asciicast) /
   control / hello の canonical JSON fixture を `src/client/web/src/wire/testdata/*.json` に書き出す
   (`-update` flag または専用 go run helper)。fixture は commit する。
2. **消費 (TS)** — `codec.test.ts` は手書き `fixtures.ts` の代わりに同一の testdata JSON を読み、decode
   round-trip と型整合を assert する。store replay テストも同じ fixture を入力に使う。
3. **CI gate** — CI step で fixture を再生成し `git diff --exit-code` で drift を fail させる。
4. adr-20260624-0021 の「型は手書き mirror」という核心判断は維持する。本 ADR が置換するのは drift 検出
   機構のみ (0021 は amend 扱いで references link を張る)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
Go 側 wire 変更が fixture 経由で TS 側テストへ機械伝搬し、手動同期漏れが CI で止まる。server=>view の契約が
「commit された fixture」という単一の物理的正本を持つ。
{% /consequence %}

{% consequence kind="negative" %}
wire 変更 PR は fixture 再生成の 1 手順が増える。fixture の網羅性 (どの event 種を含めるか) は依然人間の
判断で、含めなかった形状の drift は検出されない。
{% /consequence %}

## Alternatives

- **codegen で TS 型自体を生成する** — 却下。adr-0021 が却下済みの方向 (生成器の保守コスト、biome / TS
  idiom との不整合)。fixture 照合は型を手書きに保ったまま契約だけを機械化する。
- **JSON Schema を中間表現にする** — 却下。wire 層は stdlib-only (encoding/json) の制約があり、schema
  ライブラリ導入は depguard 方針と衝突する。fixture は追加依存ゼロ。
- **手書き fixtures.ts の継続 (現状維持)** — 却下。71d05a4 で実際に手動同期が発生しており、漏れの検出
  手段が無い。
