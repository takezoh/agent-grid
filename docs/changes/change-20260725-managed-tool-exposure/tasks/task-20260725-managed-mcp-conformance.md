---
id: task-20260725-managed-mcp-conformance
kind: task
title: Verify managed MCP boundary and update durable documentation
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
- docs/adr/
- docs/design/
- src/cmd/claude-app-server/
change: change-20260725-managed-tool-exposure
summary: Run cross-boundary tests, preserve Codex dynamicTools behavior, and update
  persistent docs.
updated: '2026-07-25'
---

## 責務

横断回帰、persistent design promotion、ADR confirmationを完了する。

## 詳細手順

1. Codex dynamicTools schema/count/dispatch既存testを実行する。
2. platform/host/orchestrator/full testとlintを実行する。
3.旧boolean/helper/mcpproxy managed injectionのproduction参照0件を確認する。
4. design-platform/design-clientへ責務・invariantをpromotionする。

## 受け入れ条件

- verification.mdの全evidenceが記録済み
- proposed ADRがhost承認後accepted
- docs lint --conformanceが成功


{% transition from="todo" to="in_progress" date="2026-07-25" %}
Implementation and verification executed.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-25" %}
Acceptance conditions verified.
{% /transition %}
