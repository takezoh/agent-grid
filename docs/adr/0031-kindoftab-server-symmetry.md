# ADR 0031 — `kindOfTab` を server `matchLogTab` と対称な判定規則にする (Web UI 問題1・2)

Status: Accepted (検出順は [ADR 0035](./0035-kindoftab-label-first-for-jsonl-transcripts.md) で path-first → label-first に反転)

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md), [ADR 0025](./0025-transcript-rest-backfill-then-ws-tail.md), [ADR 0035](./0035-kindoftab-label-first-for-jsonl-transcripts.md)
Related requirements: FR-001, FR-002, FR-003, FR-004

## Context

Web UI で EVENTS タブが空パネルになる (Web UI 問題1・2) 真因は、client 側の `src/client/web/src/components/LogTabs.tsx` の `kindOfTab` が event-log を解決できないこと。

driver 側 (`client/driver/view_builder.go` `EventLogTab`) は EVENTS タブを

- `Label = "EVENTS"`
- `Path  = "<eventLogDir>/<sid>.log"`
- `Kind  = state.TabKindText`

で載せる。一方サーバ側 (`src/server/web/transcript.go` `matchLogTab`) は

- `pathSuffixes = [".log", ".jsonl"]`
- `labelTokens  = ["events", "event-log"]`

を `strings.Contains` (部分一致) で判定して REST `/api/sessions/:id/event-log` を解決している。

ところが client の `kindOfTab` は

- path 末尾は `.transcript` / `.event-log` しか見ない (`.log` を未対応)
- label は exact-match (`l === "transcript"` / `l === "event-log"`) で見る (`"EVENTS"` を未対応)

となっており、判定規則が server と非対称。実 driver の EVENTS タブ (`label="EVENTS"` + `path=<sid>.log`) はどの分岐にも当たらず `null` を返し、`ContentArea` が描画されず空パネルになる。

plan-how の否定役はこの非対称を **major** で指摘し、初稿の「label exact-match `"events"` のまま `.log` だけ追加」案も実 label `"EVENTS"` には小文字化後マッチするが将来の label 揺れで死に枝化する懸念を提示した。

## Decision

`kindOfTab` に以下 2 系統を追加し、server `matchLogTab` と **対称化** する:

1. **path 末尾**: `.log` または `.jsonl` で終わる → `"event-log"`
2. **label 部分一致**: 小文字化後に `includes("events")` または `includes("event-log")` → `"event-log"`

追加分岐は既存の `.transcript` / `.event-log` path 判定の **後方** に置き、検出順 (path 優先) と既存マッピングの不変を保つ (FR-004)。

## Consequences

- client と server が同じタブを `event-log` と判定し、REST (`/event-log`) と表示の整合が取れる (FR-001 / FR-002)
- label を `includes` 化することにより実 driver の `"EVENTS"` が確実にマッチし、初稿 FR-002 の exact-match `"events"` 死に枝問題が解消される
- INFO 等が将来 `.log` で終わるパスを持つと event-log へ誤検出するリスクは残るが、driver の実 path 命名 (transcript → `.transcript` / event-log → `.log`) に依存する。driver 側の path 規約を docs 化する別 issue を起票する (本 ADR の Open follow-up)
- 変更面積は 2 分岐に限定され既存解決を壊さない (FR-004)

## Alternatives Considered

### label を exact-match `"events"` のまま `.log` だけ追加 (初稿)

却下: server は `Contains`、実 label は `"EVENTS"`。exact `"events"` は小文字化後マッチするが server との規則非対称が残り、将来 label 揺れで乖離する。`includes` で対称化する。

### path だけで判定し label は見ない

却下: driver が path 拡張を付けない構成変更に弱い。server が両方見るので client も両方見て対称を保つ。
