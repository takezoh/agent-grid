---
id: adr-20260624-0035-kindoftab-label-first-for-jsonl-transcripts
kind: adr
title: ADR 0035 — `kindOfTab` の検出順を label-first に反転して `.jsonl` TRANSCRIPT 衝突を解消
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0035-kindoftab-label-first-for-jsonl-transcripts.md -->

# ADR 0035 — `kindOfTab` の検出順を label-first に反転して `.jsonl` TRANSCRIPT 衝突を解消

Status: Accepted

Related: [ADR 0031](../adr/adr-20260624-0031-kindoftab-server-symmetry.md) (path-first を採用していたが本 ADR で反転), [spec](../specs/2026-06-24-web-ui-fixes/spec.md)

## Context

ADR 0031 で `kindOfTab` に `.log` / `.jsonl` path 末尾 → `event-log` ルールを追加し、検出順は **path 優先 → label fallback** とした。

しかし実 driver の TRANSCRIPT タブは:

- Claude (`client/driver/claude.go` `resolveTranscriptPath`): `~/.claude/projects/<dir>/<sid>.jsonl`
- Codex: rollout JSONL (path 末尾 `.jsonl`)

を載せる。すなわち TRANSCRIPT タブの path 末尾も `.jsonl` であり、ADR 0031 で追加した `.jsonl → event-log` ルールに **path 優先順** で先に拾われてしまう。結果として:

- TRANSCRIPT タブ → kindOfTab が `event-log` を返す → REST `/api/sessions/:id/event-log` を取得 → EVENTS タブと同じ内容を描画
- EVENTS タブ → 同じく `event-log` → 同じ REST を取得

UI 上は TRANSCRIPT / EVENTS どちらをクリックしても EVENTS ログが見える状態だった。

サーバ側 `matchLogTab` (`src/server/web/transcript.go`) は既に **labelTokens を先に走査 → pathSuffixes を後で走査** の順で動作しており、`label="TRANSCRIPT"` を `kindMatch="transcript"` で正しく解決している。クライアントのみ非対称が残っていた。

## Decision

`kindOfTab` の検出順を **label 優先 → path fallback** に反転する。

```ts
// 1. label が "transcript" を含む → "transcript"
// 2. label が "events"/"event-log" を含む → "event-log"
// 3. path が ".transcript" で終わる → "transcript"
// 4. path が ".event-log" で終わる → "event-log"
// 5. path が ".log" or ".jsonl" で終わる → "event-log"
```

これにより:

- Claude/Codex TRANSCRIPT (label="TRANSCRIPT", path=`*.jsonl`) は **1.** で transcript に解決
- EVENTS (label="EVENTS", path=`*.log`) は **2.** で event-log に解決
- driver が label を付けない・揺れる将来構成では従来通り path-suffix fallback (3–5) で解決

サーバ `matchLogTab` の走査順 (labelTokens → pathSuffixes) と完全対称になる。

## Consequences

- 観測バグ「TRANSCRIPT・EVENTS タブが同じ EVENTS 内容を表示する」が解消する
- ADR 0031 の Decision §1 の検出順 (path 優先) は本 ADR で **反転**。`.log` / `.jsonl` path 末尾の event-log 解決ルール自体は残り、label fallback で拾えない driver でも `.jsonl` ログを event-log にマップできる
- INFO 等のラベルに「transcript」「events」を含まない限り誤検出は起きない。サーバと検出順が揃ったため、片側だけ将来直しても乖離しにくくなった
- 変更は `kindOfTab` の if 文の順序のみで、新規分岐は追加しない (LogTabs.tsx の差分は最小)

## Alternatives Considered

### explicit `kind` field (`LogTab.kind` が `"transcript"`/`"codex_transcript"`/`"text"`) を最優先で見る

却下: kind は driver が stamp する文字列 (`"text"` / `"transcript"` / `"codex_transcript"`) で wire 形 (`src/client/web/src/wire/server.ts`) が `string` 型のため安定キーになるが、サーバ `matchLogTab` は kind を参照していない。client だけ kind 優先にすると client/server の判定軸が再び非対称になる。

### `.jsonl` ルールを削除して transcript 側へ寄せる

却下: サーバ `matchLogTab` の `pathSuffixes=[.log, .jsonl]` を変更せず client だけ変えると ADR 0031 の対称化が崩れる。サーバ修正は wire/REST に影響するため本変更のスコープ外。
