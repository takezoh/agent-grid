---
id: task-20260725-managed-mcp-contract
kind: task
title: Introduce managed MCP endpoint catalog and fail-closed contracts
status: done
created: '2026-07-25'
priority: normal
effort: medium
files_touched: []
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-managed-tool-exposure}
source_paths:
- src/platform/agentlaunch/
- src/host/state/
change: change-20260725-managed-tool-exposure
summary: Add typed managed MCP tools, endpoint validation, and RED contract tests.
updated: '2026-07-25'
---

## 責務

`platform/agentlaunch` に唯一の `ManagedMCPTool(s)` 型、endpoint descriptor、
validation、MCP alias builder を導入する。

## 詳細手順

1. RED testsで empty no-op、unknown bit、missing command/socket、入力非変更を固定する。
2. `agent_frames` の alias、stdio、subcommand、collision、authority propertyをcatalog化する。
3. host/container endpoint差分がcommand/socketだけであるcontract testを追加する。

## スコープ外

Codex dynamicTools schemaの共通化、host placement適用、container mount composition。

## 受け入れ条件

- unknown/missing endpointがerror
- empty setが非変更no-op
- descriptorの二重定義がない


{% transition from="todo" to="in_progress" date="2026-07-25" %}
Implementation and verification executed.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-25" %}
Acceptance conditions verified.
{% /transition %}
