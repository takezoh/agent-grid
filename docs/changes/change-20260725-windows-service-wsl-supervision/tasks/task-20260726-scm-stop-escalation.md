---
id: task-20260726-scm-stop-escalation
kind: task
title: Implement SCM stop and registered-distro escalation
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/platform/wslhost/service*.go
- src/platform/wslhost/stop*.go
- src/platform/wslhost/supervisor*.go
- src/cmd/agent-grid-wsl-supervisor/main*.go
- src/cmd/agent-grid-wsl-supervisor/register.go
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-windows-service-wsl-supervision}
- {type: dependsOn, target: task-20260726-supervised-lifecycle}
- {type: dependsOn, target: task-20260726-typed-linux-control}
source_paths: []
change: change-20260725-windows-service-wsl-supervision
summary: Bridge SCM controls to ownership-checked SIGTERM and bounded escalation.
updated: '2026-07-26'
---

## 責務

SCM control を ownership-checked SIGTERM と bounded escalation に直列化し、登録 distribution だけを強制終了対象にする。

## 詳細手順

- SCM、Linux control、child process、WSL terminator を interface 注入し、重複/late stop を一つの transaction に畳む。
- 15秒 graceful wait、SCM stop 時だけ owned child kill と2秒 wait、evidence が残る場合だけ bound distro terminate を行う。
- Windows SCM/WSL の fake、T3 backstop、invariant contract を実装と同じ責務に含める。

## 前提

Supervised lifecycle が current generation と ownership token を提供する。

## スコープ外

`wsl --shutdown`、unrelated distribution 列挙、GUI client behavior は禁止する。

## 受け入れ条件

- 15秒 graceful budget が既存 drain/grace を保持する。
- Escalation target は owned child と registered distribution だけである。
- `verify-failure-recovery`: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...` が成功する。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}
