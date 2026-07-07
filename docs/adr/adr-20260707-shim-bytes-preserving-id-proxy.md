---
id: adr-20260707-shim-bytes-preserving-id-proxy
kind: adr
title: shim は 2 方向で id を方向別に扱う (downstream echo / upstream numeric 採番)
status: accepted
created: '2026-07-07'
tags:
- adr
- codex
- shim
- jsonrpc
owners: []
decision_makers:
- unknown
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: references, target: spec-20260707-codexclient-jsonrpc-id-opaque}
- {type: referencedBy, target: note-20260707-technical-jsonrpc-id-opacity}
source_paths:
- src/cmd/bridge/codex_app_server_shim.go
- src/platform/agent/codexclient/conn.go
summary: shim の downstream Conn は CLI が発行した id を bytes-preserving に echo し、upstream
  Conn は Conn.Request の内部 numeric 採番を継続する 2 方向対称の proxy 契約を pin する
updated: '2026-07-07'
---

# shim は 2 方向で id を方向別に扱う (downstream echo / upstream numeric 採番)

## Context

{% context %}
`src/cmd/bridge/codex_app_server_shim.go` は downstream (CLI) と upstream (real codex-app-server) の 2 本の Conn を橋渡しする proxy であり、両 Conn の Handler I/F を実装する。JSON-RPC 2.0 の request は方向を持ち、shim では以下の 2 パターンを区別する:

1. **client-initiated** (CLI → shim → real): CLI が `initialize` などを発行し、shim は upstream へ転送、reply を受けたら CLI へ返す
2. **server-initiated** (real → shim → CLI): real app-server が `applyPatchApproval` などを発行し、shim は downstream へ転送、reply を受けたら real へ返す

現行実装は `Handler.OnServerRequest(id int64, ...)` の naming に引きずられて 2 方向の役割の対称性が曖昧化しており、否定役指摘 (FR-d5 の方向名指し) にあるように「片方向だけ手当てして他方向を regression させる」曖昧性を残していた。

さらに JSON-RPC 2.0 で禁じられる id 型 (object / array) や JSON literal null が到来したときの扱いも仕様化されておらず、Conn.Run の silent drop 経路と shim の proxy 経路のどちらで判定するかが未決だった。
{% /context %}

## Decision

{% decision %}
shim は 2 本の Conn の Handler 実装を **方向別に対称構造** で書き分け、以下の proxy 契約を pin する:

**downstream → upstream** (CLI が initiator):

- downstream Conn の Handler で受けた `RequestID` (opaque bytes) を struct field に保存
- `upstream.Request(method, params)` を呼び、upstream 側の Conn が **内部で新規 numeric id を採番**して upstream へ送出
- upstream から result / error が返ったら `downstream.Reply(savedID, result)` / `downstream.ReplyError(savedID, msg)` で **元 bytes を echo**

**upstream → downstream** (real app-server が initiator):

- upstream Conn の Handler で受けた `RequestID` (opaque bytes) を struct field に保存
- `downstream.Request(method, params)` を呼び、downstream 側の Conn が **内部で新規 numeric id を採番**して downstream へ送出
- downstream から result / error が返ったら `upstream.Reply(savedID, result)` / `upstream.ReplyError(savedID, msg)` で元 bytes を echo

**upstream 側で downstream の元 id を保存しない** (DP-d4 Option A 採用): shim が方向反転側で新規 numeric 採番するのは、その方向の pending map key を Conn 内で int64 SSOT に閉じるため。片側でも id を透過に upstream に転送すると id 衝突の可能性と pending map の意味論が複雑化する (Option B の risk)。

**invalid id 型 (object / array) と JSON literal null の扱い**:

- Conn.Run 側で **透過的に log + skip** (別 ADR: `adr-20260707-codexclient-observability-log`)。shim の proxy 経路には持ち込まない
- shim は method + id 有りのメッセージのみを転送対象とし、id が null で method 空の invalid Request error response は proxy せず (fake / real 双方で観測されないケース) 上位で drop する

**Notification 経路 (id 未出現)** は shim も両方向で従来通り透過 (id 変換なし)。既存の `turn/*` / `thread/status/changed` / `item/agentMessage/delta` の broadcast 挙動 (Notification は initiator にのみ届く project memo) を維持する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- 方向を FR / ADR で名指しし、片方向手当てによる regression の counterexample を仕様レベルで排除する (spec `FR-002` / `FR-004`)
- shim 内で upstream / downstream それぞれの `codexclient.Conn` の pending map SSOT が int64 単独に保たれ、`adr-20260707-jsonrpc-id-opaque-forwarding` の SSOT 単一化と整合する
- invalid id / null id は Conn 側で観測可能に drop されるため、shim は「有効な id を持つ message」のみを扱えばよく、proxy ロジックの意味論が薄い (shim = 純粋なプロキシ層に留まる)
{% /consequence %}

{% consequence kind="negative" %}
- shim の `codexShimSession` 構造体に「in-flight な server-initiated request の元 RequestID」を保持する field が方向ごとに増える (実装は `map[int64]RequestID` を 2 個持つ形になる。map key は Conn の内部 numeric 採番)
- upstream 側で id が保存されないため、「downstream の元 id が upstream にも透過に見える」proxy を将来求めるユースケースは別途対応が必要 (現状要件はない)
{% /consequence %}

{% consequence kind="neutral" %}
- Notification 経路は id を持たないため本 ADR の対象外。既存 broadcast scope (`project_codex_appserver_broadcast_scope`) の semantics に触れない
- shim 内の frame-messaging 関連 tool call ハンドリング (agent_frames.*) は本 ADR の対象外。id の受け渡しは同じ opaque bytes 経路を通るため regression 面のみ考慮する
{% /consequence %}

## Alternatives

- **shim も upstream 側で downstream の元 id を保存する (完全透過)** — 却下 (DP-d4 Option B)。upstream の Conn が同一 id を再利用する semantics になり、Conn の pending map が破壊的な意味論 (同じ int64 が二度出現しうる) を許容せざるを得ず、int64 SSOT が壊れる。CLI 側は元 id で reply が返れば満足するので下流保存だけで FR は満たせる。
- **id 変換テーブルを shim に per-session で持ち、upstream 発信でも downstream の numeric 空間と upstream の numeric 空間を独立に管理する** — 却下。Conn 内の pending map と shim 内の変換 map で 2 段の bookkeeping になり境界重複を作る。Conn.Request の返り値経由で暗黙対応付ける現行構造を維持する方が SSOT が浅い。
- **invalid id (object / array) を shim で per-directional に検出して close する** — 却下 (DP-d5 Option B の shim 版)。fail-fast は本番 CLI の envelope 変種で危険 (silent drop の再来より悪い)。Conn 側で log + skip して観測可能に落とす方針を採用。
- **JSON literal null id を Notification として扱う** — 却下 (DP-d6 Option A の一部として)。JSON-RPC 2.0 では id=null は error response の semantics であり、Notification (id 欠如) と非等価。fake / shim の contract 差異を防ぐため、Conn で pending 解決を試みず observable な log で drop する経路 (別 ADR `codexclient-observability-log`) を採用。


{% transition from="proposed" to="accepted" date="2026-07-07" %}
user G1 承認
{% /transition %}
