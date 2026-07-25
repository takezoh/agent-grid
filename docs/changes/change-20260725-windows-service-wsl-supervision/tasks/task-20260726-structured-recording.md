---
id: task-20260726-structured-recording
kind: task
title: Implement structured machine-wide recording
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/platform/wslhost/record*.go
- src/platform/wslhost/config*.go
- docs/design/design-host.md
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-windows-service-wsl-supervision}
- {type: dependsOn, target: task-20260726-registration-snapshot-acl}
source_paths: []
change: change-20260725-windows-service-wsl-supervision
summary: Provide operator-readable structured records, safe retention and pre-start
  lookup instructions.
updated: '2026-07-26'
---

## 責務

Operator-readable allowlisted structured record、safe retention、SCM pre-start failure の lookup instructions を実装する。

## 詳細手順

- `%ProgramData%\AgentGrid\logs\wsl-host.log` に allowlisted JSON event だけを記録し、10 MiB、5 files、30 days を強制する。
- raw stdout/stderr、secret、environment value を永続化しない。
- RecordSink/ACL の fake、Windows fidelity backstop、resource/observability invariant tests を実装と同じ責務に含める。

## 前提

Registration snapshot と lifecycle event taxonomy を利用し、共有 `config.go` は先行 task の成果物へ追従する。

## スコープ外

Event Log を primary runtime sink にせず、lifecycle/stop transition 自体は変更しない。

## 受け入れ条件

- ProgramData path/ACL/10MiB x5/30-day policy が強制される。
- SCM pre-process failure に deterministic documented lookup がある。
- `verify-resource-management`、`verify-user-observability`: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...` が成功する。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}
