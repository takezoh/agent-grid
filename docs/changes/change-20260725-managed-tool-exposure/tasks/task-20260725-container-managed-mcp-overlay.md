---
id: task-20260725-container-managed-mcp-overlay
kind: task
title: Compose container MCP overlays from the managed endpoint catalog
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
- src/platform/mcpproxy/
change: change-20260725-managed-tool-exposure
summary: Move agent_frames definition out of mcpproxy and compose one managed overlay
  per workspace target.
updated: '2026-07-25'
---

## 責務

container-scope default setを共通catalogからmaterializeし、mcpproxyをexternal
alias ownerへ限定する。

## 詳細手順

1. mcpproxy単体のmanaged alias期待をREDで除去契約へ変える。
2. no-proxyでもmanaged `.mcp.json` を生成する。
3. proxy mountをbaseに合成しtargetごとに最終mountを一つにする。
4. shared target dedup、非absolute skip、collision postureを固定する。

## 受け入れ条件

- user fileを変更せずmanaged run dirへread-only mount
- external alias `Override:true`、managed alias `Override:false`
- frame LaunchPlan非参照


{% transition from="todo" to="in_progress" date="2026-07-25" %}
Implementation and verification executed.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-25" %}
Acceptance conditions verified.
{% /transition %}
