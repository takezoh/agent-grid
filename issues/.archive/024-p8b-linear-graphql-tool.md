# 024: orchestrator — `linear_graphql` agent tool via codex native protocol (§10.5)

- **Phase**: P8b ([plans/04-phases.md#p8-hot-reload--linear_graphql-tool](../plans/04-phases.md))
- **Status**: Done — handler + wiring + advertise すべて実装済み
- **Depends on**: 008 (merged; Linear adapter/auth)、P0c (merged; codexclient `item/tool/call`)
- **並行可**: P5 と独立。agent 向けだが codex protocol native tool のため agent 種別（codex/claude）に依存しない
- **Blocks**: なし

## Background

SPEC §10.5 は orchestrator が advertise する optional client-side tool `linear_graphql` を規定。agent process が Linear に query/mutation を発行できるようにする。

**実装方針（確定）: codex app-server protocol の native client-side tool**。MCP / mcpproxy は使わない — SPEC §10.5 は「orchestrator 自身が tool を実行」「agent に raw token を読ませない」と規定しており、MCP という語は SPEC に登場しない。pinned codex-cli 0.133.0 の protocol を直接確認した結果、`item/tool/call` が ServerRequest（agent→orchestrator）として定義済みで、orchestrator が既に `experimentalApi:true` で opt-in 済み。`orchestrator/agent/handler.go` の `OnServerRequest` に1ケース追加するだけで実現できる。Linear POST は stdlib `net/http`。これにより orchestrator は Linear API を 2 系統持つ（tracker 用 = dispatch 判断、agent tool 用 = 本 issue）。

## Tasks

### A. native client-side tool として linear_graphql を実装

- [x] `orchestrator/lineargql/` パッケージ: `{query, variables}` を受けて Linear GraphQL に POST し §10.5 の success/errors 判別を返す（token はログ禁止）
- [x] `wfconfig.TrackerConfig`（Endpoint / APIKey）から Linear client を構築し `Runner.LinearClient` に注入（`agent.New` で APIKey + Endpoint が設定済みなら自動有効化）
- [x] agent（codex / claude-app-server）が `item/tool/call` でツールを呼べる経路を開通

### B. §10.5 input/output 形式

- [x] input shape: `query` + `variables`（§10.5）
- [x] output: `success=true/false` + `data` + `errors` の判別を結果に反映
- [x] **tool 定義を codex に advertise する** — `thread/start` の `dynamicTools` に `DynamicToolSpec`（`name`/`description`/`inputSchema`）を載せて宣言（`orchestrator/agent/dynamictools.go`）。Linear client（tracker api_key + endpoint）設定時のみ advertise し、agent は discover した `linear_graphql` を `item/tool/call` で発火できる

### C. テスト (§17.7 系)

- [x] `query` + `variables` を受けて Linear GraphQL に渡す（httptest で Linear をモック）
- [x] errors を含む応答を success と区別して返す
- [x] token がログに出ない
- [x] unknown tool / client nil の場合に JSON-RPC error を返す

## Acceptance Criteria

- `thread/start` で `linear_graphql` を advertise し、agent が `item/tool/call` で呼ぶと Linear GraphQL に転送される ✅
- input/output が §10.5 の形式（query+variables、success/errors 判別）✅
- tracker 用 Linear（dispatch）と分離（2 系統）✅
- `go test ./orchestrator/...` 緑、lint 緑 ✅

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.5 (`linear_graphql` tool), §11 (tracker)
- [plans/03-agent.md](../plans/03-agent.md)（§10.5 実装方針）、[plans/04-phases.md#p8](../plans/04-phases.md)、`orchestrator/lineargql/`、`orchestrator/agent/handler.go`
