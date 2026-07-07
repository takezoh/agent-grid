---
id: plan-20260707-codexclient-jsonrpc-id-opaque
kind: plan
title: codexclient JSON-RPC id opaque forwarding plan
status: draft
created: '2026-07-07'
goal: codexclient.Conn の JSON-RPC id を bytes-preserving な opaque 値 (named type `codexclient.RequestID`)
  にし、shim が codex-cli 0.142.5 の string id を透過して initialize を成立させる SSOT 単一化と 3 点セット
  (fake + FakeVsReal + contract) 整備を 1 PR / 5 milestone で行う
scope_in:
- src/platform/agent/codexclient/conn.go の rpcMessage.ID を json.RawMessage 化し、Handler
  I/F と Reply/ReplyError の signature を named type `codexclient.RequestID` に置換
- codexclient.Conn の pending map key を int64 のまま維持し、reply 到着時に wire id bytes を int64
  parse する経路を追加 (改善案 2 採用、要件案の string map 変更を撤回)
- src/cmd/bridge/codex_app_server_shim.go の 2 本 Conn (downstream / upstream) の役割別
  proxy 実装 (downstream 側 id echo / upstream 側 numeric 採番)
- src/client/runtime/subsystem/stream/fake/appserver.go の serverConn を string id 受理・同
  id で reply 対応
- src/platform/agent/fakecodex/fakecodex.go の OnServerRequest を string id 受理・同 id
  で reply 対応
- 上記に伴う Handler 実装 caller の signature 追従 (client/runtime/subsystem/stream/backend.go
  / orchestrator/agent/handler.go / cmd/claude-app-server / 各 *_test.go)
- Conn.Run の 3 種 silent drop 経路 (json.Unmarshal 失敗 / id parse 失敗 / pending miss) の構造化
  log 化
- T0 unit test (codexclient/conn_test.go に string / number / null / 欠如 の round-trip
  + pending 解決 + invalid envelope log)
- T2 contract test (cmd/bridge/codex_app_server_shim_test.go に string id を含む initialize
  の in-process shim 経由 echo pin)
- T2 opt-in FakeVsReal (fakecodex/codex_real_cli_e2e_test.go に shim を挟んだ real codex-cli
  0.142.5 driving case を追加、build tag `e2e` + `AG_E2E_CODEX_BIN` に相乗り)
scope_out:
- POST /api/sessions 504 (047fae39) の再修正
- frame-messaging 機能 (agent_frames.*) の追加改修
- 上流 codex-cli / codex-app-server 側 (forks/ 含む) のパッチ
- 走行中 daemon (PID 94953) の再 build/redeploy 運用 (別 task で ops 手順として追跡)
- id 型変更に便乗した codexclient 一般化 refactor (transport 抽象・timeout モデル・error 型)
- 新規 build tag の追加 (既存 `e2e` に相乗り)
- id normalization を conn.go の外に切り出す予防的 file 分割
milestones:
- id: m1
  title: RequestID named type 導入 + rpcMessage.ID を json.RawMessage 化 + T0 unit
  status: todo
- id: m2
  title: Handler I/F の signature 追従 (shim / fake × 2 / backend / orchestrator agent
    handler / claude-app-server)
  status: todo
- id: m3
  title: shim proxy の 2 方向実装 (downstream echo / upstream numeric) + T2 contract test
  status: todo
- id: m4
  title: Conn.Run の silent drop 経路 3 種を構造化 log 化
  status: todo
- id: m5
  title: fakecodex real-cli e2e に shim を挟んだ string id initialize case を追加 (opt-in
    FakeVsReal)
  status: todo
contracts:
- 'id opaque: rpcMessage.ID は bytes-preserving (json.RawMessage / RequestID named
  type)'
- 'downstream echo: shim は CLI 発行 id を bytes-preserving で reply に載せる'
- 'upstream numeric: shim は upstream に対して常に numeric id で Request を発行する'
- 'pending SSOT: Conn.pending map は int64 key を単独 SSOT とし、reply 側 wire bytes を int64
  parse する'
- 'silent drop 禁止: json.Unmarshal 失敗 / id parse 失敗 / pending miss の 3 経路すべてに構造化 log'
- 'notification 経路不変: id 未出現 message は Handler.OnNotification のみに dispatch'
tags:
- codex
- shim
- jsonrpc
- bug-fix
owners: []
relations:
- {type: implements, target: spec-20260707-codexclient-jsonrpc-id-opaque}
- {type: hasPart, target: adr-20260707-jsonrpc-id-opaque-forwarding}
- {type: hasPart, target: adr-20260707-shim-bytes-preserving-id-proxy}
- {type: hasPart, target: adr-20260707-codexclient-observability-log}
- {type: hasPart, target: adr-20260707-fakevsreal-shim-inversion}
source_paths:
- src/platform/agent/codexclient
- src/cmd/bridge
- src/client/runtime/subsystem/stream/fake
- src/platform/agent/fakecodex
- src/client/runtime/subsystem/stream/backend.go
- src/orchestrator/agent/handler.go
- src/cmd/claude-app-server
summary: id を bytes-preserving な opaque 値にする SSOT 単一化と、shim proxy / fake × 2 / caller
  の追従、observability log、opt-in FakeVsReal 相乗り拡張を 5 milestone / 1 PR で行う実装計画
---

# Plan — codexclient JSON-RPC id opaque forwarding

## Goal

`spec-20260707-codexclient-jsonrpc-id-opaque` を、**SSOT を JSON-RPC 2.0 準拠 (`codexclient.RequestID` = `json.RawMessage` named type) へ単一化**する 1 PR で実装する。

同時に fake + FakeVsReal + contract の 3 点セットを更新し、以下 4 個の設計判断を個別 ADR に分離する:

- `adr-20260707-jsonrpc-id-opaque-forwarding` — SSOT 型変更と Handler I/F 破壊的更新
- `adr-20260707-shim-bytes-preserving-id-proxy` — shim の 2 方向 proxy 契約と invalid id 型 / null id の扱い
- `adr-20260707-codexclient-observability-log` — silent drop 経路 3 種の構造化 log 化
- `adr-20260707-fakevsreal-shim-inversion` — fakecodex 側 real-cli e2e に shim を挟む driving 反転 (adr-20260624-0002 を extends)

## Implementation Sequence

5 milestone を **1 PR 内**で下記の順に走らせる (中間 commit は build 可能な境界で切る)。

```text
m1 -> m2 -> m3
m1 -> m4          # m4 は m2/m3 と独立にも書けるが m1 後に着手
m3 -> m5          # FakeVsReal は shim 実装完了後
```

{% milestone id="m1" %}
**RequestID named type 導入と rpcMessage.ID の opaque 化** — `src/platform/agent/codexclient/conn.go` に `type RequestID json.RawMessage` を公開型として追加。`rpcMessage.ID` を `*RequestID` に置換。`Conn.Run` の response 判定分岐は wire id bytes を `strconv.ParseInt` で int64 化して pending 解決する経路に変更する。ここで pending map key は int64 のまま (改善案 2)。T0 unit test (`codexclient/conn_test.go`) を追加し、(a) string / number / null / 欠如 の 4 パターン round-trip bytes 保存 (`AC-002`)、(b) 自発 Request → pending 解決の numeric 経路 (`AC-003`) を pin する。関連 ADR: `adr-20260707-jsonrpc-id-opaque-forwarding`
{% /milestone %}

{% milestone id="m2" %}
**Handler I/F の signature 一括追従** — `Handler.OnServerRequest(id RequestID, ...)` / `Conn.Reply(id RequestID, ...)` / `Conn.ReplyError(id RequestID, ...)` へ変更した結果、コンパイル時に検出される全 caller を **1 コミット** で追従させる。対象は以下:

- `src/cmd/bridge/codex_app_server_shim.go` (2 本 Conn の Handler 実装)
- `src/client/runtime/subsystem/stream/fake/appserver.go` (serverConn.OnServerRequest)
- `src/platform/agent/fakecodex/fakecodex.go` (Server.OnServerRequest — 否定役指摘 [scope 列挙])
- `src/client/runtime/subsystem/stream/backend.go` (Backend.Start / applyPatchApproval 系 Handler)
- `src/orchestrator/agent/handler.go` (Handler 実装)
- `src/cmd/claude-app-server/*.go` (Handler 実装があれば)
- `src/**/**/*_test.go` の Handler mock / stub 呼び出し (launch_flow_test / runner_events_test / runner_loop_test / handler_test / backend_test 他)

この時点では shim の proxy 動作は「元 id を素で echo する」最小実装で通す (詳細は m3)。関連 ADR: `adr-20260707-jsonrpc-id-opaque-forwarding`
{% /milestone %}

{% milestone id="m3" %}
**shim の 2 方向 proxy 実装と T2 contract test** — `codexShimSession` を以下の対称構造で書き分ける:

- **downstream → upstream** (CLI initiator): downstream Handler で受けた RequestID を struct field に保存し、`upstream.Request(method, params)` を呼ぶ (upstream 側は Conn 内部で新規 numeric 採番)。upstream から result / error が返ったら `downstream.Reply(savedID, result)` / `downstream.ReplyError(savedID, msg)` で **元 bytes を echo**。
- **upstream → downstream** (server-initiated, e.g. applyPatchApproval): upstream Handler で受けた RequestID を保存し、`downstream.Request(method, params)` を呼ぶ (downstream 側で新規 numeric 採番)。downstream 返却後 `upstream.Reply(savedID, result)` で echo。

T2 contract test を `src/cmd/bridge/codex_app_server_shim_test.go` に追加: (a) in-process で fake AppServer + shim + test client を組み立て、`{"id":"initialize","method":"initialize",...}` を送って reply bytes と id が一致することを assert (`AC-001` / `AC-004`)、(b) 別テストで upstream 発信の server-initiated request が downstream で新規 numeric 採番されたことも pin。関連 ADR: `adr-20260707-shim-bytes-preserving-id-proxy`
{% /milestone %}

{% milestone id="m4" %}
**silent drop 経路 3 種の構造化 log 化** — `Conn.Run` の以下 3 経路に構造化 log を挿入する:

1. `json.Unmarshal(data, &msg)` 失敗 → `{"event":"codexclient.decode_error","raw":<truncated bytes>,"err":<msg>}`
2. wire id が JSON-RPC 2.0 で禁じられた型 (object / array) または int64 parse 不能 numeric → `{"event":"codexclient.invalid_id","raw_id":<bytes>,"method":<msg.Method>,"err":<parse err>}`
3. pending map miss (id を parse できたが対応する自発 Request が居ない) → `{"event":"codexclient.pending_miss","raw_id":<bytes>,"method":<msg.Method>,"result_len":len,"error_len":len}`

log 出力先は既存の `slog.Default()` (もしくは Conn に注入された logger)。3 経路とも `continue` で後続 message 処理を継続 (transport は close しない — 案 A)。T0 unit で log capture を assert (`AC-006`)。関連 ADR: `adr-20260707-codexclient-observability-log`
{% /milestone %}

{% milestone id="m5" %}
**FakeVsReal 相乗り拡張 (opt-in)** — `src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` (既存 `//go:build e2e` + `AG_E2E_CODEX_BIN` gate) に、**shim を挟んで real codex-cli 0.142.5 を子プロセスとして driving する** subtest を追加する。既存 harness は agent-grid が client / real binary が app-server の方向だが、今回追加する subtest は agent-grid の shim を app-server 役として起動し real codex-cli を driving する **反転方向**。既存 build tag / env gate に相乗り、AG_E2E_CODEX_BIN の semantics は「real codex-cli バイナリの絶対パス」で共通 (`AC-005`)。関連 ADR: `adr-20260707-fakevsreal-shim-inversion`
{% /milestone %}

## Targets

**変更ファイル (production code)**:

- `src/platform/agent/codexclient/conn.go` — SSOT 型定義変更 / Handler I/F 変更 / pending 経路調整 / 構造化 log 化
- `src/platform/agent/codexclient/client.go` — Request / Notify / Initialize の caller 追従 (外部 signature は現行維持)
- `src/cmd/bridge/codex_app_server_shim.go` — 2 方向 proxy 実装
- `src/client/runtime/subsystem/stream/fake/appserver.go` — Handler 実装追従
- `src/platform/agent/fakecodex/fakecodex.go` — Handler 実装追従 (否定役指摘)
- `src/client/runtime/subsystem/stream/backend.go` — Handler 実装追従
- `src/orchestrator/agent/handler.go` — Handler 実装追従
- `src/cmd/claude-app-server/*.go` — Handler 実装追従 (該当箇所のみ)

**追加 / 更新テスト**:

- `src/platform/agent/codexclient/conn_test.go` (T0 unit, 新規または拡張)
- `src/cmd/bridge/codex_app_server_shim_test.go` (T2 contract, 拡張)
- `src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` (T2 opt-in FakeVsReal, 拡張)
- `src/client/runtime/subsystem/stream/fake/appserver_test.go` (fake の Handler 追従テスト更新)
- `src/orchestrator/agent/{runner_events_test,runner_loop_test,handler_test}.go` (mock caller の signature 追従)
- `src/client/runtime/subsystem/stream/{launch_flow_test,backend_test}.go` (mock caller の signature 追従)

**再利用する既存資産**:

- `codexclient.Server.Emit*` helpers — event 発火は変更なし
- `//go:build e2e` + `AG_E2E_CODEX_BIN` / `AG_E2E_APPSERVER_BIN` env gate (adr-20260624-0002 の contract)
- `slog.Default()` — 構造化 log の出力先
- `codexschema` — method 名 (id 型変更は method / params 経路に触れない)

## Verification

以下すべてが green になった時点で本 plan の完了とみなす。

- `cd src && go test ./...` (通常 CI build, `//go:build e2e` なし) — T0 unit + T2 contract がすべて pass、 FakeVsReal は build tag exclude で自動 skip (`AC-008`)
- `cd src && go vet ./...`
- `make lint` (golangci-lint / depguard / funlen) — `NFR-002` / `NFR-007` を検証
- Handler I/F 変更後の caller 追従漏れが `go build ./...` で非ゼロにならない (`NFR-005`)
- opt-in 手動確認 (local): `cd src && AG_E2E_CODEX_BIN=$(which codex) go test -tags e2e ./platform/agent/fakecodex/...` で m5 の shim-inverted subtest が pass (`AC-005`)
- `docs lint` warning / error なし

## Open Questions

以下は plan-impl 引き渡し時に実装局面で判断する:

- 構造化 log の rate limit をこの PR で入れるか、後続 PR で扱うか (現状は付けない方針 = 事案発生頻度が低く rate limit の要求が薄いため)
- Conn に logger を注入する I/F を追加するか、`slog.Default()` を直接使うか (NFR-002 の 500 行 target を優先し、後者を第一候補とする)
- Handler I/F を破壊的に変更するタイミングで internal use 限定の型 alias を残すか (現状 alias は残さない方針 — DP-d3 A / 改善案 5 に整合)
