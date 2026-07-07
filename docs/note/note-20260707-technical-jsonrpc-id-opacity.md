---
id: note-20260707-technical-jsonrpc-id-opacity
kind: note
title: JSON-RPC id opacity invariant for proxy/relay code
status: published
created: '2026-07-07'
tags:
- technical
- invariant
- jsonrpc
owners: []
relations:
- {type: references, target: adr-20260707-jsonrpc-id-opaque-forwarding}
- {type: references, target: adr-20260707-shim-bytes-preserving-id-proxy}
- {type: references, target: adr-20260707-codexclient-observability-log}
- {type: references, target: adr-20260707-fakevsreal-shim-inversion}
source_paths:
- src/platform/agent/codexclient
- src/cmd/bridge
- src/client/runtime/subsystem/stream/fake
- src/platform/agent/fakecodex
summary: JSON-RPC 2.0 proxy / relay / bridge を書くとき id 型は必ず bytes-preserving に扱う (int64
  決め打ち禁止)
updated: '2026-07-07'
---

## Summary

このリポで JSON-RPC 2.0 の proxy / relay / bridge を書く / 変更する / review する全員が守るべき不変条件を 1 本にまとめる。**envelope の `id` フィールドは opaque bytes として保持し、reply 時は受信 bytes をそのまま echo する**。`int` / `int64` / 独自 numeric named type で decode してはならない。

背景は 2026-07-07 の codex-app-server-shim silent drop 事案 ({% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%})。同様の落とし穴を将来の proxy 追加 (別 CLI 対応 / claude-app-server / orchestrator dispatch 等) で踏まないための SSOT。

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
- `src/client/runtime/subsystem/stream/fake/appserver.go` / `cli.go` — fake app-server / cli の Handler impl
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

### 6. 関連 ADR

- {% adr-ref id="adr-20260707-jsonrpc-id-opaque-forwarding" /%} — codexclient.Conn の SSOT 変更
- {% adr-ref id="adr-20260707-shim-bytes-preserving-id-proxy" /%} — shim の 2 方向 proxy 契約
- {% adr-ref id="adr-20260707-codexclient-observability-log" /%} — silent drop 3 経路の structured log
- {% adr-ref id="adr-20260707-fakevsreal-shim-inversion" /%} — 反転方向 FakeVsReal harness


{% transition from="draft" to="published" date="2026-07-07" %}
invariant note ready as SSOT
{% /transition %}
