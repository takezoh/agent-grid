---
id: task-20260706-frame-messaging-mcp-tools
kind: task
title: Frame messaging MCP tools for Claude overlay and Codex dynamic tools
status: done
created: '2026-07-06'
priority: normal
effort: medium
files_touched: []
pr: null
tags:
- mcp
- frame-messaging
- phase-1
owners: []
relations:
- {type: dependsOn, target: task-20260706-frame-messaging-store-broker}
- {type: partOf, target: change-20260706-frame-messaging}
source_paths:
- src/platform/mcpproxy/
- src/client/runtime/subsystem/stream/
- src/cmd/server/
summary: agent_frames.list/read/send_message/reply を frame-scoped source identity
  で公開し、source 偽装を schema 上受け取らない
updated: '2026-07-06'
change: change-20260706-frame-messaging
---

# Frame messaging MCP tools for Claude overlay and Codex dynamic tools

## 責務

Phase 1 の broker API を MCP tool として公開する。Claude は `.mcp.json` overlay / `mcp-exec` alias、Codex は app-server `dynamicTools` / `item/tool/call` 経路を優先し、user config は書き換えない。

## 詳細手順

1. `agent_frames.list`、`agent_frames.send_message`、`agent_frames.read`、`agent_frames.reply` の tool schema を定義する。
2. schema に `sourceSessionId` / `sourceFrameId` を含めない。未知 field を許す実装なら broker 側で input source を無視するのではなく validation で拒否する。
3. MCP invocation から frame-scoped source identity を解決し、broker API に渡す binding を追加する。
4. Claude workspace overlay に agent-grid 管理 alias を追加する。既存 `platform/mcpproxy` / `server mcp-exec <alias>` の構造に合わせる。
5. Codex app-server session には `dynamicTools` として tool schema を渡し、`item/tool/call` を broker invocation に接続する。
6. Phase 1 では `agent_frames.deliver_prompt` を公開しない。

## 前提

- `task-20260706-frame-messaging-store-broker` の broker API が存在する。
- Claude/Codex の user config は上書きしない。

## スコープ外

- prompt delivery tool
- Codex `config.toml` 注入
- Claude CLI 引数への one-shot MCP 注入
- Cross Session target discovery

## 受け入れ条件

- `list` は source と同一 session の claude/codex driver frame だけを返す。
- `send_message` / `read` / `reply` は broker gate と audit を通る。
- source 偽装 field は schema validation で拒否される。
- Claude overlay と Codex dynamicTools の両方で Phase 1 tool set が露出する。
- Phase 1 build で `deliver_prompt` 相当の tool は露出しない。


{% transition from="todo" to="in_progress" date="2026-07-06" %}
Started managed tool exposure implementation.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-06" %}
Implemented managed Claude alias and Codex dynamicTools shim with source-spoof rejection and tests.
{% /transition %}
