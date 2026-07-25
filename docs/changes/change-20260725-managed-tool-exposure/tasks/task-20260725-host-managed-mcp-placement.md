---
id: task-20260725-host-managed-mcp-placement
kind: task
title: Propagate Claude-compatible host MCP launch placement
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
- src/host/driver/
- src/host/runtime/
- src/platform/agentlaunch/
change: change-20260725-managed-tool-exposure
summary: Replace the frame-messaging boolean and enforce validation before authority
  and spawn.
updated: '2026-07-25'
---

## 責務

LaunchPlanのbooleanをClaude-compatible `ManagedMCPTools` contractへ置換し、
validation前にtoken/spawnを行わない。

## 詳細手順

1. state/platformで同じplatform型を参照しcontinuity testsを更新する。
2. production assignmentをClaude driverだけに限定しnegative scan/testを置く。
3. validation → authority → token → render → spawnを両host launch pathで保証する。
4. empty planではMCP argsを付けずambient tokenを除去する。

## 受け入れ条件

- new-session/cold-start双方でClaude config+token
- invalid endpoint/unknown toolでspawn/token registrationなし
- HOME、argv、env入力を変更しない


{% transition from="todo" to="in_progress" date="2026-07-25" %}
Implementation and verification executed.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-25" %}
Acceptance conditions verified.
{% /transition %}
