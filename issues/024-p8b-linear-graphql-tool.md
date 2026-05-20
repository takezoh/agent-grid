# 024: orchestrator — `linear_graphql` agent tool via mcpproxy (§10.5)

- **Phase**: P8b ([plans/04-phases.md#p8-hot-reload--linear_graphql-tool](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 008 (merged; Linear adapter/auth)、P0b/P0c (merged; `platform/mcpproxy`)
- **並行可**: P5 と独立。agent 向けだが MCP 経由なので agent 種別（codex/claude）に依存しない
- **Blocks**: M3

## Background

SPEC §10.5 は orchestrator が advertise する optional client-side tool `linear_graphql` を規定。agent process が Linear に query/mutation を発行できるようにする。

**実装方針（確定）: 自作の薄い in-repo MCP サーバ**。既製 `@anthropic-ai/linear-mcp-server` は採用しない — 高水準 tool 群を出すため §10.5 の raw `query`+`variables` passthrough と形が合わず、Task C の httptest モック/token 非ログを満たせず、node/npx の host 依存が増えるため。in-repo サーバは wire 層に既存 `codexclient.Conn`（stdio JSON-RPC）を再利用し、Linear POST は stdlib `net/http`。`platform/mcpproxy/` は container↔host の relay として使う。これにより orchestrator は Linear API を 2 系統持つ（tracker 用 = dispatch 判断、agent tool 用 = 本 issue）。

## Tasks

### A. in-repo MCP サーバ（`linear_graphql` tool 1 個）を mcpproxy 経由で公開

- [ ] `linear_graphql` だけを出す薄い MCP stdio サーバを in-repo 実装（`codexclient.Conn` の server-side helper を wire 層に再利用。既製 npm パッケージは使わない）
- [ ] WORKFLOW.md の Linear auth（`tracker.api_key`）を env 経由でサーバに渡して host 起動（token はログ禁止）
- [ ] `platform/mcpproxy/` 経由で container 内 agent から JSON-RPC で到達できるよう relay 配線（016 の sock/mounts と整合）
- [ ] agent（codex / claude-app-server）が MCP tool として `linear_graphql` を見る

### B. §10.5 input/output 形式

- [ ] input shape: `query` + `variables`（§10.5）
- [ ] output: success / errors の判別を結果に反映
- [ ] tool 定義を `initialize` の supported tool list に載せる（shim/codex 双方が advertise できる経路）

### C. テスト (§17.7 系)

- [ ] `query` + `variables` を受けて Linear GraphQL に渡す（httptest で Linear をモック）
- [ ] errors を含む応答を success と区別して返す
- [ ] token がログに出ない

## Acceptance Criteria

- agent が `linear_graphql` tool で Linear に query/mutation を発行できる
- input/output が §10.5 の形式（query+variables、success/errors 判別）
- tracker 用 Linear（dispatch）と分離（2 系統）
- `go test ./orchestrator/... ./platform/mcpproxy/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.5 (`linear_graphql` tool), §11 (tracker)
- [plans/03-agent.md](../plans/03-agent.md)（§10.5 実装方針）、[plans/04-phases.md#p8](../plans/04-phases.md)、`platform/mcpproxy`、`platform/tracker/linear`
