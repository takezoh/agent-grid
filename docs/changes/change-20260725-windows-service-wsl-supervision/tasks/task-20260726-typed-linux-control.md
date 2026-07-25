---
id: task-20260726-typed-linux-control
kind: task
title: Implement typed Linux probe and stop control
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/cmd/server/supervisor_control*.go
- src/cmd/server/daemon_lock*.go
- src/cmd/server/main.go
- src/cmd/server/main_test.go
- src/cmd/server/subcommand.go
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-windows-service-wsl-supervision}
- {type: dependsOn, target: task-20260726-registration-snapshot-acl}
source_paths: []
change: change-20260725-windows-service-wsl-supervision
summary: Add additive server CLI commands that classify lock ownership and deliver
  ownership-checked SIGTERM.
updated: '2026-07-26'
---

## 責務

Linux daemon lock の ownership を typed JSON で分類する `supervisor-probe` と、token 再検証後に SIGTERM を送る `supervisor-stop` を追加する。

## 詳細手順

- 既存 daemon lock semantics を再利用し、data-dir lock holder、executable/argv fingerprint、Linux PID start token を検証する。
- absent、matched running、mismatch、unknown を排他的に返し、stale/mismatch token では signal authority を与えない。
- HostControl fake、`FakeVsRealHostControlLockAndSignal`、`ContractProbeBeforeSpawnAndTokenRevalidation` を実装と同じ責務に含める。

## 前提

既存 server daemon mode と CLI compatibility を維持する。

## スコープ外

第二 daemon、shutdown HTTP endpoint、Windows lifecycle orchestration は追加しない。

## 受け入れ条件

- Probe が absent、matched running、mismatch、unknown を区別する。
- Stop が stale/mismatched ownership を拒否し、matched ownership では既存 SIGTERM path に到達する。
- `verify-integration-contract`: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...` が成功する。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}
