---
change: change-20260707-codexclient-jsonrpc-id-opaque
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260707-codexclient-jsonrpc-id-opaque
kind: spec
title: codexclient JSON-RPC id opaque forwarding
status: implemented
created: '2026-07-07'
tags:
- codex
- shim
- jsonrpc
- bug-fix
owners: []
methodology: sdd
functional_requirements:
- id: FR-001
  statement: システムは、JSON-RPC 2.0 envelope の id フィールドを bytes-preserving な opaque 値として保持し、id
    が string / number / null / 欠如のいずれの形で到来しても Conn の read loop を継続しなければならない。
  priority: must
  rationale: 現行 `rpcMessage.ID *int64` は JSON-RPC 2.0 spec が許容する id 型を暗黙に否定しており、codex-cli
    0.142.5 の string id (`"initialize"`) を silent drop する根本原因。invariant として常時成立させる。
- id: FR-002
  statement: CLI (downstream) が string id を含む request を shim に送信したとき、システムは対応する reply
    の id フィールドに、受信した id の生 bytes と同一の値を書き出さなければならない。
  priority: must
  rationale: shim は downstream から見て『JSON-RPC server 役』であり、client が発行した id を bytes-preserving
    で echo する責務を負う。この invariant が保存されないと CLI は 10s timeout → exit_code=1 → 60s 後 initState
    reap でセッションが停止する。
- id: FR-003
  statement: Conn が発行した Request が pending として登録されている間、システムは同 Request の numeric id
    を int64 key として pending map から一意に照合し、対応する reply を待機 caller へ引き渡さなければならない。
  priority: must
  rationale: 自発 Request の id 採番権を Conn が単独で持つ限り、pending の SSOT は int64 のままで足りる (境界重複を作らない)。state_driven
    な invariant として書く。
- id: FR-004
  statement: upstream (real codex-app-server) が server-initiated request を任意の id 型で
    shim に送信したとき、システムは downstream への転送 request に新規 numeric id を採番し、downstream からの
    reply の result を、upstream から受け取った元 id の生 bytes に付けて upstream へ返さなければならない。
  priority: must
  rationale: shim の 2 本 Conn は方向で役割が反転するため、方向を FR で名指しして impl 時に片方向だけ手当てする曖昧性を排除する
    (否定役指摘 [approach / FR-d5])。
- id: FR-005
  statement: もし JSON-RPC 2.0 で禁じられた id 型 (object / array) や int64 に parse 不能な numeric
    id を含む message を Conn が受信した場合、システムは transport を close せず、method / raw id bytes
    / decoded 型を含む構造化 log を出力してそのメッセージのみを skip しなければならない。
  priority: must
  rationale: 本 bug は silent drop が 10s + 60s の複合遅延で症状化した観測欠落事案。同種の envelope 逸脱が再発しても
    /debug の初動が『server.log を grep』で終わる状態にする。
- id: FR-006
  statement: もし method が空で id が JSON literal `null` の message を Conn が受信した場合、システムは
    pending 解決を試みず、raw id / result / error / peer 識別子を含む構造化 log を出力して drop しなければならない。
  priority: must
  rationale: JSON-RPC 2.0 で id=null は『invalid Request に対する error response』の semantics
    であり、method 未出現の Notification とは非等価。両者を混同すると fake / shim の contract が食い違う (否定役指摘
    [FR-d10 / DP-d6])。
- id: FR-007
  statement: fake app-server (`src/client/runtime/subsystem/stream/fake/appserver.go`
    および `src/platform/agent/fakecodex/fakecodex.go`) が string id を含む request を受信したとき、システムは同
    id の生 bytes を Reply の id に echo しなければならない。
  priority: must
  rationale: 2 系統の fake は Handler I/F の直接実装者であり、片方だけ更新すると orchestrator/agent 系テストと
    stream backend 系テストで挙動が食い違う (否定役指摘 [scope.in_scope の列挙])。
- id: FR-008
  statement: システムは、id フィールドが JSON envelope に出現しない inbound message を Handler.OnNotification
    へのみ dispatch し、既存の `turn/*` / `thread/status/changed` 等の notification 経路を回帰させてはならない。
  priority: must
  rationale: id 型変更に便乗した notification 経路の regression は本 fix の scope 外。invariant として明示し、機械的
    caller 追従の際の破壊を禁じる。
non_functional_requirements:
- id: NFR-001
  type: compatibility
  criteria: wire-format / persistence 型は encoding/json のみを使用し、JSON-RPC 2.0 準拠の envelope
    を stdlib で round-trip できる
  measurement: go.mod 追加なし・T0 unit test で string/number/null/欠如 4 パターンの round-trip
    bytes 保存を assert
- id: NFR-002
  type: maintainability
  criteria: '`src/platform/agent/codexclient/conn.go` は 500 行以下、Conn の各メソッドは 80 行以下を維持する'
  measurement: golangci-lint (funlen / file length rule) が green
- id: NFR-003
  type: reliability
  criteria: codex-cli / codex-app-server 境界は fake + FakeVsReal + contract の 3 点セットを維持し、FakeVsReal
    失敗時は fake を修正する (assertion を弱めない)
  measurement: T0 unit + T2 shim contract + T2 opt-in FakeVsReal (build tag `e2e`
    + `AG_E2E_CODEX_BIN`) の 3 種テストが並列で green
- id: NFR-004
  type: usability
  criteria: Conn の silent drop 経路 3 種 (`json.Unmarshal` 失敗 / id parse 失敗 / pending
    miss) はいずれも raw bytes と decoded 型を含む構造化 log を出力し、fmt.Errorf 混入で呑まれない
  measurement: T0 unit で invalid envelope を注入し、log capture が『raw bytes を含む structured
    entry』を含むことを assert
- id: NFR-005
  type: maintainability
  criteria: 'id 型変更は Handler I/F を named type (例: `codexclient.RequestID`) に置換して、Handler
    実装 caller (backend / orchestrator/agent handler / claude-app-server / shim / fake
    × 2) の追従漏れをコンパイル時に検出できる'
  measurement: 1 caller でも signature が旧型のまま残ると `go build ./...` が非ゼロで終了する
- id: NFR-006
  type: performance
  criteria: id echo / normalize に伴う per-message の追加 heap allocation は数バイト以内 (json.RawMessage
    の slice header + 1 コピー相当) に収まり、shim proxy レイテンシへの寄与は無視できる
  measurement: T0 benchmark (Conn.Reply の allocs/op) が現行 int64 版の 2 倍以内
- id: NFR-007
  type: maintainability
  criteria: 本改修による `depguard` layer 境界違反 (platform → client / orchestrator の import
    逆流) を発生させない
  measurement: '`make lint` が green'
acceptance:
- id: AC-001
  given: shim (in-process) と fake app-server (stream/fake/appserver.go) が結線され、fake
    は string id を含む `initialize` request に対して同 id で reply する契約を保持している
  when: test client が shim の downstream ソケットに `{"id":"initialize","method":"initialize","params":{...}}`
    を送る
  then: shim は 10 秒以内に `{"id":"initialize","result":{...}}` を返し、reply の id は受信した bytes
    と等価である
  requirement_refs:
  - FR-002
  - FR-007
- id: AC-002
  given: Conn の rpcMessage を対象とする T0 unit test
  when: id が JSON string (`"initialize"`) / number (`42`) / null (`null`) / 欠如 の 4
    パターンで decode → encode される
  then: いずれの id 型でも Unmarshal → Marshal 後の id フィールドが元 bytes と等価に保存される
  requirement_refs:
  - FR-001
  - NFR-001
- id: AC-003
  given: Conn が transport を持ち、内部 nextID が任意 int64 状態にある
  when: 自発 Request が発行され、対応する reply が peer から到着する
  then: Conn は int64 pending map から該当 request を一意に解決し、待機 caller に result を渡す
  requirement_refs:
  - FR-003
- id: AC-004
  given: shim が upstream Conn と downstream Conn の 2 本を保持している
  when: upstream から任意 id (string / number) の server-initiated request が届く
  then: shim は downstream に新規 numeric id で転送 request を送り、downstream の reply を upstream
    の元 id で echo する
  requirement_refs:
  - FR-004
- id: AC-005
  given: build tag `e2e` かつ環境変数 `AG_E2E_CODEX_BIN` に real codex-cli 0.142.5 バイナリが指定されている
  when: fakecodex 側の real-cli e2e test が shim を挟んで CLI を子プロセスとして起動し、string id を含む
    `initialize` を送出する
  then: shim を透過して real codex-app-server から成功 reply が返り、reply の id が入力と bytes-preserving
    に一致する
  requirement_refs:
  - FR-002
  - FR-007
  - NFR-003
- id: AC-006
  given: Conn の read loop が任意の invalid envelope (id が object / int64 overflow / method
    空かつ id=null) を注入されている
  when: 該当メッセージが Conn.Run に到達する
  then: transport は close せず、method / raw id bytes / decoded 型を含む構造化 log entry が出力され、後続
    message の処理は継続する
  requirement_refs:
  - FR-005
  - FR-006
  - NFR-004
- id: AC-007
  given: 既存の notification 経路 (`turn/completed`, `thread/status/changed`, `item/agentMessage/delta`
    等) を受け取る Backend
  when: id 型変更後の Conn を通じて notification が届く
  then: Handler.OnNotification が従来通り呼ばれ、method / params の dispatch は id 型変更以前と等価である
  requirement_refs:
  - FR-008
- id: AC-008
  given: '`//go:build e2e` を外した通常 CI build'
  when: '`cd src && go test ./...` を実行する'
  then: shim + fake の T2 contract test が green で、real binary を必要とする FakeVsReal は自動
    skip される (build tag により exclude)
  requirement_refs:
  - NFR-003
relations:
- {type: implementedBy, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: referencedBy, target: adr-20260707-jsonrpc-id-opaque-forwarding}
- {type: referencedBy, target: adr-20260707-shim-bytes-preserving-id-proxy}
- {type: referencedBy, target: adr-20260707-codexclient-observability-log}
- {type: referencedBy, target: adr-20260707-fakevsreal-shim-inversion}
source_paths:
- src/platform/agent/codexclient
- src/cmd/bridge
- src/client/runtime/subsystem/stream/fake
- src/platform/agent/fakecodex
- src/client/runtime/subsystem/stream/backend.go
- src/orchestrator/agent/handler.go
- src/cmd/claude-app-server
- src/client/lib/codex/transcript
- src/client/runtime/subsystem/stream
summary: codexclient.Conn の JSON-RPC id を bytes-preserving な opaque 値として扱い、shim が任意
  id 型 (string / number / null) を透過して codex-cli 0.142.5 の initialize を transparent
  proxy する仕様
updated: '2026-07-07'
---

# Spec — codexclient JSON-RPC id opaque forwarding

## Overview

本 spec は、`src/platform/agent/codexclient/conn.go` の JSON-RPC framing 層と、それを利用する `src/cmd/bridge/codex_app_server_shim.go`、`src/client/runtime/subsystem/stream/fake/appserver.go`、`src/platform/agent/fakecodex/fakecodex.go` が、JSON-RPC 2.0 spec に準拠して **id フィールドを bytes-preserving な opaque 値として扱う** ことを義務化する。

背景は codex-cli 0.142.5 が `initialize` 等の request 発行時に **string id** (`{"id":"initialize", ...}`) を送るのに対し、現行 `rpcMessage.ID *int64` が JSON-RPC 2.0 の許容型 (string / number / null) を暗黙に否定し `json.Unmarshal` 失敗 → `continue` で silent drop → CLI の 10s timeout → exit_code=1 → 60s 後 initState reap → session status=stopped という多段遅延の観測欠落事案。前段 `/debug` で § responsibility-4 検査結果として「境界重複=Y / 境界空白=Y / 契約暗黙化=Y」の 3 型該当と判定され、局所 patch (string → int64 強制変換) は禁じ手、**SSOT を JSON-RPC 2.0 側に単一化** する方針が承認済み。

## Scope

対象は codexclient の JSON-RPC envelope 型と、その `Handler` I/F を実装する全 caller (shim / fake × 2 / stream backend / orchestrator agent handler / claude-app-server)。frame-messaging 機能そのもの、上流 codex-cli / codex-app-server バイナリ、走行中 daemon の再 build / redeploy 手順は本 spec の対象外 (別 task で追跡)。

## Requirements

{% req id="FR-001" %}
**id opaque preservation (invariant)** — Conn は JSON-RPC 2.0 envelope の id を bytes-preserving な opaque 値として保持する。string / number / null / 欠如 のいずれでも read loop を継続し、silent drop で応答を消失させない。この invariant は Conn が transport を持つ全期間にわたり成立する。
{% /req %}

{% req id="FR-002" %}
**downstream id echo (event-driven)** — shim は downstream (CLI) が発行した request の id (string / number) を bytes-preserving に保持し、対応する reply の id フィールドに **入力 bytes と等価** な値を書き出す。この invariant を破ると CLI は自 id を含む reply を受け取れず、10s + 60s の複合遅延で session が停止する。
{% /req %}

{% req id="FR-003" %}
**self-issued request pending (state-driven)** — Conn が自発的に発行した Request が pending map に登録されている間、Conn は numeric id (int64) を canonical key として pending map から reply を一意に照合する。string 正規化キーを間に挟まないことで採番権と解決権を Conn 内 1 段に閉じる。
{% /req %}

{% req id="FR-004" %}
**upstream inverse proxy (event-driven)** — upstream (real codex-app-server) が任意 id 型で server-initiated request を送ったとき、shim は downstream への転送 request に**新規 numeric id を採番**し、downstream の reply を upstream の元 id で echo する。方向名指しで FR-002 と対にする。
{% /req %}

{% req id="FR-005" %}
**observability of malformed envelopes (unwanted)** — JSON-RPC 2.0 で禁じられた id 型 (object / array) や int64 に parse 不能な numeric id を受けても transport を close せず、method / raw id bytes / decoded 型を含む構造化 log を出して skip する。silent drop は禁止。
{% /req %}

{% req id="FR-006" %}
**null id error response (unwanted)** — method 空 + id=null の response は pending 解決を試みず、raw id / result / error / peer 識別子を構造化 log に落として drop する。JSON-RPC 2.0 の invalid Request error response と Notification (id 欠如) を区別する。
{% /req %}

{% req id="FR-007" %}
**fake string id acceptance (event-driven)** — `stream/fake/appserver.go` と `platform/agent/fakecodex/fakecodex.go` の両 fake は、string id を含む request を受理し同 id の生 bytes を Reply の id に echo する。片方だけ更新すると stream backend 系と orchestrator/agent 系の挙動が食い違うため、契約は同時に更新する。
{% /req %}

{% req id="FR-008" %}
**notification regression prohibition (invariant)** — id フィールドを持たない (JSON envelope に id が出現しない) inbound message は Handler.OnNotification へのみ dispatch する。`turn/*` / `thread/status/changed` / `item/agentMessage/delta` 等既存 notification 経路の挙動を id 型変更に便乗して変えない。
{% /req %}

## Non-Functional Requirements

- **NFR-001 (compatibility)** — wire format は encoding/json のみ。JSON-RPC 2.0 準拠 envelope を stdlib で round-trip。
- **NFR-002 (maintainability)** — `conn.go` は 500 行以下 / 各 func 80 行以下 (AGENTS.md 既定 target)。id 変更に便乗した予防的 file 分割は行わない。
- **NFR-003 (reliability)** — 外部境界は fake + FakeVsReal + contract の 3 点セット。FakeVsReal 失敗時は fake を修正する (assertion を弱めない)。
- **NFR-004 (usability / observability)** — silent drop 経路 3 種にすべて構造化 log。
- **NFR-005 (maintainability)** — Handler I/F は named type (`codexclient.RequestID`) に置換し、追従漏れを compile 時検出。
- **NFR-006 (performance)** — id echo / normalize による per-message allocation は数バイト以内 (json.RawMessage の slice header + 1 コピー相当)。
- **NFR-007 (maintainability)** — depguard の layer 境界を維持 (platform → client / orchestrator の import 逆流禁止)。

## Non-Goals

- POST /api/sessions 504 (047fae39) の再修正
- frame-messaging 機能 (`agent_frames.*`) の追加改修
- 上流 codex-cli / codex-app-server 側 (forks/ 含む) のパッチ
- 走行中 daemon (PID 94953) の再 build / redeploy 運用 (別 task で ops 手順として追跡)
- id 型変更に便乗した codexclient 一般化 refactor (transport 抽象 / timeout モデル / error 型)

## Wire Model

- `codexclient.RequestID` は `json.RawMessage` を underlying とする named type。opaque な JSON id bytes を保持し、`bytes.Equal` 相当の等値比較 helper を持つ。
- `rpcMessage.ID` は `*RequestID` (nil = id フィールド未出現 = Notification)。JSON literal `null` は `RequestID` にそのまま格納 (`[]byte("null")`)。
- Conn の `pending map[int64]chan rpcMessage` は自発 Request 用にのみ用い、reply 到着時に wire id bytes を `strconv.ParseInt` して int64 key に還元する。parse 失敗はそのメッセージを FR-005 に従って log + skip。
- Handler I/F は `OnServerRequest(id RequestID, method string, params json.RawMessage)`。Reply / ReplyError も `RequestID` を受ける。

## Acceptance Criteria

{% acceptance id="AC-001" %}
shim (in-process) と fake app-server が結線された状態で test client が `{"id":"initialize","method":"initialize",...}` を downstream に送ると、10 秒以内に `{"id":"initialize","result":{...}}` が返り、reply の id は受信 bytes と等価 (FR-002 / FR-007)。
{% /acceptance %}

{% acceptance id="AC-002" %}
`rpcMessage` の T0 unit で string / number / null / 欠如 の 4 パターンについて Unmarshal → Marshal を往復させ、id フィールドの bytes 保存が成立する (FR-001 / NFR-001)。
{% /acceptance %}

{% acceptance id="AC-003" %}
Conn が自発 Request を発行した後 peer から reply が届くと、int64 pending map から該当 request が一意に解決され待機 caller に result が引き渡される (FR-003)。
{% /acceptance %}

{% acceptance id="AC-004" %}
shim が upstream から任意 id (string / number) の server-initiated request を受けたとき、downstream への転送は新規 numeric id で送出され、downstream の reply が upstream の元 id で echo される (FR-004)。
{% /acceptance %}

{% acceptance id="AC-005" %}
build tag `e2e` + `AG_E2E_CODEX_BIN` を与えたとき、fakecodex 側の real-cli e2e test が shim を挟んで real codex-cli 0.142.5 を driving し、string id 付き `initialize` が成功 reply として bytes-preserving に返る (FR-002 / FR-007 / NFR-003)。
{% /acceptance %}

{% acceptance id="AC-006" %}
Conn.Run に invalid envelope (id=object / int64 overflow / method 空 + id=null) を注入すると、transport は close せず、method / raw id bytes / decoded 型を含む構造化 log entry が出力され、後続 message は処理継続される (FR-005 / FR-006 / NFR-004)。
{% /acceptance %}

{% acceptance id="AC-007" %}
`turn/completed` / `thread/status/changed` / `item/agentMessage/delta` の notification が id 型変更後の Conn を通じても従来通り Handler.OnNotification へ dispatch される (FR-008)。
{% /acceptance %}

{% acceptance id="AC-008" %}
通常 CI build (`//go:build e2e` なし) で `cd src && go test ./...` が green になり、real binary を要する FakeVsReal は build tag 除外で自動 skip される (NFR-003)。
{% /acceptance %}

## Open Questions

現時点で spec 側の要件確定に不明点は残っていない。実装局面の判断 (log の rate limit 有無、benchmark の閾値など) は plan.md 側の Open Questions として追跡する。


{% transition from="draft" to="approved" date="2026-07-07" %}
G1 通過
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-07" %}
plan 完了
{% /transition %}

````
