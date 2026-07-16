---
id: adr-20260707-codexclient-observability-log
kind: adr
title: Conn.Run の silent drop 経路 3 種を構造化 log で観測可能にする
status: accepted
created: '2026-07-07'
tags:
- adr
- codex
- jsonrpc
- observability
owners: []
decision_makers:
- unknown
relations:
- {type: partOf, target: change-20260707-codexclient-jsonrpc-id-opaque}
- {type: references, target: change-20260707-codexclient-jsonrpc-id-opaque}
source_paths:
- src/platform/agent/codexclient/conn.go
summary: json.Unmarshal 失敗 / invalid id 型 / pending map miss の 3 経路を silent drop から観測可能な構造化
  log に格上げし、同種の envelope 変種による再発を server.log grep で検出可能にする
updated: '2026-07-07'
---

# Conn.Run の silent drop 経路 3 種を構造化 log で観測可能にする

## Context

{% context %}
本 bug (`spec-20260707-codexclient-jsonrpc-id-opaque`) は「silent drop → 10s CLI timeout → exit_code=1 → 60s 後 initState reap → session=stopped」という多段遅延で症状化した典型的な観測欠落事案である。直接原因は `Conn.Run` の `json.Unmarshal` 失敗経路 (`if err := ...; err != nil { continue }`) が構造化 log を出さずにメッセージを消失させたこと。

現行 `Conn.Run` には silent drop が 3 経路ある:

1. `json.Unmarshal(data, &msg)` 失敗
2. `msg.Method == ""` かつ `msg.ID == nil` (method 空 + id 未出現)
3. `resolvePending` で pending map に該当 id が無いケース (`c.pending[id] == nil`)

id 型 SSOT を JSON-RPC 2.0 準拠に単一化 (`adr-20260707-jsonrpc-id-opaque-forwarding`) しても、新たな envelope 変種 (別 peer が array id を送る / int64 overflow numeric を送る / 遅延 reply が pending timeout 後に届く) が来たとき、silent drop に戻ると同じ /debug 費用が再発する。

否定役指摘 [observability_loss (target: FR-d9 / DP-d5)] と [observability_loss (target: components / FR 全般)] の両方が本 ADR で扱う対象。
{% /context %}

## Decision

{% decision %}
`Conn.Run` の silent drop 3 経路すべてに **構造化 log** を挿入し、transport は close せずに後続 message の処理を継続する (log + skip 方針)。log 出力先は既定で `slog.Default()`、Conn 生成時に logger を注入できるようにするかは plan-impl 側で最小 diff を優先して判断する (plan.md Open Questions)。

各経路の log entry:

- **decode error**: `{"event":"codexclient.decode_error","raw":<truncated bytes>,"err":<msg>}` — raw は log の肥大を防ぐため 512 バイトで truncate、`err` は `json.Unmarshal` 由来メッセージ
- **invalid id**: `{"event":"codexclient.invalid_id","raw_id":<bytes>,"method":<msg.Method>,"parse_err":<msg>}` — JSON-RPC 2.0 で禁じられた id 型 (object / array) または int64 parse 不能 numeric、および method 空 + id=null (invalid Request error response) のケース
- **pending miss**: `{"event":"codexclient.pending_miss","raw_id":<bytes>,"method":<msg.Method>,"result_len":len(msg.Result),"error_len":len(msg.Error)}` — id は parse できたが対応する自発 Request が居ないケース (timeout 済み / 二重 reply / peer bug)

3 経路とも:

- `continue` で後続 message 処理を継続 (fail-fast による transport close は却下)
- rate limit なし (本 PR では実装しない — plan.md Open Questions)
- `slog.LevelWarn` で出力

T0 unit test (`codexclient/conn_test.go`) で 3 経路それぞれに invalid envelope を注入し、log capture (`slogtest` 相当) が raw bytes を含む structured entry を含むことを assert する (`AC-006`)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- 同種の envelope 変種 (`array id`, int64 overflow, 遅延 reply) が将来到来しても `grep 'codexclient\.'` で server.log に痕跡が残り、/debug の初動が MITM 復号ではなく log 検索で完結する
- decode error / invalid id / pending miss を 3 event 名で区別することで、次に何を fix するべきかが直接分かる (spec `FR-005` / `FR-006` / `NFR-004`)
- silent drop の禁止は Conn の invariant として `FR-001` に格上げされているため、本 ADR が仕様と実装を橋渡しする
{% /consequence %}

{% consequence kind="negative" %}
- 3 経路とも per-message で slog attr を組み立てるため、正常経路にはない微小な allocation が invalid message 到来時のみ発生する (正常経路は不変)
- log 出力量が invalid envelope 頻度に比例して増える。悪意ある peer が大量に invalid message を送るケースでは rate limit が必要になるが、本 PR では実装しない (plan.md Open Questions)
- Conn に logger を注入する I/F を将来足す場合は API 表面が広がる (現状は `slog.Default()` を直接呼ぶ最小差分)
{% /consequence %}

{% consequence kind="neutral" %}
- 本 ADR は「silent drop の禁止」を Conn 内で規定するだけで、shim の proxy 経路や fake の Handler 実装には影響しない (別 ADR: `adr-20260707-shim-bytes-preserving-id-proxy` と `adr-20260707-fakevsreal-shim-inversion`)
- 既存の `fmt.Errorf` を返す経路 (Request timeout / write error) は本 ADR の対象外 (すでに caller が観測可能な error として扱っている)
{% /consequence %}

## Alternatives

- **transport を close する (fail-fast)** — 却下 (DP-d5 Option B)。本番 CLI の envelope 変種で shim が close すると SessionStatus=stopped が新たな回帰経路になり、silent drop より運用面のダメージが大きい。spec 純粋主義より本番互換を優先。
- **Handler に opaque で pass-through して呼び出し側判断** — 却下 (DP-d5 Option C)。責務分散で境界が曖昧化する。Conn は JSON-RPC framing の SSOT なので、envelope の妥当性判定を上位に投げると Handler 実装ごとに独自の drop 判定が入り込み、境界重複が復活する。
- **structured log の代わりに fmt.Errorf を event bus に emit する** — 却下。observability tool は `slog` に統一されつつあり、event bus 経由の error は集約先が分散する。単一の `slog.Default()` に落とす方が grep しやすい。
- **id=null の invalid Request error response を「Notification と同等」に扱う** — 却下 (DP-d6 Option A)。JSON-RPC 2.0 spec で id=null と id 欠如は非等価であり、fake / shim / real の contract が食い違うと `AC-006` が discriminative でなくなる。`pending_miss` event として log drop する経路に統一する。
- **rate limit を本 PR で入れる** — 却下 (plan.md Open Questions)。事案発生頻度が低く rate limit の必要性が現時点で観測されていないため、YAGNI。将来 rate limit を追加する場合も本 ADR の 3 event 名は互換に保つ。


{% transition from="proposed" to="accepted" date="2026-07-07" %}
user G1 承認
{% /transition %}
