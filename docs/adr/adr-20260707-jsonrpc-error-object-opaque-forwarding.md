---
id: adr-20260707-jsonrpc-error-object-opaque-forwarding
kind: adr
title: codexclient error object を bytes-preserving に扱い ReplyError に spec 準拠 code を強制する
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
- {type: references, target: adr-20260707-jsonrpc-id-opaque-forwarding}
- {type: references, target: adr-20260707-shim-bytes-preserving-id-proxy}
- {type: referencedBy, target: note-20260707-technical-jsonrpc-id-opacity}
source_paths:
- src/platform/agent/codexclient/conn.go
- src/cmd/bridge/codex_app_server_shim.go
- src/platform/agent/fakecodex/codex_real_cli_e2e_test.go
summary: codexclient.Conn.Request の error 返却を *RPCError 型に格上げして peer の error object bytes を保持し、Conn.ReplyRPCError を新設して proxy 側で verbatim forward。ReplyError も code=-32603 を自動付与して JSON-RPC 2.0 JSONRPCErrorError 準拠に。
updated: '2026-07-07'
---

# codexclient error object を bytes-preserving に扱い ReplyError に spec 準拠 code を強制する

## Context

{% context %}
`src/platform/agent/codexclient/conn.go` の `Conn.ReplyError(id, errMsg string)` は wire 上に `{"error":{"message":"..."}}` を書き出しており、v1 schema の `JSONRPCErrorError` が `required: ["code", "message"]` を宣言していることに反していた。codex-cli 0.142.5 (Rust) の serde untagged enum `JSONRPCMessage` は 4 variant (Request / Notification / Response / Error) を順に試行するため、`code` を欠く reply は `JSONRPCError` variant を fail し、他 3 variant にもマッチせず「`sent invalid JSON-RPC: data did not match any variant of untagged enum JSONRPCMessage`」で thread/start が deserialize 失敗、TUI bootstrap が落ちた。

さらに `Conn.Request` は peer からの error response を `fmt.Errorf("codexclient: %s error: %s", method, msg.Error)` で Go 側 error に文字列化しており、shim (`cmd/bridge/codex_app_server_shim.go`) が upstream の error を downstream に forward する経路で `ReplyError(id, err.Error())` を呼ぶと **upstream の code / data が捨てられ**、synthetic な -32603 wrap すら生成されない (Go 側では -32603 も付かず、message すら失っていた) 状態だった。

前段 `/debug` の §responsibility-4 検査で「契約の暗黙化=Y」と判定 (`ReplyError(id, errMsg string) error` API が JSON-RPC 2.0 準拠 error object を生成する invariant を signature に持たない)。ReplyError caller は 15 箇所 (orchestrator/agent × 6 / claude-app-server × 2 / bridge shim × 3 / stream fake × 2 / stream event × 1 / fakecodex × 1) と広く、局所 patch (shim だけ code を足す) は同一 pattern を他所に残す禁じ手。

本 ADR は {% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%} で id 側に確立した bytes-preserving forwarding 契約を **error object にも並列に敷く**、その決定を対象とする。
{% /context %}

## Decision

{% decision %}
`codexclient` layer で **JSON-RPC 2.0 error object も bytes-preserving に扱う** SSOT を確立する。次の 4 変更を 1 コミットで導入する:

1. **`type RPCError struct { Method string; Data json.RawMessage }`** を追加し `error` interface を実装。`Data` は peer の error object bytes を verbatim 保持する。`ErrorObject() json.RawMessage` accessor を公開。
2. **`Conn.Request` の error 返却を `*RPCError` に格上げ**。peer が JSON-RPC error response を返した場合、`Data` に msg.Error の raw bytes を verbatim 保存 (`append(json.RawMessage(nil), msg.Error...)`)。`Error()` の文字列表現は既存 `codexclient: <method> error: <json>` shape を維持し、既存 substring 依存 test を壊さない。timeout / transport error は素の `error` のまま (peer 由来の bytes が無いため)。
3. **`Conn.ReplyError(id, errMsg string)` を spec 準拠に修正**。`InternalErrorCode = -32603` (JSON-RPC 2.0 予約 code, Internal error) と message を含む JSONRPCErrorError body を書く。全 caller は signature 不変で挙動が正しくなる。
4. **`Conn.ReplyRPCError(id, err error)` を新設**。`errors.As(err, **RPCError)` で unwrap して peer の Data bytes を verbatim forward。fallback (`*RPCError` でない場合 / Data が空の場合) は `internalErrorObject` helper で -32603 + message.Error() を組む。shim proxy sites (`shimDownstreamHandler.OnServerRequest` / `shimUpstreamHandler.OnServerRequest`) は `ReplyError(id, err.Error())` を `ReplyRPCError(id, err)` に一括置換。

**Handler I/F は変更しない** (`ReplyError(id, string) error` signature は維持)。ReplyRPCError は proxy 経路の専用 API として追加。orchestrator/agent や claude-app-server (peer error を forward しない local-error 生成者) は ReplyError のまま挙動が正しくなる。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- **envelope 単位の spec 準拠が完成**する: id 側 ({% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%}) と error object 側の bytes-preserving forwarding が対で揃い、shim を経由した JSON-RPC 2.0 proxy が peer↔peer の payload を lossless に relay できる
- ReplyError 15 caller が signature 不変で自動的に spec 準拠になる (compile break なし)
- shim proxy 経路で peer の application-level error 詳細 (code / data) が downstream に伝わる。従来の -32603 wrap すら生成されずに envelope が壊れていた回帰 (thread/start が "invalid JSON-RPC" で落ちる) が、根本的に閉じる
{% /consequence %}

{% consequence kind="negative" %}
- `Conn.Request` の返り値 error の実型が `*fmt.wrapError` から `*RPCError` に変わる (interface `error` は不変なので compile break はないが、`fmt.Errorf` で wrap されているつもりのテストが type assertion で拾えなくなる可能性)。既存 substring 依存 test はそのまま通る (`Error()` 文字列 shape 維持)
- `ReplyRPCError` と `ReplyError` の 2 API が並存する。使い分けの規律 (proxy 経路 = ReplyRPCError / local error = ReplyError) を守らないと code=-32603 wrap で peer の code / data 詳細が失われる。godoc とテスト規律 (下記) で強制する
{% /consequence %}

{% consequence kind="neutral" %}
- 本 ADR は **error object の内部構造 (code の numeric range / data field の semantic)** を規定しない。JSON-RPC 2.0 spec §5.1 (`code = -32700..-32603` 予約範囲) の validation は Conn では行わず、peer 間で通す (bytes-preserving)
- `Conn.Request` の error が typed になったことで、caller が `errors.As` で `*RPCError` を掘り出して独自処理する道を開くが、本 ADR 範囲外
{% /consequence %}

## Alternatives

- **ReplyError の signature を `err error` に変える (dispatch を method 内で行う)** — 却下。15 caller の signature が同時に変わる破壊的変更で、`fmt.Errorf` 経由の caller が多く migration cost が高い。ReplyError と ReplyRPCError の 2 API 並存で proxy 経路のみを新 API に載せ替える方が範囲が閉じる。
- **`Conn.Request` の返り値を `(result, errBytes, err)` triple にする** — 却下。Go idiom (error interface) を捨ててまで 3 値を返す価値はない。`*RPCError` に格上げすれば `errors.As` で bytes を取り出せる。
- **ReplyError に `code int64` 引数を足す** — 却下。全 caller が code を明示指定する必要ができて migration cost が跳ね、code の semantic (どの code を何に使うか) を全 caller に押し付けることになる。デフォルト -32603 で足り、precise code が必要なら ReplyRPCError で peer bytes を forward する方が正しい。
- **shim だけ手動で code を足す (局所 patch)** — 却下 (§responsibility-4 で禁じ手判定済み)。ReplyError caller 15 箇所全てが同じ envelope 契約に依存するため、局所修正は同一 pattern を他所に残す。lib 側で SSOT 化するのが正しい。
- **ReplyError と Conn.Request をそのままにして、shim だけ独自の error forwarder helper を作る** — 却下。契約の暗黙化を shim 側にコピーするだけで、他 caller (別 CLI bridge / orchestrator dispatch 等) は依然 broken なまま。lib 側 SSOT が本質。

## テスト規律

新規 proxy / relay の error 経路には次の 3 assertion を必須化する:

- **spec 準拠 wire shape pin** — `ReplyError(id, msg)` の wire bytes に `code: <numeric>` と `message: <string>` の両方が JSONRPCErrorError schema を満たす形で存在すること (参照実装: `TestReplyError_WireShape` in `conn_internal_test.go`)
- **bytes-preserving forwarding pin** — `Conn.Request` が peer error に対して返す `*RPCError` の `ErrorObject()` bytes が peer 送信 bytes と bit-for-bit 一致、`Conn.ReplyRPCError(id, rpcErr)` の wire error object が同 bytes を verbatim 保持すること (参照実装: `TestRequest_ReturnsRPCError` / `TestReplyRPCError_ForwardsUpstreamErrorBytes`)
- **local error fallback pin** — `ReplyRPCError(id, errors.New(...))` (非 *RPCError) が code=-32603 + message text を含む spec 準拠 error object を出力すること (参照実装: `TestReplyRPCError_LocalErrorGetsInternalCode`)
- **FakeVsReal (real codex-cli 駆動)** — 真の regression trigger は real Rust serde deserializer の untagged enum validation なので、build tag `e2e` + `AG_E2E_CODEX_BIN` gated subtest で real codex-cli 0.142.5 が shim 越しに forward された error を受け取ったとき `"invalid JSON-RPC"` / `"did not match any variant"` を吐かないことを pin する (参照実装: `TestE2E_ShimInvertedDriving/shim_inverted_upstream_err` in `codex_real_cli_e2e_test.go`)


{% transition from="proposed" to="accepted" date="2026-07-07" %}
user 承認 (plan-* handoff せず根本対処)
{% /transition %}
