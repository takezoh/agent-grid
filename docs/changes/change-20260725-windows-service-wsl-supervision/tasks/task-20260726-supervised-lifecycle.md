---
id: task-20260726-supervised-lifecycle
kind: task
title: Implement supervised lifecycle and health state machine
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/platform/wslhost/supervisor*.go
- src/platform/wslhost/process*.go
- src/platform/wslhost/health*.go
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-windows-service-wsl-supervision}
- {type: dependsOn, target: task-20260726-registration-snapshot-acl}
- {type: dependsOn, target: task-20260726-typed-linux-control}
source_paths: []
change: change-20260725-windows-service-wsl-supervision
summary: Implement probe-before-spawn, direct foreground launch, readiness, resume
  invalidation, classifier, and fixed retry thresholds.
updated: '2026-07-26'
---

## 責務

Probe-before-spawn、direct foreground launch、readiness、resume invalidation、failure classifier、固定 retry threshold を一つの generation-serialized state machine として実装する。

## 詳細手順

- `wsl.exe --distribution D --user U --exec server ...` を shell/launcher/detach なしで起動する。
- fake clock/process/health/WSL seam を使い、stale generation event、同時 timer/health、resume、third-health-failure recovery を決定的に検証する。
- WSL/health/clock-resume の各 mandatory triple を実装と同じ責務に含める。

## 前提

Registration snapshot と typed Linux probe/stop control が利用可能である。

## スコープ外

SCM stop escalation と record file rotation は扱わない。

## 受け入れ条件

- UAC-001..006、011、012、014、015、017 の transition が決定的である。
- Determinate absence より前に spawn しない。
- `verify-control-flow`、`verify-state-lifecycle`、`verify-concurrency`、`verify-performance-budget`: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...` が成功する。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}
