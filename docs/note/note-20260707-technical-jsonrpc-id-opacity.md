---
id: note-20260707-technical-jsonrpc-id-opacity
kind: note
title: JSON-RPC envelope opacity invariants for proxy/relay code (id + error object)
status: published
created: '2026-07-07'
tags:
- technical
- invariant
- jsonrpc
owners: []
relations:
- {type: references, target: adr-20260707-codexclient-observability-log}
- {type: references, target: adr-20260707-fakevsreal-shim-inversion}
- {type: references, target: adr-20260707-jsonrpc-error-object-opaque-forwarding}
- {type: references, target: adr-20260707-jsonrpc-id-opaque-forwarding}
- {type: references, target: adr-20260707-shim-bytes-preserving-id-proxy}
source_paths:
- src/platform/agent/codexclient
- src/cmd/bridge
- src/host/runtime/subsystem/stream/fake
- src/platform/agent/fakecodex
summary: JSON-RPC 2.0 proxy / relay / bridge を書くとき、envelope の id と error object は
  ともに bytes-preserving で扱う (int64 決め打ち禁止 / message-only error 禁止)
updated: '2026-07-07'
---

## Summary

このリポで JSON-RPC 2.0 の proxy / relay / bridge を書く / 変更する / review する全員が守るべき **2 つの並列 invariant** を 1 本にまとめる:

1. **id opacity** — envelope の `id` フィールドは opaque bytes として保持し、reply 時は受信 bytes をそのまま echo する。`int` / `int64` / 独自 numeric named type で decode してはならない。
2. **error object opacity** — envelope の `error` フィールドは、peer から forward する経路では bytes-preserving に扱い、local 生成する経路では JSON-RPC 2.0 spec §5.1 の `JSONRPCErrorError` 準拠 (`code` + `message` 必須) で組む。`{"error": {"message": "..."}}` のような code 欠落 shape を書いてはならない。

背景は 2026-07-07 の codex-app-server-shim 2 段事案:

- 第 1 段 ({% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%}): CLI の string id `"initialize"` が int64 unmarshal 失敗で silent drop
- 第 2 段 ({% adr-ref id="adr-20260707-jsonrpc-error-object-opaque-forwarding" /%}): 第 1 段 fix 後、実 request が upstream に届くようになった結果、`ReplyError` が生成する code 欠落 error object を codex-cli が `"data did not match any variant of untagged enum JSONRPCMessage"` で reject し thread/start が落ちた

将来の proxy 追加 (別 CLI 対応 / claude-app-server / orchestrator dispatch 等) で同種の envelope 変種落とし穴を踏まないための SSOT。

## Notes

### 1. 不変条件 (invariant)

- envelope の `id` フィールドの in-process 表現は **`json.RawMessage` を underlying とした named type** (参照実装: `codexclient.RequestID`)。
- reply / renumber を跨いだ id の外部観測値は **受信 bytes と bit-for-bit 一致** する (bytes-preserving)。
- 内部で id を採番する必要がある経路 (Conn.pending map 等) では、pending key を int64 のまま保持することは許されるが、**wire に載る bytes は必ず opaque な RawMessage** とし、int64 ↔ bytes の変換は境界の 1 点に閉じる。
- id 型は 4 shape を許容する: **string / number (int64 内) / number (int64 overflow) / null**。いずれの shape も silent drop してはならない (drop 経路は必ず observable log を伴う)。

### 2. 根拠 (Why)

JSON-RPC 2.0 spec §4 は `id` を `String, Number, or NULL value` と規定しており、int64 決め打ちは **spec 準拠 peer からの string id を silent drop** する。実際に codex-cli 0.142.5 は `initialize` request で `{"id":"initialize", ...}` を送るため、`rpcMessage.ID *int64` の unmarshal 失敗 → `Conn.Run` の `continue` → CLI 側 10s timeout → exit_code=1 → 60s 後 initState reap → session status=Stopped の**多段遅延で症状化**した。UI 側の 504 fix (047fae39) では観測不能だったのはこの多段遅延と silent drop 経路が同居していたため。

**変換 (string ↔ int) は情報損失**を伴う: int64 overflow numeric id (`999999999999999999999`) と string id (`"initialize"`) の両方を int64 に丸めると round-trip 不能で、shim が「元に戻せる別表現」を持てなくなる。opaque bytes forwarding が唯一の安全解。

### 3. 適用範囲 (Where)

現時点で本 invariant が適用される JSON-RPC 境界:

- `src/platform/agent/codexclient/conn.go` — 参照実装 (`RequestID = json.RawMessage` の named type / pending map int64 key + reply 時 `strconv.ParseInt` 照合)
- `src/cmd/bridge/codex_app_server_shim.go` — 2 方向 proxy (downstream reply は bytes echo / upstream server-initiated request は shim 側で renumber して echo back)
- `src/host/runtime/subsystem/stream/fake/appserver.go` / `cli.go` — fake app-server / cli の Handler impl
- `src/platform/agent/fakecodex/fakecodex.go` — string id を受け付ける fake codex

**将来追加する境界も本 note を継承**する: claude-app-server 側の app-server 化 / orchestrator の 別 driver 対応 / MCP transport 追加 / 別 CLI (Anthropic 系 / OpenAI 系) の bridge — いずれも proxy / relay を書く時点で本 invariant を満たすこと。

### 4. 参照実装

`src/platform/agent/codexclient/conn.go` を canonical とする。次の 3 要素が揃っていることを確認:

1. `type RequestID json.RawMessage` の named type 定義 (alias ではない — caller の型追従漏れを `go build ./...` の非ゼロ終了で検出できるようにするため)
2. `rpcMessage.UnmarshalJSON` が「id 未出現 (`Request` は id 必須) / id: null / id: bytes」の 3 状態を明示的に区別する (spec で notification は id 無し)
3. Handler I/F (`OnServerRequest` / `Reply` / `ReplyError`) の signature が全て `RequestID` を取る (`int64` を残さない)

### 5. テスト規律

新規 proxy / relay に対しては次の 5 assertion を必須化する:

- **4 shape round-trip pin** — string id (`"initialize"`) / small numeric id (`1`) / int64 overflow numeric id (`999999999999999999999`) / null id が bytes-preserving で return するテスト
- **silent drop 経路 3 種の structured log pin** — `codexclient.decode_error` / `codexclient.invalid_id` / `codexclient.pending_miss` の 3 event の log 実在を substring assertion で pin ({% adr-ref id="adr-20260707-codexclient-observability-log" /%})。「例外が投げられないこと」だけの assertion は禁止 (silent drop を隠す)。
- **FakeVsReal** — 相当の fake が real peer (codex-cli / codex-app-server) と wire 互換であることを、build tag `e2e` + 環境変数 (例: `AG_E2E_CODEX_BIN`) で real binary を driving する反転方向テストで pin ({% adr-ref id="adr-20260707-fakevsreal-shim-inversion" /%})。fake だけ通しても [[fake-is-not-authoritative]] の教訓により**保証にならない**。

### 6. 拡張 invariant: error object opacity

第 1 段 fix で id が届くようになった結果、shim の `ReplyError(id, err.Error())` が `{"error":{"message":"..."}}` (code 欠落) を書き出す既存 bug が露見した ({% adr-ref id="adr-20260707-jsonrpc-error-object-opaque-forwarding" /%})。

#### 6.1 不変条件

- `codexclient.Conn.ReplyError(id, errMsg string)` は wire 上に **必ず `code` + `message` を含む JSONRPCErrorError**  を書く。code のデフォルトは `codexclient.InternalErrorCode = -32603` (JSON-RPC 2.0 予約 code, Internal error)。
- **proxy 経路** (upstream error を downstream に relay する経路) では `codexclient.Conn.ReplyRPCError(id, err)` を使う。`err` が `*codexclient.RPCError` (Conn.Request が peer error に対して返す typed error) なら peer の error object bytes を **verbatim forward**。それ以外 (local timeout / transport error 等) は `-32603 + err.Error()` の spec 準拠 body を fallback で組む。
- `Conn.Request` は peer error response を受けたら `*RPCError{Method, Data}` を返す。`Data json.RawMessage` は peer 送信 bytes と bit-for-bit 一致。`err.Error()` の文字列表現は既存 `codexclient: <method> error: <json>` shape を維持。
- id opacity と同じく、**変換 (JSON error object ↔ Go 側 formatted string) は情報損失**を伴う (code / data / structured message が捨てられる)。opaque bytes forwarding が唯一の安全解。

#### 6.2 適用範囲

- `src/platform/agent/codexclient/conn.go` — 参照実装 (`RPCError` typed error / `ReplyError` code auto-fill / `ReplyRPCError` bytes-preserving forwarder)
- `src/cmd/bridge/codex_app_server_shim.go` — 2 方向 proxy の error 経路は `ReplyRPCError` のみ (`ReplyError(id, err.Error())` は禁止)
- 他 caller (orchestrator/agent / claude-app-server 等) は local error 生成者なので `ReplyError` のまま (code は自動的に -32603 に fill される)

#### 6.3 参照実装のテスト規律

- **wire shape pin** — `TestReplyError_WireShape` が ReplyError の wire bytes に code (numeric) + message (string) が両方存在することを assert
- **bytes-preserving forwarding pin** — `TestRequest_ReturnsRPCError` + `TestReplyRPCError_ForwardsUpstreamErrorBytes` で peer error 生 bytes の round-trip を assert
- **local fallback pin** — `TestReplyRPCError_LocalErrorGetsInternalCode` で非 *RPCError に対する -32603 fill を assert
- **FakeVsReal (real codex-cli)** — `TestE2E_ShimInvertedDriving/shim_inverted_upstream_err` で real codex-cli 0.142.5 が shim 越しに forward された error を受け取り "invalid JSON-RPC" を吐かないことを pin

### 7. 関連 ADR

- {% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%} — id opacity (codexclient.Conn の SSOT 変更)
- {% adr-ref id="adr-20260707-shim-bytes-preserving-id-proxy" /%} — shim の 2 方向 proxy 契約 (id 側)
- {% adr-ref id="adr-20260707-codexclient-observability-log" /%} — silent drop 3 経路の structured log
- {% adr-ref id="adr-20260707-fakevsreal-shim-inversion" /%} — 反転方向 FakeVsReal harness
- {% adr-ref id="adr-20260707-jsonrpc-error-object-opaque-forwarding" /%} — error object opacity (ReplyError spec 準拠 + ReplyRPCError bytes-preserving forwarding)


{% transition from="draft" to="published" date="2026-07-07" %}
invariant note ready as SSOT
{% /transition %}
