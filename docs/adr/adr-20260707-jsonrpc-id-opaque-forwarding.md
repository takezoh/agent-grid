---
id: adr-20260707-jsonrpc-id-opaque-forwarding
kind: adr
title: codexclient rpcMessage.ID を bytes-preserving な RequestID named type に置換
status: accepted
created: '2026-07-07'
tags:
- adr
- codex
- jsonrpc
- ssot
owners: []
decision_makers:
- unknown
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: references, target: spec-20260707-codexclient-jsonrpc-id-opaque}
- {type: referencedBy, target: note-20260707-technical-jsonrpc-id-opacity}
- {type: referencedBy, target: adr-20260707-jsonrpc-error-object-opaque-forwarding}
source_paths:
- src/platform/agent/codexclient/conn.go
- src/platform/agent/codexclient/client.go
summary: rpcMessage.ID *int64 を bytes-preserving な named type `codexclient.RequestID`
  (= json.RawMessage) に置換し、Handler I/F 全体を破壊的に更新して JSON-RPC 2.0 準拠に SSOT を単一化する
updated: '2026-07-07'
---

# codexclient rpcMessage.ID を bytes-preserving な RequestID named type に置換

## Context

{% context %}
`src/platform/agent/codexclient/conn.go` の `rpcMessage.ID *int64` は JSON-RPC 2.0 spec が許容する id 型 (string / number / null) を暗黙に否定している。codex-cli 0.142.5 は `initialize` 等の request で **string id** (`"initialize"`) を送るため、`json.Unmarshal` が失敗し `Conn.Run` の `continue` で silent drop される。CLI 側は 10s timeout で `failed to connect to remote app server` → exit_code=1 → 60s 後 initState reap → session status=stopped の多段遅延で症状化した。

前段 `/debug` の §responsibility-4 検査で「境界重複=Y / 境界空白=Y / 契約暗黙化=Y」の 3 型該当と判定され、局所 patch (string を受けたら int64 に強制変換) は禁じ手 (§responsibility-4)。SSOT を JSON-RPC 2.0 準拠側に単一化する必要がある。

Conn の Handler I/F (`OnServerRequest(id int64, ...)`, `Reply(id int64, ...)`, `ReplyError(id int64, ...)`) は shim / fake × 2 / stream backend / orchestrator agent handler / claude-app-server / 各 `*_test` を含めて **約 20 caller** が依存する公開契約であり、この境界を変更する判断が本 ADR の対象。
{% /context %}

## Decision

{% decision %}
`rpcMessage.ID` の in-process 表現を **`*codexclient.RequestID`** (`type RequestID json.RawMessage`, named type) に置換し、Handler I/F と Reply/ReplyError の signature を同 named type に破壊的に変更する。

- **named type** (alias ではなく) にすることで、Handler 実装 caller の型追従漏れが `go build ./...` の非ゼロ終了で検出できる (spec `NFR-005`)
- **json.RawMessage を underlying** にすることで、encoding/json は id フィールドを bytes-preserving で round-trip し、外部の JSON library 追加を不要にする (spec `NFR-001`)
- **1 コミットで全 caller を追従** させる (改善案 7 の実装順序): (1) codexclient に RequestID 追加 + rpcMessage.ID 変更 + T0 pin → (2) Handler I/F を破壊的変更 → 全 caller の signature を `RequestID` に一括置換

**Conn.pending map の SSOT は int64 のままとする** (改善案 2 採用)。理由は自発 Request の id 採番権を Conn が単独で持つ限り、pending 解決の SSOT は「Conn が採番した int64」1 段で足り、「wire bytes を canonical string 化した key」を挟むと 2 段変換になり境界重複を再生成するため。reply 到着時は wire id bytes を `strconv.ParseInt` して int64 化し、parse 失敗は本 ADR とは別 (observability ADR) の構造化 log 経路で drop する。

**外部 signature の維持**: `codexclient.Client.Request(method, params) (json.RawMessage, error)` / `Notify(method, params) error` / `Initialize(...)` などのユーザー向け API は現状のまま。内部で id を採番する経路は変わらない (numeric monotonic)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- JSON-RPC 2.0 準拠に SSOT が単一化される。同種の envelope 変種 (別 CLI が array id を送るなど) に対しても、追加の局所 patch なしに **観測可能な drop** (別 ADR 参照) で対応できる
- Handler I/F 変更は named type 置換で caller 追従漏れがコンパイル時に検出される (silent runtime 分岐が入り込む余地なし)
- named type は godoc と等値比較 helper のフックポイントを持つため、ecosystem 標準 (sourcegraph/jsonrpc2, gopls/internal/jsonrpc2) の JSON-RPC id 表現とも整合する
{% /consequence %}

{% consequence kind="negative" %}
- 約 20 caller の signature が同時に変わる (backend / orchestrator agent handler / claude-app-server / shim / fake × 2 / 各 `*_test`)。1 PR で追従が完結しない中間 commit は build 不能なので、commit 分割は「(1) codexclient 内 + T0 pin」と「(2) Handler I/F 破壊的変更 + 全 caller 追従」の 2 段に固定する
- 自発 Request が採番する int64 と、Handler が受ける RequestID (bytes) の 2 表現が Conn 内で共存する。Conn.Run の response 判定分岐で bytes → int64 parse を要するため、`conn.go` に +30〜50 行程度の実装が入る (500 行 target 内、責務分割は行わない — 改善案 8)
- json.RawMessage の named type ゆえに `bytes.Equal` を helper でラップする必要がある (Go の struct equality は使えない)
{% /consequence %}

{% consequence kind="neutral" %}
- 本 ADR は「invalid id 型 (object / array) / null id / pending miss の扱い」を含まない (別 ADR: `adr-20260707-codexclient-observability-log` と `adr-20260707-shim-bytes-preserving-id-proxy` が担当)。粒度細分は「個別に supersede される単位」の原則に従う
- fake × 2 の Handler 実装追従は各 fake の e2e / contract test で pin される (別 ADR: `adr-20260707-fakevsreal-shim-inversion`)
{% /consequence %}

## Alternatives

- **`interface{}` + type switch を Handler に渡す** — 却下。type switch が caller に散在するとテストハーネスが 3 表現ごとに書かれ、SSOT が「switch のバリエーション網羅」に霧散する (境界重複の再燃)。DP-d1 の Option C と同じ。
- **独自 struct `RequestID { raw json.RawMessage; kind {string,number,null} }` (kind enum つき)** — 却下。id の意味は wire bytes だけで決まり、kind を別 field で SSOT に持つと二重管理になる (bytes と kind の不整合を lint できない)。Handler I/F 変更の影響範囲もさらに広がる。DP-d1 の Option B。
- **既存 `int64` signature を残し `ReplyRaw(id RequestID, ...)` を追加、shim だけ新 API を使う (dual API)** — 却下。2 系統 API が並走し「どちらが SSOT か」が曖昧化する。Handler 実装のうち片方だけが旧型のまま残っても compile が通ってしまい `NFR-005` の compile-time 検出が破れる。DP-d3 の Option B。
- **Handler I/F は int64 のまま、string id の request は Conn 内部で新規 int64 に採番し直して bookkeeping** — 却下。CLI が受け取る reply の id が「Conn が採番した int64」に化けてしまい、CLI の `"initialize"` id と一致せずタイムアウトする (FR-002 と両立しない)。DP-d3 の Option C。
- **pending map key を string に変更 (要件案の第一候補)** — 却下 (改善案 2 採用)。自発 Request の id 採番権が Conn 単独である以上、pending 解決の SSOT は int64 1 段で足り、string 正規化を挟むと採番 int64 と wire bytes → string 正規化の 2 段変換が新たな暗黙契約を生む。DP-d2 の Option B。
- **json.number canonical 化ルール (1e0 と 1 と 1.0 の等価性) を仕様化して string key で照合する** — 却下。Go の encoding/json は `1e0` を書かないため自発 Request との衝突は起きないが、他言語 peer の spec 差に対して canonical 化を hand-rolling するのは overkill。int64 parse + parse 失敗の log drop で観測可能に落とす方針で足りる。
- **局所 patch (string → int64 強制変換)** — 却下 (§responsibility-4 で禁じ手判定済み)。SSOT を int64 側に残すため境界重複 (JSON-RPC 2.0 spec と in-process 型の乖離) が固定化する。


{% transition from="proposed" to="accepted" date="2026-07-07" %}
user G1 承認
{% /transition %}
