---
id: adr-20260706-frame-messaging-managed-tool-exposure
kind: adr
title: Frame messaging tools are exposed through managed Claude overlay and Codex
  dynamicTools
status: accepted
created: '2026-07-06'
decision_makers:
- unknown
tags:
- mcp
- frame-messaging
- tooling
owners: []
relations:
- {type: partOf, target: change-20260706-frame-messaging}
source_paths:
- src/platform/mcpproxy/
- src/client/runtime/subsystem/stream/
- src/cmd/server/
summary: Claude は .mcp.json overlay/mcp-exec、Codex は app-server dynamicTools を使い、project/global
  user config 書き換えを避ける
updated: '2026-07-06'
---

# Frame messaging tools are exposed through managed Claude overlay and Codex dynamicTools

## Context

{% context %}
frame messaging tool は agent から broker を呼ぶ入口であり、source frame identity を daemon binding から解決できる必要がある。agent-grid は user config を無断で書き換えない方針を持つため、Claude/Codex の project/global 設定へ直接注入する方式は避けたい。

Claude Code は workspace の `.mcp.json` や MCP add-json 系の設定で MCP server を構成できる。agent-grid には既に `platform/mcpproxy` が workspace overlay を生成し、`server mcp-exec <alias>` で host MCP へ stdio relay する仕組みがある。Codex は agent-grid 管理 session で app-server thread に `dynamicTools` を渡す経路がある。
{% /context %}

## Decision

{% decision %}
Claude には agent-grid 管理の `.mcp.json` overlay / `mcp-exec` alias として frame messaging tools を露出する。Claude CLI 引数への都度注入や user config の直接書き換えは Phase 1 では使わない。

Codex には app-server `dynamicTools` と `item/tool/call` 経路で frame messaging tools を露出する。Codex の project/global `.codex/config.toml` へ MCP 設定を書き込まない。

どちらの経路でも tool input から `sourceSessionId` / `sourceFrameId` は受け取らず、adapter が daemon の frame binding に基づいて source identity を broker へ渡す。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
既存の agent-grid 管理経路を使うため、source frame binding と user config 保護を両立できる。Claude と Codex の露出方法の違いを daemon broker の手前で吸収できる。
{% /consequence %}

{% consequence kind="negative" %}
Claude と Codex で tool exposure の実装 seam が分かれる。共通 MCP server だけを実装すれば済む構造にはならない。
{% /consequence %}

{% consequence kind="neutral" %}
外部 MCP server は将来 thin adapter として追加できるが、authority は daemon broker のままにする。
{% /consequence %}

## Alternatives

- **Codex の `.codex/config.toml` に MCP 設定を書き込む** — 却下。user/project config の変更を伴い、source frame binding も解きにくい。
- **Claude CLI 引数で毎回 MCP を注入する** — 却下。既存の workspace overlay / `mcp-exec` と重複し、version 差分の検証面が増える。
- **単一の外部 MCP server を全 driver に直接配る** — 却下。source frame identity と target runtime access の authority が結局 daemon に戻るため、thin adapter 以上の責務を持たせない。


{% transition from="proposed" to="accepted" date="2026-07-06" %}
Decision extracted from promoted inter-session MCP plan
{% /transition %}
